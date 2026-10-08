# BidEngine 全页面响应式改造 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for
> tracking.

**Goal:**
让标擎全部前端路由在 2560×1440、常见 14 英寸屏幕及 320–768px 窄屏下充分利用可用空间，同时不产生页面级横向溢出。

**Architecture:** 以现有 `design.md` 的 Split Studio /
Workbench 系统为唯一视觉基线，新增一个无页面级最大宽度的共享响应式画布。页面只保留内容级阅读宽度；列表、卡片、表格和详情分栏使用完整主工作区，并通过 Chakra 响应式属性在内容开始拥挤时重排。

**Tech Stack:** Next.js 14 App Router、React 18、Chakra UI
2.8、TypeScript/JavaScript、Node test runner

**Spec:** `design.md`

## Global Constraints

- 保留全部生产路由、业务逻辑、文案意图、深海蓝/信号金品牌与现有侧栏信息架构。
- 页面级内容宽度不得再受
  `container.md`、`container.xl`、1440/1480/1540/1720px 等硬上限约束。
- 可读性上限只允许落在正文段落、表单字段或单个信息块上，不能限制整个页面画布。
- `html` 与 `body` 使用 `overflow-x: clip`；根页面不能产生横向滚动。
- 320、375、414、768、常见 14 英寸视口和 2560×1440 都必须安全。
- 不修改或覆盖用户已有的 `components/analysis/bid-analysis-v3/detail/panels.tsx`
  工作区改动。
- 遵循仓库规则，不执行 `git commit` 或 `git push`。

---

### Task 1: 共享流体画布

**Files:**

- Create: `components/layout/responsive-layout.mjs`
- Create: `components/layout/responsive-layout.test.mjs`
- Create: `components/layout/responsive-page.tsx`
- Modify: `components/layout/index.js`
- Modify: `components/analysis/bid-analysis-v3/workspace.tsx`
- Modify: `styles/globals.css`

**Interfaces:**

- Produces:
  `getResponsivePageLayout(options)`、`PAGE_GUTTERS`、`AUTH_PAGE_GUTTERS`、`PageViewport`、`PageContent`

- [ ] **Step 1: Write the failing tests**
      — 覆盖 320/375/414/768/1440/2560 宽度、流体边距、全宽内容轨道和滚动策略。
- [ ] **Step 2: Run RED** —
      `node --test components/layout/responsive-layout.test.mjs`
      应因模块不存在而失败。
- [ ] **Step 3: Implement the layout contract**
      — 返回 Chakra 可直接消费的 viewport/content 属性，并由两个 React 包装组件使用。
- [ ] **Step 4: Run GREEN** — 同一命令应全部通过。

### Task 2: SignIn 与注册入口

**Files:**

- Modify: `components/signin/experience/HeroSection.tsx`
- Modify: `components/signin/experience/CapabilitySection.tsx`
- Modify: `components/signin/experience/EntrySection.tsx`
- Modify: `components/signin/experience/SigninAuthCard.tsx`
- Modify: `components/auth/AuthCanvas.tsx`

- [ ] **Step 1: Add a failing wide-canvas assertion**
      — 认证区不再返回 1440/1480px 页面上限。
- [ ] **Step 2: Run RED** — 确认现有认证容器触发失败。
- [ ] **Step 3: Apply fluid gutters and scalable split tracks**
      — 大屏扩展左右工作区，窄屏仍单列并保证登录卡先于展示内容。
- [ ] **Step 4: Run GREEN** — 响应式契约测试通过。

### Task 3: 登录后列表、首页和配置页

**Files:**

- Modify: `app/(main)/page.tsx`
- Modify: `app/(main)/bid-analysis/page.tsx`
- Modify: `app/(main)/bid-audit/page.tsx`
- Modify: `app/(main)/file-gen/page.tsx`
- Modify: `app/(main)/file-feedback/page.tsx`
- Modify: `app/(main)/system/llm-config/page.tsx`
- Modify: `components/material/type-list-page.tsx`
- Modify: `app/(main)/material/knowledge/page.tsx`
- Modify: `app/(main)/material/gallery/page.tsx`
- Modify: `app/(main)/material/products/page.tsx`

- [ ] **Step 1: Add failing route-family assertions**
      — 页面不得出现限制整页的 container 上限或 `100vh` 嵌套高度。
- [ ] **Step 2: Run RED** — 确认首页、审核列表、模型配置等现有硬限制被捕获。
- [ ] **Step 3: Migrate to `PageViewport` / `PageContent`**
      — 统一全宽画布和流体边距，工具栏在窄屏换行，表格与卡片视图使用可用宽度。
- [ ] **Step 4: Run GREEN** — 页面族契约测试通过。

### Task 4: 详情与编辑工作区

**Files:**

- Modify: `app/(main)/bid-analysis/[id]/page.tsx`
- Modify: `app/(main)/file-gen/[id]/page.tsx`
- Modify: `components/file-gen/outline-review-gate.tsx`
- Modify: `components/audit/detail.tsx`
- Modify: `app/(main)/material/knowledge/[id]/page.tsx`

- [ ] **Step 1: Add failing workspace assertions**
      — 详情页不使用 1720px 页面上限，窄屏不得依赖大于视口的固定主栏宽度。
- [ ] **Step 2: Run RED** — 现有 1720px 与 560px 固定宽度应触发失败。
- [ ] **Step 3: Make workspaces adaptive**
      — 大屏使用完整多栏画布；较窄屏折叠侧栏、使用抽屉/纵向流或仅在内部区域滚动。
- [ ] **Step 4: Run GREEN** — 工作区契约测试通过。

### Task 5: 验证与审阅

**Files:**

- Modify: `.hallmark/log.json`

- [ ] **Step 1: Run focused tests** —
      `node --test components/layout/responsive-layout.test.mjs`
      以及现有前端 Node 测试。
- [ ] **Step 2: Run lint/type/build verification** — 至少执行
      `npm run build`；若 lint 存在历史问题，分别记录本次相关与无关错误。
- [ ] **Step 3: Run responsive static audit** — 搜索剩余页面级
      `maxW`、`100vh`、固定主栏 `minW` 与根 `overflowX`。
- [ ] **Step 4: Self-review**
      — 核对所有 22 个路由入口、别名/重定向页、共享壳层、认证页和详情工作区。
- [ ] **Step 5: Hallmark handoff audit** — 运行 58 项 slop test 并写入一次
      `scope: app` 记录。
