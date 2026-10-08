# Tasks

## 1. 采集服务（tender-collection）

- [x] 1.1 创建 `tender-collection/` 目录骨架（app、pipeline、recipes、tests、requirements.txt、README.md、.dockerignore），`python -m compileall app tests` 通过
- [x] 1.2 实现 `/healthz`、`/sources`、`/discover`、`/extract`、`/collect`、`/probe` 六个接口，`app.main:app` 导入自检通过（6 条业务路由齐全）
- [x] 1.3 实现分级发现（接口 / 列表页 / 浏览器）与游标增量，游标往返单测通过（date:/url: 两种形态）
- [x] 1.4 实现通用正文抽取（trafilatura 主、readability 兜底）与规范文档组装，fixture 抽取单测通过
- [x] 1.5 实现限速、超时、重试与并发上限；`/collect` 单条抽取失败写入 errors 且不中断同轮其他条目
- [ ] 1.6 首批 12 个 recipe 已随代码落地；**仍需**在真实网络环境逐个用 `/probe` 验证并按结果收敛 `linkPattern` / `contentSelectors`（不通过则从同类站点补位）
- [ ] 1.7 `Dockerfile` 与 `docker-compose.yml` 的 `tender-intel-collector` 已就位，`docker buildx build --check` 与 `docker compose config` 通过；**仍需**实际构建镜像并验证容器健康检查

## 2. 数据库与 GORM

- [x] 2.1 在 `smart-bid` 建 11 张新表（`docs/sql/tender-intel.sql`，可重复执行），库内 FOREIGN KEY 数量为 0
- [x] 2.2 预置 14 个行业枚举与 12 个采集源，重复执行脚本后数量稳定
- [x] 2.3 扩展 `cmd/db/gen.go` 并重新生成 GORM 代码（model + query），`go build ./...` 通过

## 3. 全局模型配置

- [x] 3.1 实现 `pkg/repo/sysllm`（列表、候选筛选、Resolve 降级、CRUD、排序），`IsComplete` 过滤字段不完整配置、按 `sort ASC, id DESC` 取首条
- [x] 3.2 实现超管路由 `/api/sys/llm-config*`（列表/新增/修改/删除/排序/连通性测试/查看生效配置），非超管由 `IsSuperAdmin` 拦截返回 403

## 4. 后端情报站

- [x] 4.1 实现 `pkg/repo/tenderintel` 数据访问层（公告、行业、源、批次、订阅、提醒、收藏、解读）
- [x] 4.2 实现采集 worker：调用采集服务 → `url_hash` 去重 → 落库/刷新末次发现时间 → 回写源游标与批次明细；重复公告命中唯一键只更新 `last_seen_at`
- [x] 4.3 实现规则抽字段 + 全局模型批量打标（每批 20 条）；模型不可用时回退关键词打标并把公告标为 `tag_status=failed`，数据不丢
- [x] 4.4 实现订阅匹配与提醒生成，`MatchSubscription` 单测覆盖任一/全部命中、行业不匹配、预算边界、无预算不排除
- [x] 4.5 实现情报大厅接口（列表、详情、筛选项、收藏、解析预填）与 AI 解读全局缓存
- [x] 4.6 实现订阅与提醒接口（CRUD、启停、自然语言解析、提醒列表、标记已读、未读数）
- [x] 4.7 注册路由、两个任务队列（`tender_intel_collect` / `tender_intel_enrich`）与 cron 轮次调度（Redis 锁 + DB 防重入）；`go build ./...`、`go vet ./...`、`go test ./... -run TestNonexistent` 均通过
- [x] 4.8 扩展 `/api/home/stats` 返回 `tender_intel{today_new, unread_alerts, subscription_count}`

## 5. 前端

- [x] 5.1 新增 `service/intel.ts`（类型 + hooks），`npx tsc --noEmit` 通过
- [x] 5.2 新增情报大厅页（筛选栏含关键词/行业/地区/类型/来源/日期/预算/只看收藏、列表、分页、空/错状态）
- [x] 5.3 新增情报详情页（正文 Markdown、附件链接、收藏、AI 解读、发起招标解析）
- [x] 5.4 新增我的订阅页（规则 CRUD + 启停 + 自然语言解析回填草稿）
- [x] 5.5 新增提醒中心页与左导航未读角标（`useIntelUnreadCount` 30s 轮询 + 聚焦刷新）
- [x] 5.6 新增「系统管理 → 系统模型配置」与「采集源状态」页面（沿用 `isAdmin` 导航过滤，普通用户不可见）
- [ ] 5.7 左导航与首页情报卡片已更新，`yarn build` 通过；**仍需**真机核对窄屏 320/375/414/768 无页面级横向滚动
- [x] 5.8 招标解析创建入口支持预填：后端 `CreateProject` 接受可选 `name`，前端从 `?prefill_name/prefill_url/prefill_publisher` 展示情报上下文并用于项目命名

## 6. 收尾自检

- [x] 6.1 后端：改动文件 `gofmt -l` 无输出、`go build ./...`、`go vet ./...`、`go test ./... -run TestNonexistent` 全部通过
- [x] 6.2 采集服务：`python -m compileall`、`app.main:app` 导入冒烟、`python -m unittest discover -s tests`（8 例）全部通过
- [x] 6.3 前端：`npx tsc --noEmit`、`npx eslint`（新增/改动文件，`--max-warnings=0`）、`yarn build` 全部通过
- [x] 6.4 更新根 `README.md`、`docker-compose.yml`、`start.sh` 与 `tender-collection/README.md`（新增基础服务与端口说明）

## 7. 验收反馈后的补充（第二轮）

- [x] 7.1 自动采集任务配置化：新增 `tender_intel_collect_schedule` 单行配置表（默认每天 06:00），提供 `GET/PUT /zb/intel/schedule`，保存后热生效并返回下次执行时间；后端启动与保存时都会按库中配置重新注册 cron
- [x] 7.2 统一招标情报管理入口：系统管理新增「招标情报管理」（`/system/intel`），用子 tab 切换采集源状态 / 采集批次清单 / 采集任务配置；右上角保留“手动采集一轮”，并在其旁边新增“管理自动采集任务”直达任务配置 tab
- [x] 7.3 采集源清单能力增强：新增编辑（名称/地址/分类/地区/发现方式/是否需浏览器/优先级/params）、删除、单源立即采集、联通性探测（调用采集服务 `/discover` 返回可达性与候选公告样例）
- [x] 7.4 采集源导入：新增 `GET /zb/intel/sources/template`（CSV 模板，含示例行与列说明）与 `POST /zb/intel/sources/import`（支持 CSV/XLSX，逐行校验并返回失败原因，支持覆盖开关）
- [x] 7.5 采集服务支持无代码 recipe 的导入源：缺少 recipe 时回退通用规则，并支持用数据库 `params` 覆盖 `linkPattern` / `contentSelectors` / `maxItems` 等字段（单测覆盖）
- [x] 7.6 文案规范：`AGENTS.md` 新增第 10 节，禁止在用户可见文案中使用 `「」` / `『』`；清理本模块前后端全部相关文案
- [x] 7.7 采集服务镜像使用显式 tag `tender-intel-collector:v1`（compose `image:` + Dockerfile LABEL），并补充构建/启动命令注释
- [x] 7.8 修复共享页头把 React 节点包进 `Text`（`<p>`）导致的 hydration 报错与按钮挤压：节点型 activity 直接作为兄弟节点渲染，并新增回归测试与 `titleSuffix` 开关
- [x] 7.9 订阅表单补齐：公告类型/地区改为后端静态枚举（不再依赖库中已有数据），行业支持自由输入 + 常用建议，弹窗加宽并按分区两列布局
- [x] 7.10 破坏性操作二次确认：采集源删除、订阅删除均改为先弹 `DeleteConfirmModal`（写明对象、后果与不可恢复提示），确认后才调用接口
- [x] 7.11 修复采集请求序列化缺陷：Go 端空关键词切片曾序列化为 `"keywords": null`，导致采集服务的 `/collect` 与 `/discover` 全部 422（探测先暴露）；Go 侧改用 `omitempty`，采集服务侧同时把显式 null 按缺省处理，两侧各加回归测试
- [x] 7.12 页面补充操作语义说明：采集源状态页说明“探测/立即采集/编辑/删除”各自的作用与影响
- [x] 7.13 采集批次升级为“采集任务历史”：批次新增 `scope` / `source_keys` / `retry_of` 字段（含幂等 ALTER 升级脚本），列表显式展示采集范围与重试来源
- [x] 7.14 批次管理能力：新增 `POST /zb/intel/runs/:runId/retry`（按原范围重新发起并记录 `retry_of`，原批次保留）与 `DELETE /zb/intel/runs/:runId`（删除执行历史与源明细，公告不受影响，前端二次确认）
- [x] 7.15 修复批次卡死导致后续采集全被拒绝：触发前自动把超过 30 分钟仍未结束的 running 批次收尾为失败；防重入改为按范围判定（整轮采集互斥、单源触发只与该源冲突），并新增仓库层单测
- [x] 7.16 管理页交互调整：子 tab 采用显式选中胶囊样式；tab 更名为“采集源管理 / 采集任务历史 / 采集任务配置”；源数量与启用数移入页面正文；“手动采集一轮”移到“采集源管理”页右上角并与“导入采集源”并列；删除多余的“管理自动采集任务”按钮
- [x] 7.17 采集源管理改为卡片式列表（纯前端重构，功能不变）：复用招标解析列表页设计语言（`DataSurface` 纸面卡片 + 健康度驱动描边 + 等宽数字 + 底部动作行），网格 `repeat(auto-fill,minmax(min(100%,340px),1fr))` 自适应单列到多列；探测/立即采集/编辑/删除保留并加 Tooltip，启用开关改为卡片内可见状态；响应式审计与生产构建通过
- [x] 7.18 采集任务历史布局重构：说明文案与「刷新」并入同一容器并默认折叠（折叠态只渲染刷新按钮），展开后分条陈述口径与操作含义；表格操作列改为「查看明细 + 重试/删除图标按钮（Tooltip）」且禁止换行；列宽改为自适应 + 容器内横向滚动；新增列显示配置（复用 `ColumnConfig`，支持记忆到 localStorage 按用户区分、可恢复默认列），默认渲染批次/采集范围/状态/触发方式/操作
- [x] 7.19 `ColumnConfig` 组件增补可选能力（triggerProps 自定义触发按钮尺寸、onReset 恢复默认），并清理其历史未使用导入；既有调用方行为不变
- [x] 7.20 修复提醒中心打不开（Maximum update depth exceeded）：hooks 在数据未返回时用 `data?.data?.list || []` 兜底，每次渲染都产生新数组引用，页面又用 `useEffect([alerts])` 把它 setState 进本地 state，形成无限更新循环；改为模块级 `EMPTY_*` 常量兜底 + 页面直接渲染 hook 数据（删除副本 state），并新增静态守卫测试 `npm run test:intel`（禁止不稳定兜底与“集合拷贝进 state”写法）

## 8. 验收反馈后的补充（第三轮）

- [x] 8.1 修复采集任务负载序列化缺陷（“触发成功却查不到批次”的根因）：`startCollectRound` / `maybeDispatchEnrich` 把 `json.Marshal` 得到的 `[]byte` 交给 `queue.Enqueue`，被二次序列化成 base64 字符串（形如 `"eyJydW5faWQiOi..."`），worker 反序列化必然失败，任务全部进 DLQ、批次永久停在 `running`；改为直接投递结构体，`decodeTaskPayload` 同时兼容历史 base64 负载并补回归测试
- [x] 8.2 批次收尾兜底与统计口径修正：采集/打标任务重试耗尽时经 `SetOnTaskFinal` 回写失败明细、按 `tender_intel_run_source` 重算批次统计（重试导致的累加漂移不再出现）并落终态，杜绝僵死“执行中”批次继续阻塞后续触发；`GET /zb/intel/runs` 增补 `summary`（总/执行中/部分成功/失败）供管理页概览直接使用
- [x] 8.3 管理页提示与概览优化：`/system/intel` 所有写操作显式判断业务错误码（HTTP 200 + `code != 0`），失败不再被当成“已启动”，并把“已有采集批次正在执行”等真实原因展示出来；采集任务历史折叠态左侧常驻批次概览（共/执行中/部分成功/失败 + 最近采集与下次自动采集），存在执行中批次时每 6s 自动刷新，切到该 tab 时重新拉取最新批次

## 9. 全链路追踪（run_20260921T191032_251000）

- [x] 9.1 追踪结论：采集链路本身已通。`run_20260921T191032_251000`（重试自 18:34 批次，单源 ccgp_central）采集用时 84s（发现/抽取/入库各 35 条），随后 `tender_intel_enrich` 打标用时约 2.3 分钟（2 批 LLM，每批 20 条），批次于 19:14:20 落终态 `success`（source_success 1/1、inserted 35、enriched 35）。此前“全部失败”是 base64 负载缺陷遗留的 3 条历史批次，已修复并收尾
- [x] 9.2 采集服务发现阶段收窄：列表页混入的导航/政策动态/购买服务资讯链接（本次 15/35 条）会被当成公告；`_extract_links` 新增「优先保留与列表页同目录子树下的链接」规则，并把 `ccgp_central` 的 `linkPattern` 限定到 `/cggg/zygg/<栏目>/`，实测该列表页从 35 条噪声收敛为 20 条真实公告（含单测）
- [x] 9.3 发布时间修正：ccgp 的“公告时间”位于正文容器之外的页头表格，正文抽取拿不到，导致无标签兜底把“投标截止时间”写成发布时间（例：北京邮电大学项目被写成 2026-10-12）。`extract_fields` 增加 `page_text` 入参，优先按标签在整页文本定位；无标签兜底只在正文开头找且排除截止/开标语境，宁缺勿错（含单测，真实页面验证回 2026-09-21）
- [x] 9.4 库内数据修正：把误采的 15 条非公告行置为 `archived`（情报大厅按 `status=normal` 过滤，不再展示）；对保留的 20 条按修正后的规则回填 `publish_date`，当前 20 条公告均有发布时间与采购人
- [x] 9.5 执行中状态可视化：采集任务历史的状态列在 `running` 时补充实时口径（“已入库 N 条，正在打标”/“正在采集源站” + 已运行分钟数），配合 6s 轮询让长耗时批次不再是黑盒

## 10. 验收反馈后的补充（第四轮：局部刷新、筛选修复与情报管理）

- [x] 10.1 采集源启停/编辑改为局部刷新：`/system/intel` 采集源列表只有首次加载才渲染骨架屏，启停改为乐观值（`pendingEnabled`）先行更新开关、成功后后台重取再对齐，避免整列表被替换导致的焦点丢失与滚动跳变，并给出启停成功的文案反馈
- [x] 10.2 下拉框统一换用自绘组件 `IntelSelect`：原生 `Select` 的选项列表由操作系统渲染、无法继承设计令牌，改为 Chakra Menu 面板（与触发器等宽、选中态对勾、hover/focus/disabled/empty 全状态、44px 触控目标），应用于采集源“发现方式”、情报大厅“地区/公告类型/来源站/采集时间”与情报管理“状态/来源类型”
- [x] 10.3 修复情报大厅筛选条件不生效（真实缺陷）：axios 默认把数组序列化为 `industries[]=a`，而后端用 `c.QueryArray("industries")` 读取，键名不一致导致行业/地区/公告类型/来源四项筛选静默失效；`service/intel.ts` 统一使用 `paramsSerializer: { indexes: null }`（重复键写法），并新增静态守卫测试钉死该口径
- [x] 10.4 筛选条件自动生效：去掉“筛选”按钮，所有条件（关键词、行业、地区、公告类型、来源、发布时间、预算、采集时间、收藏开关）统一防抖 350ms 后自动请求，并回到第一页；筛选栏右上角常驻“正在筛选/已自动应用 N 个条件”状态，保留“重置”
- [x] 10.5 日期条件语义化：原有两个 date 控件只写占位符，用户无法分辨口径；改为带字段名的“发布时间 从/到”，并新增“采集时间”预设（今天/近 3 天/近 7 天/近 30 天，对应后端 `collect_within_days`，按服务器时区换算）
- [x] 10.6 预算筛选组件升级：由裸文本框改为正整数 `NumberInput`（步进加减、万元单位、提交前换算为元），并按 `下限(万元) / 上限(万元)` 成对标注
- [x] 10.7 情报管理子 tab（超管）：`/system/intel` 新增“情报管理”，列表口径与情报大厅一致（复用同一张 `IntelNoticeCard`），支持置顶/取消置顶、隐藏、下架、恢复、编辑、删除（二次确认），并提供多选批量操作（批量置顶/隐藏/下架/恢复/删除）
- [x] 10.8 手工发布与批量导入：新增 `POST/PUT /zb/intel/admin/notices`（手工发布与编辑，来源标记为“系统录入”，链接留空时生成 `manual://` 占位标识）、`POST /zb/intel/admin/notices/status|pin|delete`（单条与批量共用）、`GET /zb/intel/admin/notices/template` 与 `POST /zb/intel/admin/notices/import`（CSV/XLSX，逐行校验、按来源链接幂等、返回导入批次号）
- [x] 10.9 数据模型扩展：`tender_intel_notice` 新增 `origin`（collect/manual）、`import_batch`、`pinned`、`pinned_at`、`admin_note` 与 `idx_status_pinned_publish` / `idx_origin_batch` / `idx_first_seen_at`，补齐幂等 ALTER 升级脚本并重跑 `make gen`；情报大厅列表改为置顶优先排序，手工录入的情报同样参与订阅匹配并进入筛选来源（“系统录入”）
- [x] 10.10 管理能力与说明配套：情报管理工具栏默认折叠（折叠态只渲染概览统计与刷新/批量导入/发布情报），展开后分条说明置顶、隐藏、下架、发布、导入、删除的语义与后果
- [x] 10.11 本轮自检：后端 `gofmt -l` 无输出、`go build ./...`、`go vet`、`go test ./pkg/handler/tenderintel/... ./pkg/repo/tenderintel/... ./pkg/router/...`（新增公告仓储单测 7 例、导入解析单测 6 例）全部通过；前端 `npx tsc --noEmit`、`npx eslint --max-warnings=0`、`npm run test:intel`（4 例）全部通过

## 11. 验收反馈后的补充（第五轮：下拉框可见性、二次确认、卡片布局与滑动分页）

- [x] 11.1 修复弹窗内下拉框“没有数据”（真实缺陷）：Chakra `Menu` 的定位层 z-index 取自主题 `Menu.list`（值为 1），在弹窗（遮罩 z-index 1400）内面板确实渲染但被遮罩盖住，表现为选项为空；`IntelSelect` 显式把定位层抬到 `popover`(1500)，采集源“发现方式”、公告类型等弹窗内下拉恢复正常
- [x] 11.2 隐藏 / 下架的二次确认：单条与批量隐藏、下架都改为先弹确认弹窗（写明影响范围、可从“状态=已隐藏/已下架”找回、历史保留），确认后才调用接口；删除（单条/批量）沿用原有二次确认
- [x] 11.3 情报管理卡片选择框移到右上角：不再占用标题左侧空间，长标题不再被挤压换行，触控目标保持 44px
- [x] 11.4 分页改滑动分页（隐式分页）：情报大厅与情报管理都移除分页按钮，改用 `useInfiniteList` + `InfiniteScrollList`，绑定页面滚动容器（`intel-hall-scroll` / `intel-admin-scroll`）滚到底部自动加载下一页，加载完展示“已加载全部”；加载失败在列表底部给出重试入口，首屏之外的刷新不再闪骨架屏
- [x] 11.5 共享 hooks 增强：`useInfiniteList` 的错误文案同时识别 `msg` 与 `message`（情报站后端用 `message`），`loadMore` 开始前清空上一次错误，避免重试成功后仍显示错误态
- [x] 11.6 本轮自检：前端 `npx tsc --noEmit`、`npx eslint --max-warnings=0`（intel 相关文件与 hooks）、`npm run test:intel`（4 例）全部通过；另用临时 jsdom 渲染环境（仓库外，不入库）验证下拉面板 z-index 已解析为 `var(--chakra-zIndices-popover)`，以及滑动分页能累积 25 条数据并渲染“已加载全部”

## 12. 验收反馈后的补充（第六轮：回到顶部与详情返回来路）

- [x] 12.1 新增可复用的“回到顶部”按钮 `components/common/scroll-to-top.tsx`：滚动容器滚过阈值（默认 400px）后淡入，点击平滑回顶；`prefers-reduced-motion` 时改为即时定位；隐藏态 `aria-hidden` + `tabIndex=-1` + `pointer-events:none`，不做无意义焦点停留；无 `Element.scrollTo` 的环境（老浏览器 / jsdom）退化为直接设置 `scrollTop`
- [x] 12.2 情报大厅（滚动容器 `intel-hall-scroll`）与情报管理（`intel-admin-scroll`，由 `/system/intel` 的 PageViewport 提供）各自挂载回到顶部按钮，绑定自身滚动容器
- [x] 12.3 修复详情页返回丢失来路（真实反馈）：列表卡片在详情链接上带 `from`（情报大厅 `hall` / 情报管理 `system-intel` / 提醒中心 `alerts`），详情页按来路解析返回目标与按钮文案（回到情报大厅 / 返回情报管理 / 返回提醒中心），从情报管理点进详情后返回会回到“招标情报管理 → 情报管理”子 tab
- [x] 12.4 本轮自检：前端 `npx tsc --noEmit`、`npx eslint --max-warnings=0` 通过；`npm run test:intel` 新增两条静态守卫（回到顶部按钮与滚动容器绑定、详情返回来路映射）后共 6 例全部通过；另用仓库外临时 jsdom 环境验证回到顶部按钮的显示/隐藏与点击回顶行为（初始隐藏 → 滚动后可见 → 点击后 scrollTop=0 → 再次隐藏）

## 13. 验收反馈后的补充（第七轮：详情路由归属与左导航不跳模块）

- [x] 13.1 修复“从情报管理点进详情，左导航跳到招标情报站-情报大厅”（真实反馈）：根因是详情页只有一个路由 `/intel/[id]`，左导航按路径前缀匹配，`/intel/42` 必然归属 `/intel`。把详情视图抽成 `components/intel/notice-detail-view.tsx`，并为情报管理新增独立路由 `/system/intel/notices/[id]`：路径本身就表达管理上下文，左导航自然定位到“系统管理 → 招标情报管理”，且无需依赖查询参数、无刷新闪烁
- [x] 13.2 详情路由分工：`/intel/[id]` 承载情报大厅与提醒中心（按 `from` 决定返回目标），`/system/intel/notices/[id]` 承载情报管理（返回目标固定为“返回情报管理”→ `/system/intel?tab=notices`）；列表卡片新增 `detailBasePath` 以声明详情路由前缀
- [x] 13.3 左导航匹配规则可测试化：把 `findBestMatchHref` / `nodeIsActive` 抽到 `components/layout/nav-match.mjs`（与 `responsive-layout.mjs` 等保持一致的组织方式），并新增 `components/layout/nav-match.test.mjs`，断言 `/intel/42` 归属情报大厅、`/system/intel/notices/42` 归属招标情报管理且**不**命中 `/intel`、更具体的子菜单优先
- [x] 13.4 本轮自检：`npx tsc --noEmit`、`npx eslint --max-warnings=0`（改动文件）通过；`npm run test:intel`（6 例）与 `node --test components/layout/nav-match.test.mjs`（4 例）全部通过；另用仓库外临时 jsdom 环境分别渲染两个详情路由，验证返回控件分别渲染为 `返回情报管理 → /system/intel?tab=notices` 与 `返回情报大厅 → /intel`

## 14. 验收反馈后的补充（第八轮：筛选栏默认折叠）

- [x] 14.1 情报大厅与情报管理的筛选栏默认折叠：`IntelFilterBar` 新增 `collapsible` 开关，展开状态只属于本次会话（不持久化），折叠态与展开态之间用 `Collapse` 过渡；展开区头部保留“已自动应用 N 个条件 / 正在筛选”与“重置”
- [x] 14.2 折叠态内容按角色区分：情报大厅折叠时只保留关键词搜索框（收藏开关、行业、地区、公告类型、来源、发布时间、预算、采集时间全部收起）；情报管理折叠时一行内保留搜索框 + 状态下拉 + 来源类型下拉（通过新增的 `primaryExtras` 常驻插槽实现，展开后仍可见）
- [x] 14.3 折叠入口的可见性兜底：折叠态下若已有生效条件，展开按钮带数量角标（例如“展开筛选 3”），加载时按钮显示“筛选中”，保证收起后仍能感知当前筛选状态
- [x] 14.4 本轮自检：`npx tsc --noEmit`、`npx eslint --max-warnings=0`（改动文件）通过；`npm run test:intel`（新增折叠守卫 1 例，共 7 例）与 `node --test components/layout/nav-match.test.mjs`（4 例）全部通过；另用仓库外临时 jsdom 环境验证折叠态结构：大厅折叠态关键词在常驻行、地区/收藏在折叠面板内（面板 height=0/display=none）；管理折叠态关键词 + 状态 + 来源类型均在常驻行、地区在折叠面板内；点击后 `aria-expanded` 变 true 且面板 height=auto/display=block

- [x] 14.5 回归修正：上一轮为了让管理端三个控件同排，把关键词输入框误改成 `flex: 1` 且去掉宽度上限，输入框被撑满整行（视觉回归）。恢复为改动前的尺寸口径：`flex: 0 1 420px / max-width: 420px`（只在空间不足时收缩，永不放大），折叠行改为 `wrap` 换行而不是撑宽控件；jsdom 复核生成样式为 `flex: 0 1 420px; min-width: 220px; max-width: 420px`

## 15. 验收反馈后的补充（第九轮：筛选栏文案与按钮对齐）

- [x] 15.1 去掉无信息量的状态占位文案：筛选栏不再显示“条件变更后自动筛选”；仅在“正在筛选”或存在生效条件时渲染状态文案（“已自动应用 N 个条件”），无生效条件时该位置不渲染任何内容
- [x] 15.2 按钮统一右对齐：折叠与展开状态下的“展开筛选 / 收起筛选”按钮均 `margin-left: auto` 贴右；“重置”在展开区头部右对齐（仅有重置时 `justify-content: flex-end`；同时存在状态文案时文案在左、重置在右，`space-between`）
- [x] 15.3 本轮自检：`npx tsc --noEmit`、`npx eslint --max-warnings=0` 通过；`npm run test:intel`（7 例）与 `node --test components/layout/nav-match.test.mjs`（4 例）全部通过；另用仓库外临时 jsdom 环境核对生成样式：DOM 中已无“条件变更后自动筛选”、展开/收起按钮为 `margin-left: auto`、无生效条件时重置行 `justify-content: flex-end` 且仅含重置、有生效条件时为 `space-between`（左“已自动应用 1 个条件”/右“重置”）
