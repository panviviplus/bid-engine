/**
 * 招标情报站前端静态守卫。
 *
 * 背景（真实线上问题）：接口数据未返回时，如果 hooks 用 `data?.data || []` 兜底，
 * 每次渲染都会得到新的数组引用；页面再把该数组 setState 进本地 state，
 * 就会形成“渲染 → 新数组 → useEffect → setState → 渲染”的无限循环，
 * React 抛出 Maximum update depth exceeded，页面直接打不开（提醒中心曾因此挂掉）。
 *
 * 这两条规则把该类写法钉死：
 *   1. service/intel.ts 的列表兜底必须是模块级常量，禁止 `|| []` / `|| {}`；
 *   2. 页面/组件禁止在 useEffect 内把接口 hook 返回的集合直接 setState。
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import test from "node:test";

const here = path.dirname(fileURLToPath(import.meta.url));
const projectRoot = path.resolve(here, "../..");

const INTEL_FILES = [
  "app/(main)/intel/page.tsx",
  "app/(main)/intel/[id]/page.tsx",
  "app/(main)/intel/subscriptions/page.tsx",
  "app/(main)/intel/alerts/page.tsx",
  "app/(main)/system/intel/page.tsx",
  "components/intel/filter-bar.tsx",
  "components/intel/alert-drawer.tsx",
  "components/intel/intel-select.tsx",
  "components/intel/notice-detail-view.tsx",
  "components/common/scroll-to-top.tsx",
  "components/intel/notice-card.tsx",
  "components/intel/subscription-editor.tsx",
  "components/intel/tag-field.tsx",
  "components/intel/admin/source-card.tsx",
  "components/intel/admin/source-editor-modal.tsx",
  "components/intel/admin/source-import-modal.tsx",
  "components/intel/admin/run-detail-modal.tsx",
  "components/intel/admin/notice-manager.tsx",
  "components/intel/admin/match-manager.tsx",
  "components/intel/admin/hint-toggle.tsx",
  "components/intel/admin/notice-editor-modal.tsx",
  "components/intel/admin/notice-import-modal.tsx",
  "app/(main)/system/intel/notices/[id]/page.tsx",
];

/** 接口集合名：把它们拷进本地 state 极易形成循环更新。 */
const COLLECTION_NAMES = [
  "alerts",
  "notices",
  "sources",
  "runs",
  "subscriptions",
  "configs",
  "filters",
];

function readSource(relativePath) {
  return readFileSync(path.join(projectRoot, relativePath), "utf8");
}

/**
 * 去掉注释后再扫描：本文件的作用就是描述这段坏代码的形态，
 * 注释里出现示例写法不能算违规（保持行号不变，便于定位）。
 */
function stripComments(source) {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .split("\n")
    .map((line) => (line.trimStart().startsWith("//") ? "" : line))
    .join("\n");
}

function lineOf(source, index) {
  return source.slice(0, index).split("\n").length;
}

test("service 层列表兜底必须是稳定引用，禁止每次渲染新建数组/对象", () => {
  const source = stripComments(readSource("service/intel.ts"));
  const offenders = [];
  const unstableFallback = /\|\|\s*(\[\]|\{\})/g;
  let match = unstableFallback.exec(source);
  while (match) {
    offenders.push(`line ${lineOf(source, match.index)}: ${match[0]}`);
    match = unstableFallback.exec(source);
  }
  assert.deepEqual(
    offenders,
    [],
    `service/intel.ts 出现不稳定兜底（会引发无限渲染），请改用模块级 EMPTY_* 常量：\n${offenders.join("\n")}`,
  );
});

test("页面不得在 useEffect 里把接口集合 setState 进本地 state", () => {
  const violations = [];
  const copyIntoState = new RegExp(
    `set[A-Z][A-Za-z0-9_]*\\(\\s*(${COLLECTION_NAMES.join("|")})\\s*\\)`,
    "g",
  );

  for (const file of INTEL_FILES) {
    const source = stripComments(readSource(file));
    let match = copyIntoState.exec(source);
    while (match) {
      violations.push(`${file}:${lineOf(source, match.index)} → ${match[0]}`);
      match = copyIntoState.exec(source);
    }
  }

  assert.deepEqual(
    violations,
    [],
    `禁止把接口集合拷贝进本地 state（改用 hook 返回值直接渲染）：\n${violations.join("\n")}`,
  );
});

/**
 * “我的订阅 / 提醒中心”已合并为“订阅与提醒”：
 * 提醒按订阅收敛在右侧抽屉里，旧路径保留为服务端重定向。
 */
test("提醒中心并入订阅与提醒，旧路径保留为服务端重定向", () => {
  const source = stripComments(readSource("app/(main)/intel/alerts/page.tsx"));
  assert.match(source, /redirect\("\/intel\/subscriptions"\)/);
  assert.doesNotMatch(
    source,
    /"use client"/,
    "重定向必须是服务端组件，否则会先渲染空页再跳转",
  );
});

test("合并页以订阅卡片为主体，提醒在抽屉里滚动加载", () => {
  const page = stripComments(
    readSource("app/(main)/intel/subscriptions/page.tsx"),
  );
  assert.match(page, /<AlertDrawer/);
  assert.match(
    page,
    /aria-haspopup="dialog"/,
    "订阅卡片需要可发现的抽屉入口（键盘可达）",
  );
  assert.doesNotMatch(page, /useState<IntelAlert\[\]>/);

  const drawer = stripComments(readSource("components/intel/alert-drawer.tsx"));
  assert.match(drawer, /useInfiniteList<IntelAlert>/);
  assert.match(drawer, /scrollableTarget=\{ALERT_DRAWER_SCROLL_ID\}/);
  assert.match(
    drawer,
    /id=\{ALERT_DRAWER_SCROLL_ID\}/,
    "无限滚动要绑定抽屉自身的滚动容器 id",
  );
  assert.doesNotMatch(
    drawer,
    /useState<IntelAlert\[\]>/,
    "抽屉直接渲染 hook 数据，不保留副本 state",
  );
});

test("提醒的已读 / 未读 / 删除走新接口，未读强调使用深海蓝", () => {
  const drawer = stripComments(readSource("components/intel/alert-drawer.tsx"));
  assert.match(drawer, /useIntelAlertStatus/);
  assert.match(drawer, /useIntelAlertDelete/);
  assert.match(
    drawer,
    /colorScheme=\{item\.is_read \? "neutral" : "primary"\}/,
    "未读徽标应使用 primary，不再用金色当功能色",
  );
  assert.match(drawer, /deleteDialog/, "删除提醒需要二次确认弹窗");
});

test("详情页可从订阅与提醒返回，并自动重开对应订阅抽屉", () => {
  const detail = stripComments(readSource("app/(main)/intel/[id]/page.tsx"));
  assert.match(detail, /subscriptions: subscriptionsBackTarget/);
  assert.match(
    detail,
    /subscription=\$\{id\}/,
    "返回链接需要带订阅 ID，页面据此重开抽屉",
  );
});

test("筛选栏与首个卡片留出间距，情报卡片两侧走统一小卡片表面", () => {
  const bar = stripComments(readSource("components/intel/filter-bar.tsx"));
  assert.match(bar, /mb=\{5\}/, "筛选栏需要与下方第一个卡片留出间距");

  const card = stripComments(readSource("components/intel/notice-card.tsx"));
  assert.match(card, /DataSurface/, "用户侧卡片复用共享表面组件");
  assert.match(card, /variant = "user"/, "卡片默认走用户侧口径");
  assert.doesNotMatch(
    card,
    /variant === "legacy"/,
    "管理侧不再保留旧的 legacy 表面分支",
  );

  const hall = stripComments(readSource("app/(main)/intel/page.tsx"));
  assert.match(
    hall,
    /repeat\(auto-fill, minmax\(min\(100%, 340px\), 1fr\)\)/,
    "情报大厅需要使用与采集源管理一致的小卡片网格",
  );

  const manager = stripComments(
    readSource("components/intel/admin/notice-manager.tsx"),
  );
  assert.match(
    manager,
    /repeat\(auto-fill, minmax\(min\(100%, 340px\), 1fr\)\)/,
    "情报管理需要使用与采集源管理一致的小卡片网格",
  );
  assert.match(manager, /variant="admin"/, "管理侧卡片走管理口径");
});

test("情报卡片标题不截断并按长度降字号，管理侧标签分层渲染", () => {
  const card = stripComments(readSource("components/intel/notice-card.tsx"));
  assert.match(card, /function titleFontSize\(title: string\)/);
  assert.match(card, /fontSize=\{titleFontSize\(title\)\}/);
  assert.match(
    card,
    /variant\?: "user" \| "admin"/,
    "variant 需要表达用户侧与管理侧两种口径",
  );
  assert.match(card, /isAdmin && notice\.pinned/, "置顶标签只能在管理侧渲染");
  assert.match(
    card,
    /\{isAdmin \? statusBadge : null\}/,
    "状态标签只能在管理侧渲染",
  );
  assert.match(
    card,
    /industryNames\.map/,
    "行业标签必须全量渲染，并单独占一行",
  );
});

/**
 * 数组型筛选条件必须用重复键序列化。
 *
 * 背景（真实缺陷）：axios 默认把数组序列化成 `industries[]=a`，而后端 Gin 用
 * `c.QueryArray("industries")` 读取，两者键名不一致，导致行业/地区/公告类型/来源
 * 四个筛选条件静默失效（接口照样返回 200，用户以为筛选没生效）。
 */
test("情报筛选的数组参数必须用重复键序列化，不能用默认的 [] 下标", () => {
  const source = stripComments(readSource("service/intel.ts"));
  assert.match(
    source,
    /intelParamsSerializer\s*=\s*\{\s*indexes:\s*null\s*\}/,
    "service/intel.ts 需要导出 indexes:null 的 paramsSerializer",
  );
  const serialized = source.match(/paramsSerializer:\s*([A-Za-z0-9_]+)/g) || [];
  assert.ok(
    serialized.length >= 2,
    "情报大厅列表与情报管理列表都必须显式指定 paramsSerializer",
  );
  for (const item of serialized) {
    assert.match(item, /intelParamsSerializer/);
  }
});

/**
 * 长列表（情报大厅 / 情报管理）必须能回到顶部。
 *
 * 两个页面都是滑动分页，滚到几十条后没有回到顶部的入口就只能手动拖滚动条；
 * 按钮必须绑定各自的滚动容器 id，否则监听不到滚动、点了也不会动。
 */
test("滑动分页页面都挂载回到顶部按钮并绑定各自的滚动容器", () => {
  const hall = stripComments(readSource("app/(main)/intel/page.tsx"));
  assert.match(hall, /const HALL_SCROLL_ID = "intel-hall-scroll"/);
  assert.match(hall, /id=\{HALL_SCROLL_ID\}/);
  assert.match(hall, /<ScrollToTopButton targetId=\{HALL_SCROLL_ID\}/);

  const manager = stripComments(
    readSource("components/intel/admin/notice-manager.tsx"),
  );
  assert.match(manager, /const MANAGER_SCROLL_ID = "intel-admin-scroll"/);
  assert.match(manager, /<ScrollToTopButton targetId=\{MANAGER_SCROLL_ID\}/);

  // 滚动容器 id 必须真的存在，否则按钮监听不到滚动
  const systemPage = stripComments(
    readSource("app/(main)/system/intel/page.tsx"),
  );
  assert.match(systemPage, /id="intel-admin-scroll"/);
});

/**
 * 筛选条件过多，两个页面都默认折叠，且折叠态保留各自的关键入口。
 *
 * 需求：情报大厅折叠时只留关键词搜索框；情报管理折叠时一行内保留
 * 搜索框 + 状态下拉 + 情报来源下拉。
 */
test("筛选栏默认折叠，且折叠态保留各自的关键条件", () => {
  const bar = stripComments(readSource("components/intel/filter-bar.tsx"));
  assert.match(
    bar,
    /useState\(!collapsible\)/,
    "折叠状态必须默认收起（collapsible 时 expanded 初始为 false）",
  );
  assert.match(bar, /primaryExtras/, "需要支持折叠态也常驻的条件");
  // 折叠态常驻内容必须在 #intel-filter-panel 之外
  assert.match(bar, /<Collapse in=\{expanded\}/);

  const hall = stripComments(readSource("app/(main)/intel/page.tsx"));
  assert.match(hall, /\n\s+collapsible\n/, "情报大厅需要可折叠");
  assert.doesNotMatch(
    hall,
    /primaryExtras/,
    "情报大厅折叠态只保留关键词搜索框",
  );

  const manager = stripComments(
    readSource("components/intel/admin/notice-manager.tsx"),
  );
  assert.match(manager, /\n\s+collapsible\n/, "情报管理需要可折叠");
  assert.match(
    manager,
    /primaryExtras=\{/,
    "情报管理折叠态需要保留状态 / 来源类型条件",
  );
  assert.match(manager, /ariaLabel="状态筛选"/);
  assert.match(manager, /ariaLabel="来源类型筛选"/);
});

test("筛选下拉的清空入口与展开图标使用互不重叠的尾部布局", () => {
  const select = stripComments(readSource("components/intel/intel-select.tsx"));

  assert.doesNotMatch(
    select,
    /rightIcon=\{/,
    "清空入口不能再与 Chakra Button 的 rightIcon 占用同一尾部区域",
  );
  assert.match(
    select,
    /<IconButton[\s\S]*aria-label="清空选择"/,
    "清空入口需要是独立的 44px 图标按钮，而不是嵌套在下拉按钮内的 span",
  );
  assert.match(select, /aria-label="清空选择"[\s\S]*as=\{FiChevronDown\}/);
});

test("自动应用提示位于展开态重置按钮左侧", () => {
  const bar = stripComments(readSource("components/intel/filter-bar.tsx"));
  const collapsibleStart = bar.indexOf("{collapsible ? (");
  const collapsibleEnd = bar.indexOf(") : (", collapsibleStart);
  const collapsibleControls = bar.slice(collapsibleStart, collapsibleEnd);

  assert.ok(collapsibleStart >= 0 && collapsibleEnd > collapsibleStart);
  const statusIndex = collapsibleControls.indexOf(
    "{expanded && showStatusChip ? statusChip : null}",
  );
  const resetIndex = collapsibleControls.indexOf(
    "{expanded ? resetButton : null}",
  );
  assert.ok(statusIndex >= 0, "展开态控制行需要显示自动应用提示");
  assert.ok(resetIndex > statusIndex, "自动应用提示必须位于重置按钮左侧");
});

test("两个发布时间输入框都允许点击整个输入区域唤起日期选择器", () => {
  const bar = stripComments(readSource("components/intel/filter-bar.tsx"));
  const pickerTriggers = bar.match(/onClick=\{openNativeDatePicker\}/g) || [];

  assert.equal(pickerTriggers.length, 2);
});

/**
 * 详情页的“返回”必须回到来路，且左导航不能被带跑。
 *
 * 背景（真实反馈）：从情报管理点进详情后，
 *   1) 返回写死跳情报大厅，管理员丢失管理上下文；
 *   2) 左侧导航被定位到“招标情报站-情报大厅”（详情挂在 /intel 下）。
 * 因此情报管理使用独立详情路由 /system/intel/notices/[id]：路径本身就表达了管理上下文，
 * 左导航按路径匹配到 /system/intel，返回也回情报管理子 tab。
 */
test("情报管理详情使用独立路由，返回与左导航都留在管理上下文", () => {
  const card = stripComments(readSource("components/intel/notice-card.tsx"));
  assert.match(card, /detailBasePath/, "卡片需要支持自定义详情路由前缀");
  assert.match(card, /detailFrom/, "卡片需要支持带回来路标识");
  assert.match(card, /`\$\{detailBasePath\}\/\$\{notice\.id\}/);

  const manager = stripComments(
    readSource("components/intel/admin/notice-manager.tsx"),
  );
  assert.match(manager, /detailBasePath="\/system\/intel\/notices"/);

  const hall = stripComments(readSource("app/(main)/intel/page.tsx"));
  assert.match(hall, /detailFrom="hall"/);

  const adminDetail = stripComments(
    readSource("app/(main)/system/intel/notices/[id]/page.tsx"),
  );
  assert.match(adminDetail, /ADMIN_BACK_TARGET/);

  const view = stripComments(
    readSource("components/intel/notice-detail-view.tsx"),
  );
  assert.match(view, /ADMIN_BACK_TARGET[\s\S]*?\/system\/intel\?tab=notices/);
  assert.match(view, /<BackButton href=\{backTarget\.href\}/);

  // 管理端详情必须挂在 /system 下，否则左导航会匹配到 /intel
  const adminHref = manager.match(/detailBasePath="([^"]+)"/);
  assert.ok(adminHref && adminHref[1].startsWith("/system/"));
});

/**
 * 订阅匹配作业面：管理端必须能调整优先级、取消、重新入队、删除。
 *
 * 背景：匹配从打标流程里解耦成独立队列任务后，管理端需要能看见并干预，
 * 尤其是“取消”必须明确告知后果——这批情报不会给用户提醒。
 */
test("情报管理提供订阅匹配子 tab 与四类任务操作", () => {
  const page = stripComments(readSource("app/(main)/system/intel/page.tsx"));
  assert.match(page, /"matches"/, "子 tab 需要注册 matches");
  assert.match(page, /"订阅匹配"/, "子 tab 需要有中文标题");
  assert.match(page, /<MatchManager \/>/);

  const manager = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );
  assert.match(manager, /priority/, "需要支持调整排队中任务的优先级");
  assert.match(manager, /\/cancel`/, "需要支持取消任务");
  assert.match(manager, /\/retry`/, "需要支持按原范围重新入队");
  assert.match(manager, /method: "DELETE"|fetchDelete/, "需要支持删除终态任务");
  assert.match(
    manager,
    /这批情报不会给任何用户生成提醒/,
    "取消的二次确认必须写清后果",
  );
  assert.match(manager, /queue_order/, "排队中的任务要展示执行顺位");
});

test("订阅匹配重试按钮显示中文，并正确拦截业务失败", () => {
  const manager = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );
  assert.match(
    manager,
    /return status === "cancelled" \? "重新入队" : "重试"/,
    "失败/成功任务显示重试，取消任务显示重新入队",
  );
  assert.match(manager, /\{retryActionLabel\(task\.status\)\}/);
  assert.match(manager, /删除/, "删除按钮必须有中文文案");
  assert.match(
    manager,
    /const code = res\?\.data\?\.code;[\s\S]{0,160}code !== 0/,
    "必须检查业务 code，避免把状态不允许重试误报成入队成功",
  );
  assert.match(manager, /title: "重新入队失败"/);
  assert.match(
    manager,
    /if \(status\) \{[\s\S]{0,120}setStatus\(""\)/,
    "成功任务重跑后要回到全部状态，保证新 pending 任务可见",
  );
  assert.match(
    manager,
    /匹配规则更新后可用“重试”补算提醒/,
    "业务说明要讲清成功任务也能重跑",
  );
});

test("成功的订阅匹配任务也允许按原范围重跑", () => {
  const admin = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/match_admin.go"),
  );
  assert.match(admin, /func matchTaskRetryable\(status string\) bool/);
  assert.match(
    admin,
    /case "success", "failed", "cancelled":/,
    "成功任务也要允许在匹配规则更新后重新执行",
  );
});

test("采集任务历史展示本轮匹配出的提醒数", () => {
  const page = stripComments(readSource("app/(main)/system/intel/page.tsx"));
  assert.match(page, /\{ label: "匹配提醒" \}/);
  assert.match(page, /run\.matched_count/);
});

/**
 * 匹配已解耦：打标流程不得再同步匹配，必须投递独立队列任务。
 *
 * 这是本轮最关键的结构性约束——一旦有人把 matchSubscriptions 调回打标循环，
 * 用户多、订阅多时匹配就会重新拖住采集与打标。
 */
test("打标流程不再同步匹配，改为投递独立匹配任务", () => {
  const enrich = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/enrich.go"),
  );
  assert.doesNotMatch(
    enrich,
    /s\.matchSubscriptions\(/,
    "打标循环里不得再同步匹配",
  );
  assert.match(enrich, /enqueueMatchTaskForRun/, "打标终态后要投递匹配任务");

  const worker = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/match.go"),
  );
  assert.match(worker, /TaskTypeTenderIntelMatch = "tender_intel_match"/);
  assert.match(worker, /taskqueue\.Cancelled\(/, "运行中任务要支持协作式取消");
  assert.match(worker, /ListEnabledSubscriptions/, "订阅集合应一次性加载");
});

/**
 * 子 tab 次序与说明开关位置。
 *
 * 背景：招标情报管理有 5 个子 tab，此前“说明”折叠开关在采集源管理/情报管理里
 * 位于左侧、在采集任务历史里位于右侧，管理员每次都要重新找。这里把次序与位置钉死。
 */
test("招标情报管理子 tab 次序固定", () => {
  const page = stripComments(readSource("app/(main)/system/intel/page.tsx"));
  assert.match(
    page,
    /const TAB_KEYS = \["sources", "notices", "matches", "schedule", "runs"\]/,
  );
  const labels = page.match(/const TAB_LABELS = \[([\s\S]*?)\];/);
  assert.ok(labels, "需要 TAB_LABELS");
  const ordered = labels[1].match(/"[^"]+"/g).map((s) => s.replace(/"/g, ""));
  assert.deepEqual(ordered, [
    "采集源管理",
    "情报管理",
    "订阅匹配",
    "采集任务配置",
    "采集任务历史",
  ]);
});

test("四个子 tab 的说明开关位置统一为操作条最右", () => {
  // 位置约定写在组件注释里，注释会被 stripComments 抹掉，因此这里读原文
  const toggle = readSource("components/intel/admin/hint-toggle.tsx");
  assert.match(
    toggle,
    /固定放在顶部操作条的最右侧/,
    "共享组件里必须写明位置约定",
  );

  const page = stripComments(readSource("app/(main)/system/intel/page.tsx"));
  const notice = stripComments(
    readSource("components/intel/admin/notice-manager.tsx"),
  );
  const match = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );

  // 三个老页面不再自己画图标开关，统一改用共享组件
  for (const [name, source] of [
    ["采集源管理/采集任务历史", page],
    ["情报管理", notice],
  ]) {
    assert.doesNotMatch(source, /FiChevronDown/, `${name} 不应再自带折叠图标`);
  }

  // 说明开关必须排在动作按钮之后（即操作条最右）
  const after = (source, anchor, label) => {
    const anchorAt = source.indexOf(anchor);
    const toggleAt = source.indexOf(label);
    assert.ok(anchorAt >= 0 && toggleAt >= 0, `${label} 与 ${anchor} 都应存在`);
    assert.ok(toggleAt > anchorAt, `${label} 必须排在 ${anchor} 之后`);
  };
  after(
    page,
    "导入采集源",
    '<HintToggle\n                      label="采集说明"',
  );
  after(
    page,
    "刷新",
    '<HintToggle\n                      label="采集任务历史说明"',
  );
  after(notice, "发布情报", "<HintToggle");
  after(match, "手动补扫", "<HintToggle");
});

test("订阅匹配附带默认折叠的业务说明", () => {
  const match = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );
  assert.match(match, /const \[helpOpen, setHelpOpen\] = useState\(false\)/);
  assert.match(match, /<Collapse in=\{helpOpen\}/);
  assert.match(match, /把入库情报与用户订阅规则比对，命中就生成站内提醒/);
  assert.match(
    match,
    /取消任务表示这批情报这次不生成任何提醒/,
    "说明里要讲清取消的后果",
  );
});

/**
 * 订阅匹配表格的对齐与信息完整度。
 *
 * 背景（真实反馈）：表格“列名跟内容没有对齐”。根因是数值单元格用了 isNumeric
 * （右对齐）而表头没有，于是列名左对齐、数字右对齐，整张表看起来是错位的。
 */
test("匹配任务表格的表头与内容同向对齐", () => {
  const match = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );
  assert.match(
    match,
    /<Th\s+isNumeric/,
    "数值列的表头必须同样 isNumeric，否则与右对齐的数值内容错位",
  );
  assert.match(match, /verticalAlign="top"/, "多行单元格应统一顶对齐");
});

test("订阅回溯任务展示对应用户与订阅", () => {
  const match = stripComments(
    readSource("components/intel/admin/match-manager.tsx"),
  );
  assert.match(match, /task\.subscription_name/);
  assert.match(match, /task\.user_name/);
  assert.match(match, /task\.user_mobile/);

  const service = stripComments(readSource("service/intel.ts"));
  assert.match(service, /subscription_name: string/);
  assert.match(service, /user_name: string/);
  assert.match(service, /user_mobile: string/);
});

/**
 * 情报卡片的行业标签必须全量且正确地渲染出来。
 *
 * 背景（真实反馈）：管理员在情报管理里给情报加了自定义行业标签（如“软件开发”），
 * 卡片上却显示成一排“其他”。根因是列表视图用 IndustryName 把未知编码统一成了
 * “其他”；自定义行业存的是原始文本，必须走 IndustryLabel 原样展示。
 */
test("情报列表视图用 IndustryLabel 渲染行业标签", () => {
  const handlers = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/handlers.go"),
  );
  assert.match(
    handlers,
    /names = append\(names, IndustryLabel\(code\)\)/,
    "自定义行业标签必须原样渲染，不能被 IndustryName 变成“其他”",
  );
});

test("情报卡片把公告类型与行业标签分组渲染", () => {
  const card = stripComments(readSource("components/intel/notice-card.tsx"));
  assert.match(card, /const industryNames = notice\.industry_names \|\| \[\]/);
  assert.match(card, /industryNames\.map/, "行业标签要全量渲染，不做截断");
  assert.match(
    card,
    /const INDUSTRY_TAG_SCHEMES = \["primary", "info", "success", "purple"\] as const/,
    "行业标签需要有稳定的主题色池",
  );
  assert.match(
    card,
    /colorScheme=\{industryTagScheme\(name\)\}/,
    "行业标签需要按名称稳定映射主题色",
  );
  assert.match(
    card,
    /<Tag[\s\S]{0,240}variant="subtle"/,
    "行业标签需要柔和底色，不再使用黑框白底",
  );
  assert.match(
    card,
    /notice\.notice_type_name \|\| "类型未识别"/,
    "未识别类型要有明确文案，而不是留一个“其他”占位",
  );
  assert.match(
    card,
    /typeUnclear/,
    "未识别的公告类型要弱化，不能与行业标签混在一起抢视觉",
  );
  assert.match(card, /formatBudget\(notice\)/, "预算作为首要关键事实");
  assert.match(card, /formatRegion\(notice\)/);
});

/**
 * 匹配语义：关键词与行业互为替代路径。
 *
 * 背景（真实反馈）：原来全条件 AND，关键词是字面包含匹配、行业是机器打的粗粒度标签，
 * 用户配了三四个条件的订阅一条情报都匹配不到（实测 55 条在架情报 → 0 命中）。
 * 现在改为“（关键词命中 或 行业命中）且 地区/类型/预算命中”，并用静态守卫钉死，
 * 防止以后有人顺手改回全 AND。
 */
test("订阅匹配：关键词与行业互为替代路径", () => {
  const enrich = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/enrich.go"),
  );
  assert.match(
    enrich,
    /case len\(keywords\) > 0 && len\(subIndustries\) > 0:[\s\S]{0,120}if !keywordHit && !industryHit/,
    "同时配置关键词与行业时，任一命中即应通过召回组",
  );
  assert.match(
    enrich,
    /isRecognizedNoticeType/,
    "公告类型未识别时不应因类型条件淘汰",
  );

  const worker = stripComments(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/match.go"),
  );
  assert.match(
    readSource("../bid-engine-backend/pkg/handler/tenderintel/match.go"),
    /行业不参与预筛/,
    "结构化预筛不能再用行业排除，否则会误杀靠关键词命中的订阅",
  );
  assert.doesNotMatch(worker, /matchIndustries/, "预筛里不应再出现行业判定");
});
