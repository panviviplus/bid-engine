"""页面抓取：静态 HTTP 优先，必要时切换无头浏览器。

工程约束（与设计文档一致）：
- 同域名串行访问 + 随机间隔 1–3s；
- 单次请求超时不超过 request_timeout_sec；
- 失败按指数退避重试，最多 max_retries 次；
- 浏览器上下文并发上限 max_browser_contexts，避免容器资源被打满。
"""

from __future__ import annotations

import asyncio
import logging
import random
from dataclasses import dataclass
from urllib.parse import urlparse

import httpx

from .config import settings

logger = logging.getLogger(__name__)

# 需要切换浏览器的情况：Cloudflare 系拦截码 + 需要 JS 渲染的空壳页面
CLOUDFLARE_STATUS = {403, 412, 429, 520, 521, 522, 523, 524}


@dataclass
class FetchResult:
    url: str
    content: str
    status_code: int
    strategy: str          # static | browser
    warnings: list[str]


class DomainThrottle:
    """按域名串行 + 随机间隔，避免对上游站点造成压力。"""

    def __init__(self) -> None:
        self._locks: dict[str, asyncio.Lock] = {}

    def lock_for(self, url: str) -> asyncio.Lock:
        host = urlparse(url).netloc.lower()
        lock = self._locks.get(host)
        if lock is None:
            lock = asyncio.Lock()
            self._locks[host] = lock
        return lock

    async def wait(self, url: str) -> None:
        delay = random.uniform(settings.min_delay_sec, settings.max_delay_sec)
        if delay > 0:
            await asyncio.sleep(delay)


class BrowserPool:
    """Playwright 浏览器池：懒加载 + 上下文信号量。"""

    def __init__(self) -> None:
        self._playwright = None
        self._browser = None
        self._semaphore = asyncio.Semaphore(max(1, settings.max_browser_contexts))
        self._lock = asyncio.Lock()
        self._ready = False

    @property
    def ready(self) -> bool:
        return self._ready

    async def _ensure_browser(self) -> None:
        if self._ready:
            return
        async with self._lock:
            if self._ready:
                return
            from playwright.async_api import async_playwright

            self._playwright = await async_playwright().start()
            self._browser = await self._playwright.chromium.launch(
                headless=settings.headless,
                args=["--no-sandbox", "--disable-dev-shm-usage"],
            )
            self._ready = True
            logger.info("Playwright 浏览器已启动（headless=%s）", settings.headless)

    async def fetch(self, url: str) -> tuple[str, int]:
        """使用浏览器渲染页面，返回 HTML 与状态码。"""
        await self._ensure_browser()
        assert self._browser is not None
        async with self._semaphore:
            context = await self._browser.new_context(
                user_agent=settings.user_agent,
                locale="zh-CN",
            )
            try:
                page = await context.new_page()
                page.set_default_timeout(settings.request_timeout_sec * 1000)
                response = await page.goto(url, wait_until="domcontentloaded")
                # 给前端渲染留一点时间，但不做无限等待
                try:
                    await page.wait_for_load_state("networkidle", timeout=5000)
                except Exception:  # noqa: BLE001 - 网络空闲超时属于正常情况
                    pass
                html = await page.content()
                status = response.status if response else 0
                return html, status
            finally:
                await context.close()

    async def close(self) -> None:
        if self._browser is not None:
            await self._browser.close()
        if self._playwright is not None:
            await self._playwright.stop()
        self._ready = False


class Fetcher:
    """统一的抓取入口：静态优先，按需切换浏览器。"""

    def __init__(self) -> None:
        self._client: httpx.AsyncClient | None = None
        self.browser = BrowserPool()
        self.throttle = DomainThrottle()

    async def client(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = httpx.AsyncClient(
                timeout=settings.request_timeout_sec,
                follow_redirects=True,
                headers={
                    "User-Agent": settings.user_agent,
                    "Accept-Language": "zh-CN,zh;q=0.9",
                },
            )
        return self._client

    async def close(self) -> None:
        if self._client is not None:
            await self._client.aclose()
            self._client = None
        await self.browser.close()

    async def fetch(self, url: str, force_browser: bool = False) -> FetchResult:
        """抓取单个 URL。静态失败或命中拦截码时自动切换浏览器。"""
        warnings: list[str] = []
        async with self.throttle.lock_for(url):
            await self.throttle.wait(url)
            if not force_browser:
                try:
                    content, status = await self._fetch_static(url)
                    if status < 400 and content.strip():
                        return FetchResult(url, content, status, "static", warnings)
                    warnings.append(f"静态请求返回 {status}")
                except Exception as exc:  # noqa: BLE001 - 网络异常统一降级到浏览器
                    warnings.append(f"静态请求失败：{exc}")
                    logger.debug("静态抓取失败 %s: %s", url, exc)

            try:
                html, status = await self.browser.fetch(url)
                if not html.strip():
                    warnings.append("浏览器渲染结果为空")
                return FetchResult(url, html, status, "browser", warnings)
            except Exception as exc:  # noqa: BLE001 - 浏览器不可用时如实上报
                warnings.append(f"浏览器渲染失败：{exc}")
                logger.warning("浏览器抓取失败 %s: %s", url, exc)
                return FetchResult(url, "", 0, "browser", warnings)

    async def _fetch_static(self, url: str) -> tuple[str, int]:
        client = await self.client()
        last_error: Exception | None = None
        for attempt in range(settings.max_retries + 1):
            try:
                response = await client.get(url)
                if response.status_code in CLOUDFLARE_STATUS:
                    # 交给上层切换浏览器
                    return response.text, response.status_code
                response.raise_for_status()
                return response.text, response.status_code
            except Exception as exc:  # noqa: BLE001 - 重试后仍失败则向上抛出
                last_error = exc
                if attempt < settings.max_retries:
                    backoff = 1.5 ** attempt
                    await asyncio.sleep(backoff)
        assert last_error is not None
        raise last_error

    async def fetch_json(self, url: str, method: str = "GET", body: dict | None = None) -> object:
        """抓取 JSON 接口（站内搜索接口优先策略）。"""
        client = await self.client()
        async with self.throttle.lock_for(url):
            await self.throttle.wait(url)
            if method.upper() == "POST":
                response = await client.post(url, json=body or {})
            else:
                response = await client.get(url)
            response.raise_for_status()
            return response.json()


fetcher = Fetcher()
