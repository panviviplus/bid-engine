import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChakraProvider } from "@chakra-ui/react";

let gridModule = null;

try {
  gridModule = await import("./feedback-card-grid.mjs");
} catch {
  gridModule = null;
}

test("feedback card grid is available", () => {
  assert.equal(typeof gridModule?.FeedbackCardGrid, "function");
});

test("one feedback card keeps a bounded column instead of stretching across the row", () => {
  assert.ok(gridModule, "feedback card grid implementation is available");
  const html = renderToStaticMarkup(
    React.createElement(
      ChakraProvider,
      null,
      React.createElement(
        gridModule.FeedbackCardGrid,
        null,
        React.createElement("article", null, "唯一反馈项"),
      ),
    ),
  );

  assert.match(html, /repeat\(auto-fill,/);
  assert.match(html, /340px/);
  assert.doesNotMatch(html, /repeat\(auto-fit,/);
  assert.match(html, />唯一反馈项<\/article>/);
});
