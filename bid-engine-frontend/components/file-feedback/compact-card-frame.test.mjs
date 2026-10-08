import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChakraProvider, extendTheme } from "@chakra-ui/react";

let compactCardModule = null;

try {
  compactCardModule = await import("./compact-card-frame.mjs");
} catch {
  compactCardModule = null;
}

const theme = extendTheme({
  colors: {
    workbench: { paper: "#FFFFFF", line: "#DCE4EA" },
    success: { 200: "#A7F3D0", 300: "#6EE7B7" },
    warning: { 200: "#FDE68A", 300: "#FCD34D" },
  },
});

function renderCard(processed) {
  return renderToStaticMarkup(
    React.createElement(
      ChakraProvider,
      { theme },
      React.createElement(
        compactCardModule.CompactFeedbackCardFrame,
        { processed },
        React.createElement("span", null, "反馈内容"),
      ),
    ),
  );
}

/** 取 <article> 自身命中的 CSS 规则，避免把 Chakra 全局样式里的 min-height 也算进来 */
function articleCss(html) {
  const styleBlocks = [...html.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)]
    .map((match) => match[1])
    .join("\n");
  const articleTag = html.match(/<article[^>]*>/)?.[0] || "";
  const classNames = (articleTag.match(/class="([^"]+)"/)?.[1] || "")
    .split(/\s+/)
    .filter(Boolean);

  return styleBlocks
    .split("}")
    .filter((rule) => classNames.some((name) => rule.includes(`.${name}`)))
    .join("}");
}

test("compact feedback card frame is available", () => {
  assert.equal(typeof compactCardModule?.CompactFeedbackCardFrame, "function");
});

test("compact feedback card is content-sized and keeps 16px content padding", () => {
  assert.ok(compactCardModule, "compact card implementation is available");
  const html = renderCard(false);

  assert.match(html, /<article/);
  assert.match(html, /padding:var\(--chakra-space-4\)/);
  assert.match(html, />反馈内容<\/span>/);

  // 卡片不能有硬编码高度地板，否则内容短时底部会留大片空白
  const css = articleCss(html);
  assert.notEqual(css, "", "article 自身应命中样式，否则该断言失去意义");
  assert.doesNotMatch(css, /min-height/);
});

test("compact feedback card keeps processed and pending border states distinct", () => {
  assert.ok(compactCardModule, "compact card implementation is available");
  assert.match(renderCard(false), /warning-200/);
  assert.match(renderCard(true), /success-200/);
});
