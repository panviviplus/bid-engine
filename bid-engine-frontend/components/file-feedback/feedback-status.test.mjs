import assert from "node:assert/strict";
import test from "node:test";

let statusModule = null;

try {
  statusModule = await import("./feedback-status.mjs");
} catch {
  statusModule = null;
}

test("feedback status resolver is available", () => {
  assert.equal(typeof statusModule?.resolveFeedbackStatusMeta, "function");
});

test("new and unknown feedback states remain pending", () => {
  assert.ok(statusModule, "feedback status implementation is available");
  for (const status of [0, "0", null, undefined, "submitted", "已提交"]) {
    assert.deepEqual(statusModule.resolveFeedbackStatusMeta(status), {
      text: "待处理",
      scheme: "orange",
      processed: false,
    });
  }
});

test("only processed status renders as processed", () => {
  assert.ok(statusModule, "feedback status implementation is available");
  for (const status of [1, "1"]) {
    assert.deepEqual(statusModule.resolveFeedbackStatusMeta(status), {
      text: "已处理",
      scheme: "green",
      processed: true,
    });
  }
});
