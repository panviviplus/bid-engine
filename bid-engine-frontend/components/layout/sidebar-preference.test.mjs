import test from "node:test";
import assert from "node:assert/strict";

import {
  readSidebarPreference,
  writeSidebarPreference,
} from "./sidebar-preference.mjs";

const store = new Map();
globalThis.window = {
  localStorage: {
    getItem: (key) => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => store.set(key, String(value)),
  },
};

test("returns null when no record exists", () => {
  store.clear();
  assert.equal(readSidebarPreference("user-1"), null);
});

test("round-trips the stored collapsed state", () => {
  store.clear();
  writeSidebarPreference("user-1", true);
  assert.equal(readSidebarPreference("user-1"), true);
  writeSidebarPreference("user-1", false);
  assert.equal(readSidebarPreference("user-1"), false);
});

test("keeps preferences isolated per user", () => {
  store.clear();
  writeSidebarPreference("user-a", false);
  assert.equal(readSidebarPreference("user-a"), false);
  assert.equal(readSidebarPreference("user-b"), null);
});

test("falls back to the generic key when userId is absent", () => {
  store.clear();
  writeSidebarPreference("", false);
  assert.equal(readSidebarPreference(""), false);
  assert.equal(readSidebarPreference("user-a"), null);
  writeSidebarPreference("", true);
  assert.equal(readSidebarPreference(""), true);
});

test("treats an invalid stored value as no record", () => {
  store.clear();
  store.set("bid-engine.sidebar.collapsed.user-x", "banana");
  assert.equal(readSidebarPreference("user-x"), null);
});
