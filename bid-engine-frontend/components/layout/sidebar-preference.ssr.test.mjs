import test from "node:test";
import assert from "node:assert/strict";

import {
  readSidebarPreference,
  writeSidebarPreference,
} from "./sidebar-preference.mjs";

test("is a no-op when there is no window (SSR)", () => {
  assert.equal(readSidebarPreference("user-1"), null);
  assert.doesNotThrow(() => writeSidebarPreference("user-1", true));
});
