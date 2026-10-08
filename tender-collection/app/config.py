"""采集服务运行配置。

所有可调项走环境变量，便于容器化部署与灰度调整，不在代码里硬编码。
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


def _env_str(key: str, default: str) -> str:
    value = os.environ.get(key, "").strip()
    return value or default


def _env_int(key: str, default: int) -> int:
    raw = os.environ.get(key, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _env_float(key: str, default: float) -> float:
    raw = os.environ.get(key, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


@dataclass(frozen=True)
class Settings:
    """服务配置快照。"""

    host: str
    port: int
    recipes_dir: Path
    request_timeout_sec: float
    max_retries: int
    min_delay_sec: float
    max_delay_sec: float
    max_items_per_source: int
    max_pages_per_request: int
    max_browser_contexts: int
    headless: bool
    user_agent: str
    log_level: str


def load_settings() -> Settings:
    """从环境变量加载配置。"""
    default_recipes = Path(__file__).resolve().parent.parent / "recipes"
    return Settings(
        host=_env_str("HOST", "0.0.0.0"),
        port=_env_int("PORT", 5011),
        recipes_dir=Path(_env_str("RECIPES_DIR", str(default_recipes))),
        # 单次请求超时（上游站点响应慢时由调用方队列重试兜底）
        request_timeout_sec=_env_float("REQUEST_TIMEOUT_SEC", 30.0),
        max_retries=_env_int("MAX_RETRIES", 2),
        # 同域名串行 + 随机间隔，避免给上游站点造成压力
        min_delay_sec=_env_float("MIN_DELAY_SEC", 1.0),
        max_delay_sec=_env_float("MAX_DELAY_SEC", 3.0),
        max_items_per_source=_env_int("MAX_ITEMS_PER_SOURCE", 50),
        max_pages_per_request=_env_int("MAX_PAGES_PER_REQUEST", 3),
        max_browser_contexts=_env_int("MAX_BROWSER_CONTEXTS", 2),
        headless=_env_str("HEADLESS", "true").lower() != "false",
        user_agent=_env_str(
            "USER_AGENT",
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 "
            "(KHTML, like Gecko) Chrome/126.0 Safari/537.36",
        ),
        log_level=_env_str("LOG_LEVEL", "info"),
    )


settings = load_settings()
