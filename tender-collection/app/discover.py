"""发现阶段：从源站列表/接口找出「新公告」的详情页链接。

分级策略（接口优先）：
1. api    —— 站内搜索/列表 JSON 接口，最稳定，优先使用；
2. list   —— 静态列表页 HTML，通用链接发现 + 可选站点选择器；
3. browser——需要 JS 渲染或反爬的站点，渲染后再走 list 逻辑。

增量：游标支持两种形态
- date:<YYYY-MM-DD>  按发布时间过滤（列表页带日期时使用）
- url:<详情页链接>    遇到该链接即停止（列表按时间倒序时使用）
"""

from __future__ import annotations

import logging
import re
from urllib.parse import urljoin, urlparse

from bs4 import BeautifulSoup

from .fetcher import Fetcher
from .recipes import Recipe
from .schemas import DiscoveredItem, ItemError

logger = logging.getLogger(__name__)

# 明显不是公告详情页的链接（导航、登录、静态资源等）
DEFAULT_EXCLUDE = re.compile(
    r"(javascript:|mailto:|tel:|#|\.(css|js|png|jpe?g|gif|svg|ico|woff2?)($|\?)"
    r"|/(login|register|signin|signup|about|contact|help|rss)(/|$|\.))",
    re.IGNORECASE,
)

ATTACHMENT_SUFFIX = re.compile(r"\.(pdf|docx?|xlsx?|zip|rar|7z)(\?|$)", re.IGNORECASE)
DATE_IN_TEXT = re.compile(r"(20\d{2})[-/年.](\d{1,2})[-/月.](\d{1,2})")


async def discover(
    recipe: Recipe,
    fetcher: Fetcher,
    list_url: str,
    cursor: str = "",
    max_pages: int = 1,
    max_items: int = 50,
) -> tuple[list[DiscoveredItem], str, list[ItemError]]:
    """执行发现，返回（条目, 新游标, 错误列表）。"""
    mode = (recipe.discovery_mode or "list").lower()
    if mode == "api":
        return await _discover_api(recipe, fetcher, list_url, cursor, max_items)
    return await _discover_list(recipe, fetcher, list_url, cursor, max_pages, max_items)


async def _discover_api(
    recipe: Recipe,
    fetcher: Fetcher,
    list_url: str,
    cursor: str,
    max_items: int,
) -> tuple[list[DiscoveredItem], str, list[ItemError]]:
    errors: list[ItemError] = []
    try:
        payload = await fetcher.fetch_json(list_url, recipe.api_method, recipe.api_body)
    except Exception as exc:  # noqa: BLE001 - 发现失败交由上层记录到批次明细
        return [], cursor, [ItemError(url=list_url, reason=f"接口请求失败：{exc}")]

    rows = _walk_path(payload, recipe.api_items_path) if recipe.api_items_path else payload
    if not isinstance(rows, list):
        return [], cursor, [ItemError(url=list_url, reason="接口返回结构不是列表")]

    cursor_kind, cursor_value = _parse_cursor(cursor)
    items: list[DiscoveredItem] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        url = _pick_field(row, recipe.api_field_map.get("url", "url"))
        if not url:
            continue
        url = urljoin(list_url, str(url))
        title = _pick_field(row, recipe.api_field_map.get("title", "title"))
        publish_date = _normalize_date(_pick_field(row, recipe.api_field_map.get("publish_date", "publishDate")))

        if cursor_kind == "date" and publish_date and cursor_value and publish_date <= cursor_value:
            continue
        if cursor_kind == "url" and cursor_value and url == cursor_value:
            break

        items.append(DiscoveredItem(url=url, title=str(title or ""), publish_date=publish_date))
        if len(items) >= max_items:
            break

    return items, _build_cursor(items, cursor), errors


async def _discover_list(
    recipe: Recipe,
    fetcher: Fetcher,
    list_url: str,
    cursor: str,
    max_pages: int,
    max_items: int,
) -> tuple[list[DiscoveredItem], str, list[ItemError]]:
    errors: list[ItemError] = []
    cursor_kind, cursor_value = _parse_cursor(cursor)
    items: list[DiscoveredItem] = []
    seen: set[str] = set()
    pages = max(1, min(max_pages, recipe.max_pages or max_pages))

    for page_index in range(pages):
        page_url = _page_url(list_url, page_index)
        result = await fetcher.fetch(page_url, force_browser=recipe.needs_browser)
        if not result.content.strip():
            errors.append(ItemError(url=page_url, reason="；".join(result.warnings) or "页面为空"))
            continue

        page_items, stop = _extract_links(recipe, page_url, result.content, cursor_kind, cursor_value, seen)
        items.extend(page_items)
        if stop or len(items) >= max_items:
            break

    if len(items) > max_items:
        items = items[:max_items]
    return items, _build_cursor(items, cursor), errors


def _extract_links(
    recipe: Recipe,
    page_url: str,
    html: str,
    cursor_kind: str,
    cursor_value: str,
    seen: set[str],
) -> tuple[list[DiscoveredItem], bool]:
    soup = BeautifulSoup(html, "lxml")
    anchors = soup.select(recipe.list_link_selector) if recipe.list_link_selector else soup.find_all("a", href=True)

    include = re.compile(recipe.link_pattern) if recipe.link_pattern else None
    exclude = re.compile(recipe.list_exclude_pattern) if recipe.list_exclude_pattern else None

    items: list[DiscoveredItem] = []
    stop = False
    for anchor in anchors:
        href = anchor.get("href") or ""
        if not href or href.startswith("#"):
            continue
        url = urljoin(page_url, href)
        if urlparse(url).netloc == "" or DEFAULT_EXCLUDE.search(url):
            continue
        if include and not include.search(url):
            continue
        if exclude and exclude.search(url):
            continue
        if ATTACHMENT_SUFFIX.search(url):
            # 附件直链不是公告详情页
            continue
        if url in seen:
            continue
        if cursor_kind == "url" and cursor_value and url == cursor_value:
            stop = True
            break

        title = " ".join((anchor.get_text() or "").split())
        seen.add(url)
        items.append(DiscoveredItem(url=url, title=title))
    return _prefer_same_directory(items, page_url), stop


def _dir_prefix(url: str) -> str:
    """取 URL 的目录前缀（含末尾斜杠）。"""
    path = urlparse(url).path or "/"
    if not path.endswith("/"):
        path = path.rsplit("/", 1)[0] + "/"
    return path or "/"


def _prefer_same_directory(items: list[DiscoveredItem], page_url: str) -> list[DiscoveredItem]:
    """列表页常混入导航、政策动态等站内链接，只保留与列表页同目录子树下的详情链接。

    仅在「同目录确实解析出链接」时收窄，避免把详情页位于兄弟目录的站点误伤为空结果。
    """
    base = _dir_prefix(page_url)
    if base == "/" or len(items) <= 1:
        return items
    inside = [item for item in items if urlparse(item.url).path.startswith(base)]
    return inside if inside else items


def _page_url(list_url: str, page_index: int) -> str:
    """翻页：仅在 URL 含 {page} 占位或 ?page= 参数时生效。"""
    if page_index == 0:
        return list_url
    if "{page}" in list_url:
        return list_url.replace("{page}", str(page_index + 1))
    if re.search(r"[?&]page=\d+", list_url):
        return re.sub(r"([?&]page=)\d+", rf"\g<1>{page_index + 1}", list_url)
    return list_url


def _parse_cursor(cursor: str) -> tuple[str, str]:
    cursor = (cursor or "").strip()
    if not cursor:
        return "", ""
    if cursor.startswith("date:"):
        return "date", cursor[5:]
    if cursor.startswith("url:"):
        return "url", cursor[4:]
    return "", cursor


def _build_cursor(items: list[DiscoveredItem], fallback: str) -> str:
    if not items:
        return fallback
    dated = [item for item in items if item.publish_date]
    if dated:
        newest = max(item.publish_date for item in dated)
        return f"date:{newest}"
    return f"url:{items[0].url}"


def _walk_path(payload: object, path: str) -> object:
    """按 `a.b.c` 路径取嵌套字段。"""
    current = payload
    for part in path.split("."):
        if not part:
            continue
        if isinstance(current, dict):
            current = current.get(part)
        elif isinstance(current, list):
            try:
                current = current[int(part)]
            except (ValueError, IndexError):
                return None
        else:
            return None
    return current


def _pick_field(row: dict, path: str) -> object:
    if not path:
        return ""
    return _walk_path(row, path) or ""


def _normalize_date(raw: object) -> str:
    if raw is None:
        return ""
    text = str(raw).strip()
    if not text:
        return ""
    match = DATE_IN_TEXT.search(text)
    if match:
        year, month, day = match.groups()
        return f"{int(year):04d}-{int(month):02d}-{int(day):02d}"
    return text[:10]
