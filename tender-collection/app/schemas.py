"""跨服务契约：REST 请求 / 响应模型。

字段名使用 camelCase（与后端 Go 结构体保持一致），
Python 侧使用 snake_case，通过 pydantic alias 转换。
"""

from __future__ import annotations

from typing import Any, List, Optional, get_origin

from pydantic import BaseModel, ConfigDict, Field, field_validator
from pydantic.alias_generators import to_camel


class CamelModel(BaseModel):
    """统一 camelCase 输出、snake_case 入参的基类。"""

    model_config = ConfigDict(alias_generator=to_camel, populate_by_name=True)

    @field_validator("*", mode="before")
    @classmethod
    def _empty_string_for_none(cls, value: object, info) -> object:
        """把显式 null 当作缺省值处理。

        调用方（例如 Go 服务）可能把空切片/空值序列化成 null；
        服务端不应该因为这种写法直接 422，而是按缺省语义处理。
        """
        if value is not None:
            return value
        field = cls.model_fields.get(info.field_name)
        if field is None:
            return value
        annotation = field.annotation
        if annotation is str:
            return ""
        if get_origin(annotation) is list:
            return []
        return value


class Attachment(CamelModel):
    name: str = ""
    url: str = ""


class ExtractMeta(CamelModel):
    """抽取元信息：让调用方知道这条文档是怎么来的、可信度如何。"""

    strategy: str = ""
    confidence: float = 0.0
    warnings: List[str] = Field(default_factory=list)


class NoticeDocument(CamelModel):
    """规范化公告文档（采集服务的输出契约）。"""

    source_key: str = ""
    source_name: str = ""
    url: str = ""
    canonical_url: str = ""
    external_id: str = ""
    title: str = ""
    publisher_name: str = ""
    agency_name: str = ""
    project_code: str = ""
    budget_text: str = ""
    publish_date: str = ""
    deadline_text: str = ""
    region_text: str = ""
    notice_type_text: str = ""
    body_html: str = ""
    body_markdown: str = ""
    body_text: str = ""
    attachments: List[Attachment] = Field(default_factory=list)
    fetched_at: str = ""
    extract: ExtractMeta = Field(default_factory=ExtractMeta)
    raw_html: Optional[str] = None


class DiscoveredItem(CamelModel):
    url: str
    title: str = ""
    publish_date: str = ""


class ItemError(CamelModel):
    url: str
    reason: str


class DiscoverRequest(CamelModel):
    source_key: str
    list_url: str = ""
    discovery_mode: str = ""
    needs_browser: bool = False
    cursor: str = ""
    max_pages: int = 1
    keywords: List[str] = Field(default_factory=list)
    params: str = ""


class DiscoverResponse(CamelModel):
    source_key: str
    cursor: str = ""
    items: List[DiscoveredItem] = Field(default_factory=list)
    errors: List[ItemError] = Field(default_factory=list)


class ExtractRequest(CamelModel):
    url: str
    source_key: str = ""
    needs_browser: bool = False
    include_raw_html: bool = False


class CollectRequest(CamelModel):
    source_key: str
    list_url: str = ""
    discovery_mode: str = ""
    needs_browser: bool = False
    cursor: str = ""
    max_pages: int = 1
    max_items: int = 50
    keywords: List[str] = Field(default_factory=list)
    params: str = ""
    include_raw_html: bool = False


class CollectResponse(CamelModel):
    source_key: str
    cursor: str = ""
    items: List[NoticeDocument] = Field(default_factory=list)
    errors: List[ItemError] = Field(default_factory=list)


class ProbeRequest(CamelModel):
    url: str
    source_key: str = ""
    needs_browser: bool = False


class ProbeResponse(CamelModel):
    url: str
    reachable: bool
    status_code: int = 0
    strategy: str = ""
    title: str = ""
    body_chars: int = 0
    warnings: List[str] = Field(default_factory=list)
    sample: str = ""


class SourceInfo(CamelModel):
    source_key: str
    name: str
    homepage: str = ""
    discovery_mode: str = ""
    needs_browser: bool = False
    enabled: bool = True


class HealthResponse(CamelModel):
    status: str
    version: str
    browser_ready: bool
    sources: int


RecipeParams = dict[str, Any]
