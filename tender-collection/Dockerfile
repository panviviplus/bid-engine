# 招标情报采集服务镜像
#
# 说明：
# - 基于官方 Playwright Python 镜像，自带 Chromium 与系统依赖（避免自建浏览器依赖链）；
# - 本机为 Apple Silicon（arm64），优先使用官方 arm64 tag；
#   若目标环境只有 amd64，可在 docker-compose 中加 `platform: linux/amd64`（与 docling 一致）。
# - 采集服务只做「站点 → 规范文档」，不连数据库、不持存储凭据，因此镜像内无需任何业务配置。

ARG PLAYWRIGHT_IMAGE=mcr.microsoft.com/playwright/python:v1.47.0-jammy
FROM ${PLAYWRIGHT_IMAGE}

LABEL org.opencontainers.image.title="tender-intel-collector" \
      org.opencontainers.image.description="标擎 · 招标情报采集服务（站点 → 规范公告文档）" \
      org.opencontainers.image.version="v1"

WORKDIR /service

ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1
ENV PORT=5011
ENV RECIPES_DIR=/service/recipes
# 同域名串行 + 随机间隔，避免给上游站点造成压力
ENV MIN_DELAY_SEC=1.0
ENV MAX_DELAY_SEC=3.0
ENV REQUEST_TIMEOUT_SEC=30
ENV MAX_RETRIES=2
ENV MAX_ITEMS_PER_SOURCE=50
ENV MAX_BROWSER_CONTEXTS=2
ENV HEADLESS=true

RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo "$TZ" > /etc/timezone \
    && (apt-get update && apt-get install -y --no-install-recommends curl tzdata ca-certificates \
        && rm -rf /var/lib/apt/lists/* || true)

COPY requirements.txt ./
RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir -r requirements.txt

# 基础镜像自带的浏览器只对应“镜像 tag”的 Playwright 版本，而 requirements 用区间版本，
# 一旦 pip 解析到更新版 Playwright，就会与镜像内浏览器版本不匹配，launch 时报
# "Executable doesn't exist at /ms-playwright/..."。这里按实际安装的版本补装 Chromium，
# 幂等：已存在则跳过。装在 /ms-playwright（基础镜像已设 PLAYWRIGHT_BROWSERS_PATH）。
RUN python -m playwright install chromium

COPY app ./app
COPY recipes ./recipes

EXPOSE 5011

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://localhost:5011/healthz || exit 1

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "5011"]
