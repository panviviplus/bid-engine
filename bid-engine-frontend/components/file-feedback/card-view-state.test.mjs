import assert from "node:assert/strict";
import test from "node:test";

let cardViewState = null;

try {
  cardViewState = await import("./card-view-state.mjs");
} catch {
  cardViewState = null;
}

test("feedback card view exposes its state resolver", () => {
  assert.equal(typeof cardViewState?.resolveFeedbackCardViewState, "function");
});

test("feedback card view preserves visible cards while background data refreshes", () => {
  assert.ok(
    cardViewState,
    "feedback card view state implementation is available",
  );
  assert.equal(
    cardViewState.resolveFeedbackCardViewState({
      loading: true,
      error: null,
      itemCount: 3,
    }),
    "ready",
  );
});

test("feedback card view distinguishes loading, error, empty, and ready states", () => {
  assert.ok(
    cardViewState,
    "feedback card view state implementation is available",
  );
  const cases = [
    [{ loading: true, error: null, itemCount: 0 }, "loading"],
    [{ loading: false, error: "加载失败", itemCount: 0 }, "error"],
    [{ loading: false, error: null, itemCount: 0 }, "empty"],
    [{ loading: false, error: null, itemCount: 2 }, "ready"],
  ];

  for (const [input, expected] of cases) {
    assert.equal(cardViewState.resolveFeedbackCardViewState(input), expected);
  }
});
