import test from "node:test";
import assert from "node:assert/strict";

import {
  advanceTimelineProgress,
  getDownstreamIndexes,
  getNextIndex,
  getTimelineStageIndex,
  getVisibleBubbleIndexes,
  normalizeTimelinePositions,
  shouldDisableResponsiveMotion,
  shouldPauseRotation,
} from "./rotation.mjs";

test("cycles through stage indexes without skipping", () => {
  assert.equal(getNextIndex(0, 6), 1);
  assert.equal(getNextIndex(4, 6), 5);
  assert.equal(getNextIndex(5, 6), 0);
});

test("keeps a stage bubble visible for eight seconds", () => {
  const activatedAt = [1_000, 5_000, 9_000, null, null, null];
  assert.deepEqual(getVisibleBubbleIndexes(activatedAt, 9_000, 8_000), [1, 2]);
  assert.deepEqual(getVisibleBubbleIndexes(activatedAt, 12_999, 8_000), [1, 2]);
  assert.deepEqual(getVisibleBubbleIndexes(activatedAt, 13_000, 8_000), [2]);
});

test("highlights the selected entry stage and every downstream stage", () => {
  assert.deepEqual(getDownstreamIndexes(0, 6), [0, 1, 2, 3, 4, 5]);
  assert.deepEqual(getDownstreamIndexes(3, 6), [3, 4, 5]);
  assert.deepEqual(getDownstreamIndexes(5, 6), [5]);
});

test("pauses automatic rotation for motion, focus, hover, and hidden pages", () => {
  assert.equal(shouldPauseRotation({}), false);
  assert.equal(shouldPauseRotation({ disabled: true }), true);
  assert.equal(shouldPauseRotation({ reducedMotion: true }), true);
  assert.equal(shouldPauseRotation({ hovering: true }), true);
  assert.equal(shouldPauseRotation({ focusWithin: true }), true);
  assert.equal(shouldPauseRotation({ documentHidden: true }), true);
});

test("enables automatic motion once a desktop media query matches", () => {
  assert.equal(
    shouldDisableResponsiveMotion({
      requireDesktop: true,
      desktopMatches: false,
    }),
    true,
  );
  assert.equal(
    shouldDisableResponsiveMotion({
      requireDesktop: true,
      desktopMatches: true,
    }),
    false,
  );
  assert.equal(
    shouldDisableResponsiveMotion({
      requireDesktop: false,
      desktopMatches: false,
    }),
    false,
  );
});

test("normalizes uneven node positions against the full timeline path", () => {
  assert.deepEqual(
    normalizeTimelinePositions([10, 30, 70, 90], 10, 110),
    [0, 0.2, 0.6, 0.8],
  );
});

test("activates a node only when continuous progress reaches its position", () => {
  const positions = [0, 0.2, 0.6, 0.8];
  assert.equal(getTimelineStageIndex(0.199, positions), 0);
  assert.equal(getTimelineStageIndex(0.2, positions), 1);
  assert.equal(getTimelineStageIndex(0.599, positions), 1);
  assert.equal(getTimelineStageIndex(0.6, positions), 2);
  assert.equal(getTimelineStageIndex(0.95, positions), 3);
});

test("advances the timeline uniformly through a twenty-second lifecycle", () => {
  assert.equal(advanceTimelineProgress(0, 5_000, 20_000), 0.25);
  assert.equal(advanceTimelineProgress(0.9, 3_000, 20_000), 0.05);
});
