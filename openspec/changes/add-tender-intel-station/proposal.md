# 招标情报站

## Why

标擎现有「招标解析 → 标书生成 → 投标审核」都发生在用户**已经拿到招标文件之后**。用户缺少一条前置链路：持续发现正在发布的招标公告，判断哪些值得跟进。没有这条链路，用户只能人工浏览十几个招标网站，覆盖面窄、时效差。

## What Changes

- 新增独立采集服务 `tender-collection/`：接口优先、通用正文抽取、无头浏览器兜底，把招标站点变成结构化「招标公告」文档。
- 后端新增「招标情报站」模块：定时多轮采集、清洗打标、增量入库、订阅匹配与站内提醒；情报库全平台共享。
- 新增 `system_llm_config` 全局模型配置表与超管页面；情报站全部 LLM 环节（打标、订阅自然语言解析、AI 解读）读取该表，不读用户级 `user_llm_config`。
- 前端新增一级导航「招标情报站」（情报大厅 / 我的订阅 / 提醒中心）、首页情报卡片、顶部未读铃铛；情报详情可预填信息一键发起招标解析。
- 新增 Docker 常驻基础服务 `tender-intel-collector`（端口 5011），与 Redis / MinIO / Docling / Doc-Converter 同级。

**非目标**：中标/结果公告、招标附件下载、邮件与短信推送、用户自建采集源、多租户隔离、搜索引擎中间件。

## Capabilities

### New Capabilities

- `tender-intel/collection`: 采集服务的发现/抽取能力与采集批次的可观测、增量与容错行为。
- `tender-intel/library`: 情报大厅的检索、筛选、详情、收藏与解析联动能力。
- `tender-intel/subscription-alert`: 订阅规则与站内提醒能力。
- `tender-intel/system-llm-config`: 全局模型配置的排序、选取、降级与超管管理能力。
- `tender-intel/notice-admin`: 超管的情报管理能力（置顶、隐藏、下架、恢复、编辑、删除、手工发布与批量导入）。

### Modified Capabilities

（无）

## Impact

- 新增服务：`tender-collection/`（Python + FastAPI + Playwright），纳入 `docker-compose.yml` 与 `start.sh` 健康检查提醒。
- 新增数据库表：`system_llm_config`、`tender_intel_source`、`tender_intel_collect_run`、`tender_intel_run_source`、`tender_intel_notice`、`tender_intel_notice_industry`、`tender_intel_industry`、`tender_intel_subscription`、`tender_intel_alert`、`tender_intel_favorite`、`tender_intel_notice_insight`。
- 后端：新增 `pkg/handler/tenderintel`、`pkg/repo/tenderintel`、`pkg/repo/sysllm`、两个任务队列、cron 轮次调度、`/api/zb/intel/*` 与 `/api/sys/llm-config/*` 路由、`/api/home/stats` 扩展。
- 前端：新增 `/intel`、`/intel/[id]`、`/intel/subscriptions`、`/intel/alerts`、`/system/llm-config/system` 页面与 `service/intel.ts`。
- 依赖：采集服务新增 Python 依赖（FastAPI / Playwright / trafilatura 等）；后端与前端不新增第三方依赖。
- 数据保留：招标公告保留 180 天，超期与原始 HTML 快照（30 天）由定时任务清理。
