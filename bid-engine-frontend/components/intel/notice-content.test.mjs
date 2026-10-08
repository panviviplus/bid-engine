import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";
import test from "node:test";

import {
  noticeHTMLRehypePlugins,
  noticeSanitizeSchema,
} from "./notice-content-schema.mjs";
import { formatInsightMeta } from "./notice-detail-meta.mjs";

test("AI 解读元信息只显示时间和风险提示", () => {
  assert.equal(
    formatInsightMeta("2026-09-22 10:00"),
    "2026-09-22 10:00 · AI 结论仅供参考",
  );
  assert.equal(formatInsightMeta(""), "AI 结论仅供参考");
});

test("公告 HTML schema 保留文档结构和合并单元格", () => {
  for (const tag of [
    "h2",
    "ol",
    "ul",
    "table",
    "thead",
    "tbody",
    "tr",
    "th",
    "td",
    "a",
  ]) {
    assert.ok(noticeSanitizeSchema.tagNames.includes(tag));
  }
  assert.ok(noticeSanitizeSchema.attributes.td.includes("colSpan"));
  assert.ok(noticeSanitizeSchema.attributes.td.includes("rowSpan"));
  assert.ok(noticeSanitizeSchema.attributes.ol.includes("start"));
});

test("公告 HTML 渲染移除脚本和事件属性并保留合并单元格", () => {
  const html = renderToStaticMarkup(
    React.createElement(ReactMarkdown, {
      rehypePlugins: noticeHTMLRehypePlugins,
      children:
        '<table><tbody><tr><td colspan="2" onclick="alert(1)">正文</td></tr></tbody></table><script>alert(2)</script>',
    }),
  );
  assert.match(html, /colspan="2"/i);
  assert.doesNotMatch(html, /onclick|script|alert/);
});
