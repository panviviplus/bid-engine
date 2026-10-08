import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChakraProvider, extendTheme } from "@chakra-ui/react";

let workbenchModule = null;

try {
  workbenchModule = await import("./module-workbench.mjs");
} catch {
  workbenchModule = null;
}

const theme = extendTheme({
  colors: {
    workbench: {
      canvas: "#F2F5F7",
      paper: "#FFFFFF",
      control: "#0B1B2B",
      controlRaised: "#132B40",
      line: "#DCE4EA",
      muted: "#66798B",
    },
    gold: { 400: "#E7B757" },
  },
});

function renderWithTheme(element) {
  return renderToStaticMarkup(
    React.createElement(ChakraProvider, { theme }, element),
  );
}

test("module workbench exposes the shared V3 surfaces", () => {
  assert.equal(typeof workbenchModule?.ModuleWorkbenchHeader, "function");
  assert.equal(typeof workbenchModule?.ModuleWorkbenchDeck, "function");
});

test("module workbench renders one page heading and a live activity summary", () => {
  assert.ok(workbenchModule, "module workbench implementation is available");
  const html = renderWithTheme(
    React.createElement(workbenchModule.ModuleWorkbenchHeader, {
      title: "标书生成",
      activity: "当前有 2 个项目生成中",
    }),
  );

  assert.match(html, /<h1[^>]*>标书生成<\/h1>/);
  assert.match(html, />工作台</);
  assert.match(html, /aria-live="polite"/);
  assert.match(html, />当前有 2 个项目生成中</);
});

test("node activity is not wrapped in a paragraph", () => {
  assert.ok(workbenchModule, "module workbench implementation is available");
  const html = renderWithTheme(
    React.createElement(workbenchModule.ModuleWorkbenchHeader, {
      title: "我的订阅",
      titleSuffix: null,
      activity: React.createElement("button", { type: "button" }, "新建订阅"),
    }),
  );

  assert.match(html, /<h1[^>]*>我的订阅<\/h1>/);
  assert.doesNotMatch(html, />工作台</);
  assert.match(html, /<button[^>]*>新建订阅<\/button>/);
  // 关键回归：节点活动区不能落在 <p> 里，否则会触发 hydration 报错
  assert.doesNotMatch(html, /<p[^>]*>(?:(?!<\/p>).)*<button/s);
});

test("module workbench deck keeps its purpose and primary action discoverable", () => {
  assert.ok(workbenchModule, "module workbench implementation is available");
  const html = renderWithTheme(
    React.createElement(
      workbenchModule.ModuleWorkbenchDeck,
      {
        title: "从大纲开始，组织整份投标书",
        description: "选择创建方式后进入编辑工作区。",
        action: React.createElement("button", { type: "button" }, "创建标书"),
        expanded: true,
        onToggle: () => {},
      },
      React.createElement("span", null, "三种创建方式"),
    ),
  );

  assert.match(html, /role="region"/);
  assert.match(html, /aria-label="从大纲开始，组织整份投标书"/);
  assert.match(html, />创建标书<\/button>/);
  assert.match(html, />三种创建方式<\/span>/);
});

test("module workbench deck exposes the same compact single-line disclosure contract as bid analysis", () => {
  assert.ok(workbenchModule, "module workbench implementation is available");
  const collapsed = renderWithTheme(
    React.createElement(
      workbenchModule.ModuleWorkbenchDeck,
      {
        title: "在交付前定位风险",
        description: "对照原文检查。",
        expanded: false,
        onToggle: () => {},
      },
      React.createElement("span", null, "符合性"),
    ),
  );
  const expanded = renderWithTheme(
    React.createElement(workbenchModule.ModuleWorkbenchDeck, {
      title: "在交付前定位风险",
      description: "对照原文检查。",
      expanded: true,
      onToggle: () => {},
    }),
  );

  assert.match(collapsed, /aria-expanded="false"/);
  assert.match(collapsed, /aria-controls="module-workbench-deck-details"/);
  assert.doesNotMatch(collapsed, /展开说明|收起说明/);
  assert.match(expanded, /aria-expanded="true"/);
  assert.doesNotMatch(expanded, /展开说明|收起说明/);
});
