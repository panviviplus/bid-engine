import assert from "node:assert/strict";
import test from "node:test";

import {
  getHighlightNavigationTarget,
  mapHighlightAreas,
} from "./pdf-navigation.mjs";

const multiSourceAreas = [
  {
    top: 0.2,
    left: 0.1,
    width: 0.3,
    height: 0.04,
    page_index: 0,
    active: false,
  },
  {
    top: 0.65,
    left: 0.18,
    width: 0.42,
    height: 0.05,
    page_index: 2,
    active: true,
  },
];

test("waits for the PDF document before selecting a navigation target", () => {
  assert.equal(getHighlightNavigationTarget(multiSourceAreas, false), null);
});

test("navigates to the active source instead of the first source", () => {
  assert.deepEqual(getHighlightNavigationTarget(multiSourceAreas, true), {
    top: 65,
    left: 18,
    width: 42,
    height: 5,
    pageIndex: 2,
    active: true,
  });
});

test("keeps the first source as the legacy fallback when no source is explicitly active", () => {
  const areas = [
    {
      top: 0.1,
      left: 0.2,
      width: 0.3,
      height: 0.04,
      page_index: 4,
    },
    {
      top: 0.5,
      left: 0.6,
      width: 0.2,
      height: 0.03,
      page_index: 8,
    },
  ];

  assert.deepEqual(getHighlightNavigationTarget(areas, true), {
    top: 10,
    left: 20,
    width: 30,
    height: 4,
    pageIndex: 4,
    active: true,
  });
});

test("maps every source so inactive evidence remains renderable", () => {
  assert.deepEqual(mapHighlightAreas(multiSourceAreas), [
    {
      top: 20,
      left: 10,
      width: 30,
      height: 4,
      pageIndex: 0,
      active: false,
    },
    {
      top: 65,
      left: 18,
      width: 42,
      height: 5,
      pageIndex: 2,
      active: true,
    },
  ]);
});
