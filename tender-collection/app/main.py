"""招标情报采集服务 HTTP 入口。

接口契约（与后端 bid-engine 的 tenderintel 模块一一对应）：
- GET  /healthz    健康检查（含浏览器是否就绪）
- GET  /sources    支持的源与能力声明
- POST /discover   只做发现，返回详情页链接与游标
- POST /extract    单条 URL → 规范公告文档
- POST /collect    组合接口：发现 + 逐条抽取（后端每轮每源调用一次）
- POST /probe      单 URL 连通性与抽取样例（接入验证与排障）
"""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from . import __version__
from .config import settings
from .discover import discover
from .extract import extract_document
from .fetcher import fetcher
from .recipes import registry, resolve_recipe
from .schemas import (
    CollectRequest,
    CollectResponse,
    DiscoverRequest,
    DiscoverResponse,
    ExtractRequest,
    HealthResponse,
    ItemError,
    NoticeDocument,
    ProbeRequest,
    ProbeResponse,
    SourceInfo,
)

logging.basicConfig(
    level=getattr(logging, settings.log_level.upper(), logging.INFO),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(_: FastAPI):
    logger.info("tender-collection 启动，recipe 数量=%d", len(registry))
    try:
        yield
    finally:
        await fetcher.close()
        logger.info("tender-collection 关闭")


app = FastAPI(
    title="Tender Collection Service",
    description="标擎 · 招标情报采集服务：站点 → 规范公告文档",
    version=__version__,
    lifespan=lifespan,
)


@app.get("/healthz", response_model=HealthResponse)
async def healthz() -> HealthResponse:
    """健康检查。浏览器未启动时不算故障（仅静态抓取也可工作）。"""
    return HealthResponse(
        status="ok",
        version=__version__,
        browser_ready=fetcher.browser.ready,
        sources=len(registry),
    )


@app.get("/sources", response_model=list[SourceInfo])
async def list_sources() -> list[SourceInfo]:
    """返回已加载的源与能力声明。"""
    return [
        SourceInfo(
            source_key=recipe.source_key,
            name=recipe.name,
            homepage=recipe.homepage,
            discovery_mode=recipe.discovery_mode,
            needs_browser=recipe.needs_browser,
            enabled=True,
        )
        for recipe in registry.all()
    ]


@app.post("/discover", response_model=DiscoverResponse)
async def discover_endpoint(req: DiscoverRequest) -> DiscoverResponse:
    """只做发现：返回详情页候选与新的增量游标。"""
    recipe = resolve_recipe(
        req.source_key,
        req.list_url,
        req.needs_browser,
        req.discovery_mode,
        req.params,
    )
    list_url = req.list_url or (recipe.list_urls[0] if recipe.list_urls else recipe.homepage)
    items, cursor, errors = await discover(
        recipe,
        fetcher,
        list_url=list_url,
        cursor=req.cursor,
        max_pages=req.max_pages or recipe.max_pages,
        max_items=settings.max_items_per_source,
    )
    return DiscoverResponse(source_key=req.source_key, cursor=cursor, items=items, errors=errors)


@app.post("/extract", response_model=NoticeDocument)
async def extract_endpoint(req: ExtractRequest) -> NoticeDocument:
    """单条 URL → 规范公告文档。"""
    recipe = (
        resolve_recipe(req.source_key, "", req.needs_browser, "", "")
        if req.source_key
        else None
    )
    return await extract_document(
        recipe,
        fetcher,
        url=req.url,
        source_key=req.source_key,
        source_name=recipe.name if recipe else "",
        needs_browser=req.needs_browser,
        include_raw_html=req.include_raw_html,
    )


@app.post("/collect", response_model=CollectResponse)
async def collect_endpoint(req: CollectRequest) -> CollectResponse:
    """组合采集：发现新公告后逐条抽取，返回规范文档列表与新游标。"""
    recipe = resolve_recipe(
        req.source_key,
        req.list_url,
        req.needs_browser,
        req.discovery_mode,
        req.params,
    )
    if not (req.list_url or recipe.list_urls or recipe.homepage):
        return CollectResponse(
            source_key=req.source_key,
            cursor=req.cursor,
            errors=[ItemError(url=req.source_key, reason="该采集源没有配置列表地址")],
        )

    list_url = req.list_url or (recipe.list_urls[0] if recipe.list_urls else recipe.homepage)
    max_items = req.max_items or recipe.max_items or settings.max_items_per_source
    items, cursor, errors = await discover(
        recipe,
        fetcher,
        list_url=list_url,
        cursor=req.cursor,
        max_pages=req.max_pages or recipe.max_pages,
        max_items=max_items,
    )

    documents: list[NoticeDocument] = []
    for item in items:
        try:
            document = await extract_document(
                recipe,
                fetcher,
                url=item.url,
                source_key=recipe.source_key,
                source_name=recipe.name,
                needs_browser=req.needs_browser or recipe.needs_browser,
                include_raw_html=req.include_raw_html,
            )
        except Exception as exc:  # noqa: BLE001 - 单条失败不影响同轮其他条目
            logger.warning("抽取失败 %s: %s", item.url, exc)
            errors.append(ItemError(url=item.url, reason=f"抽取失败：{exc}"))
            continue

        # 列表页标题作为兜底：详情页标题缺失时仍能入库
        if not document.title and item.title:
            document.title = item.title
        if not document.publish_date and item.publish_date:
            document.publish_date = item.publish_date
        document.source_key = document.source_key or req.source_key
        document.source_name = document.source_name or recipe.name
        documents.append(document)

    return CollectResponse(source_key=req.source_key, cursor=cursor, items=documents, errors=errors)


@app.post("/probe", response_model=ProbeResponse)
async def probe_endpoint(req: ProbeRequest) -> ProbeResponse:
    """连通性与抽取样例：用于新源接入验证与线上排障。"""
    recipe = (
        resolve_recipe(req.source_key, "", req.needs_browser, "", "")
        if req.source_key
        else None
    )
    document = await extract_document(
        recipe,
        fetcher,
        url=req.url,
        source_key=req.source_key,
        source_name=recipe.name if recipe else "",
        needs_browser=req.needs_browser or bool(recipe and recipe.needs_browser),
    )
    return ProbeResponse(
        url=req.url,
        reachable=bool(document.body_text),
        strategy=document.extract.strategy,
        title=document.title,
        body_chars=len(document.body_text or ""),
        warnings=document.extract.warnings,
        sample=(document.body_text or "")[:200],
    )
