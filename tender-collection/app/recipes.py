"""站点适配规则（recipe）的加载与查询。

设计取舍：规则以 JSON 文件随服务代码版本管理、走代码评审，
数据库只保存「启停 / 列表地址覆盖 / 参数覆盖」等运行态信息。
这样上游站点改版时改动可追溯、可回滚。
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .config import settings

logger = logging.getLogger(__name__)


@dataclass
class Recipe:
    """单个采集源的适配规则。"""

    source_key: str
    name: str = ""
    homepage: str = ""
    category: str = ""
    region: str = ""
    discovery_mode: str = "list"          # api | list | browser
    needs_browser: bool = False
    list_urls: list[str] = field(default_factory=list)
    # 发现阶段
    link_pattern: str = ""                # 详情页链接的正则（留空表示全部链接）
    list_link_selector: str = ""          # 列表项链接选择器（留空走通用链接发现）
    list_exclude_pattern: str = ""        # 需要排除的链接正则
    api_method: str = "GET"
    api_body: dict[str, Any] = field(default_factory=dict)
    api_items_path: str = ""              # JSON 中的列表路径，如 data.rows
    api_field_map: dict[str, str] = field(default_factory=dict)  # 目标字段 -> JSON 路径
    # 详情阶段
    content_selectors: list[str] = field(default_factory=list)
    title_selectors: list[str] = field(default_factory=list)
    max_items: int = 50
    max_pages: int = 1
    keywords: list[str] = field(default_factory=list)

    @classmethod
    def from_dict(cls, source_key: str, data: dict[str, Any]) -> "Recipe":
        known = {f.name for f in cls.__dataclass_fields__.values()}  # type: ignore[attr-defined]
        payload = {k: v for k, v in data.items() if k in known}
        payload["source_key"] = payload.get("source_key") or source_key
        return cls(**payload)

    def apply_overrides(self, overrides: dict[str, Any]) -> "Recipe":
        """用数据库里的 params 覆盖规则字段，返回新实例（不改动代码里的 recipe）。"""
        if not overrides:
            return self
        allowed = {f.name for f in self.__dataclass_fields__.values()}  # type: ignore[attr-defined]
        merged = {name: getattr(self, name) for name in allowed}
        for raw_key, value in overrides.items():
            key = _to_snake(str(raw_key))
            if key not in allowed or key == "source_key":
                continue
            merged[key] = value
        return Recipe(**merged)


class RecipeRegistry:
    """加载 `recipes/*.json` 并提供查询。"""

    def __init__(self, recipes_dir: Path | None = None) -> None:
        self.recipes_dir = recipes_dir or settings.recipes_dir
        self._recipes: dict[str, Recipe] = {}
        self.reload()

    def reload(self) -> int:
        """重新加载全部 recipe，返回加载数量。"""
        recipes: dict[str, Recipe] = {}
        if not self.recipes_dir.exists():
            logger.warning("recipe 目录不存在: %s", self.recipes_dir)
            self._recipes = recipes
            return 0
        for path in sorted(self.recipes_dir.glob("*.json")):
            try:
                data = json.loads(path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError) as exc:
                logger.error("加载 recipe 失败 %s: %s", path.name, exc)
                continue
            source_key = data.get("sourceKey") or data.get("source_key") or path.stem
            # 兼容 camelCase 键名
            normalized = {_to_snake(k): v for k, v in data.items()}
            recipes[source_key] = Recipe.from_dict(source_key, normalized)
        self._recipes = recipes
        logger.info("已加载 %d 个采集 recipe", len(recipes))
        return len(recipes)

    def get(self, source_key: str) -> Recipe | None:
        return self._recipes.get(source_key)

    def all(self) -> list[Recipe]:
        return list(self._recipes.values())

    def __len__(self) -> int:
        return len(self._recipes)


def _to_snake(name: str) -> str:
    out = []
    for index, char in enumerate(name):
        if char.isupper() and index > 0:
            out.append("_")
        out.append(char.lower())
    return "".join(out)


def build_generic_recipe(
    source_key: str,
    list_url: str,
    needs_browser: bool = False,
    discovery_mode: str = "list",
) -> Recipe:
    """为「没有代码 recipe」的导入源构造通用规则。

    导入的源数量会持续增长，不可能每个都先写一份 recipe；
    这里用通用规则（浏览器渲染 + 通用链接发现）先跑起来，
    站点需要特殊处理时再用 params 覆盖，或补一份 recipe 走代码评审。
    """
    return Recipe(
        source_key=source_key or "generic",
        name=source_key or "通用采集源",
        discovery_mode=(discovery_mode or "list").lower(),
        needs_browser=needs_browser,
        list_urls=[list_url] if list_url else [],
    )


def resolve_recipe(
    source_key: str,
    list_url: str,
    needs_browser: bool = False,
    discovery_mode: str = "list",
    params: str = "",
) -> Recipe:
    """取得本次采集实际使用的规则：代码 recipe（如有）→ 通用规则 → params 覆盖。"""
    base = registry.get(source_key)
    if base is None:
        base = build_generic_recipe(source_key, list_url, needs_browser, discovery_mode)
    if not base.list_urls and list_url:
        base = base.apply_overrides({"list_urls": [list_url]})
    overrides = parse_params(params)
    return base.apply_overrides(overrides)


def parse_params(params: str) -> dict[str, Any]:
    """解析数据库里的 params（JSON 字符串），非法内容忽略并记日志。"""
    raw = (params or "").strip()
    if not raw:
        return {}
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        logger.warning("采集源 params 不是合法 JSON，已忽略: %s", exc)
        return {}
    if not isinstance(parsed, dict):
        logger.warning("采集源 params 需为 JSON 对象，已忽略")
        return {}
    return parsed


registry = RecipeRegistry()
