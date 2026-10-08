# tender-collection · 招标情报采集服务

标擎「招标情报站」的采集侧服务。职责边界只有一句话：

> **输入源站与查询条件，输出结构一致的「规范公告文档」。**

它不连接数据库、不持有对象存储凭据、不写任何业务状态。落库、去重、行业打标、
订阅匹配与提醒全部由后端（`bid-engine-backend/pkg/handler/tenderintel`）负责。
这样采集能力可以被未来的合同情报、政策情报等场景复用。

## 为什么不用「逐站正则爬虫」

早期方案（参考实现）为每个站点写死列表页/详情页正则，站点一改版就大面积失效。
本服务改为分级策略：

| 级别 | 方式 | 说明 |
|---|---|---|
| 1 | `api` | 站内搜索/列表 JSON 接口（模板化 URL + JSON 路径映射），最稳定 |
| 2 | `list` | 静态列表页：通用链接发现 + 可选站点选择器 |
| 3 | `browser` | Playwright 无头浏览器渲染，处理 JS 渲染与反爬 |
| 4 | `search` | 搜索 API 补长尾（接口已预留，本期未启用） |

取数同理：正文用通用正文抽取（trafilatura 主、readability 兜底），字段用「规则优先 +
后端 LLM 兜底」，正则只做校验，不再逐站手写详情页解析。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查（含 `browserReady`） |
| GET | `/sources` | 已加载的源与能力声明 |
| POST | `/discover` | 只做发现，返回详情页候选与增量游标 |
| POST | `/extract` | 单 URL → 规范公告文档 |
| POST | `/collect` | 组合接口：发现 + 逐条抽取（后端每轮每源调用一次） |
| POST | `/probe` | 单 URL 连通性与抽取样例，用于接入验证与排障 |

规范文档字段见 `app/schemas.py::NoticeDocument`，与后端
`pkg/handler/tenderintel/collector.go` 的 `collectorDoc` 一一对应，改动需两侧同步。

## 站点规则（recipes）

规则以 `recipes/*.json` 形式**随代码版本管理**（走代码评审，可追溯、可回滚）；
数据库只保存「启停 / 列表地址覆盖 / 参数覆盖」等运行态信息。这样上游站点改版时，
改动是一份可 review 的 diff，而不是散落在库里的数据。

recipe 关键字段：

```jsonc
{
  "sourceKey": "ccgp_central",          // 必须与 tender_intel_source.source_key 一致
  "discoveryMode": "list",              // api | list | browser
  "needsBrowser": false,                // 是否强制走无头浏览器
  "listUrls": ["http://.../cggg/zygg/"],
  "linkPattern": "/\\d{6}/t\\d{8}_\\d+\\.htm",  // 详情页链接正则（留空=不过滤）
  "listLinkSelector": "",               // 可选：列表项选择器
  "contentSelectors": [".vF_detail_content"],   // 可选：正文容器
  "maxItems": 50,
  "maxPages": 1
}
```

站点规则见 `recipes/`（每个 `recipes/*.json` 对应一个源，与库表 `tender_intel_source` 一一对应）。
**新增或调整源后必须用 `/probe` 验证**：

```bash
curl -s -X POST http://localhost:5011/probe \
  -H 'Content-Type: application/json' \
  -d '{"url":"http://www.ccgp.gov.cn/cggg/zygg/gkzb/202609/t20260921_27371425.htm","sourceKey":"ccgp_central"}' | jq
```

关注返回中的 `reachable`、`bodyChars`、`warnings`：
`bodyChars` 过小或出现「正文过短」告警说明该源需要补 `contentSelectors` 或改走浏览器。

## 本地运行

```bash
cd tender-collection
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
playwright install chromium        # 仅需要浏览器兜底的源才必需
uvicorn app.main:app --host 0.0.0.0 --port 5011
```

容器运行（与标擎其他基础服务同级）：

```bash
# 在项目根目录
docker compose -f docker-compose.yml up -d tender-intel-collector
```

## 测试

```bash
cd tender-collection
python -m pytest tests -q          # 或 python -m unittest discover -s tests
```

测试全部基于 `tests/fixtures/` 的本地 HTML，不访问网络，可在 CI 中稳定运行。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `5011` | 监听端口 |
| `RECIPES_DIR` | `/service/recipes` | recipe 目录 |
| `REQUEST_TIMEOUT_SEC` | `30` | 单次请求超时 |
| `MAX_RETRIES` | `2` | 静态请求重试次数（指数退避） |
| `MIN_DELAY_SEC` / `MAX_DELAY_SEC` | `1.0` / `3.0` | 同域名随机间隔 |
| `MAX_ITEMS_PER_SOURCE` | `50` | 每源每次抽取上限 |
| `MAX_BROWSER_CONTEXTS` | `2` | 浏览器上下文并发上限 |
| `HEADLESS` | `true` | 是否无头运行 |

## 合规与边界

- 只采集公开的招标公告页面，不尝试绕过登录与验证码；遇到 JS Challenge 直接记为失败。
- 同域名串行 + 随机间隔，不做高频扫描。
- 只记录附件**链接**，不下载文件（附件下载不在本期范围）。

## 已知待收敛项（接入期逐源验证）

已完成一次真实站点验证（中国政府采购网详情页：正文 2600+ 字、项目编号、预算、地区均正确抽取，
置信度 0.95）。以下两项属于「逐源调优」范畴，需在接入每个源时用 `/probe` 收敛：

1. **列表页链接规则**：目前仅 `ccgp_central` 已用真实页面验证 `linkPattern`；
   其余源先以「浏览器渲染 + 通用链接发现」保证可用，验证后再补精确规则以减少无效详情页访问。
2. **发布日期偏差**：正文里出现多个日期时，当前先按「发布时间/发布日期」等标签定位，
   定位不到才退回「首个日期」，可能命中获取文件时间或开标时间。逐源验证时可收紧
   `contentSelectors` 让抽取范围聚焦公告头部，使日期定位更准确。

另外，规则抽取会**主动弃用**明显跑偏的捕获值（例如把正文片段误当采购人），
这些字段会留空并由后端 LLM 兜底补齐，避免脏数据入库。
