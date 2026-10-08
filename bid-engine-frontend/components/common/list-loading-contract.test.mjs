import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const feedbackPageUrl = new URL(
  "../../app/(main)/file-feedback/page.tsx",
  import.meta.url,
);
const analysisPageUrl = new URL(
  "../../app/(main)/bid-analysis/page.tsx",
  import.meta.url,
);

test("feedback list uses progressive loading without a pagination footer", async () => {
  const source = await readFile(feedbackPageUrl, "utf8");

  assert.match(source, /InfiniteScrollList/);
  assert.match(source, /useInfiniteList/);
  assert.doesNotMatch(source, /@ajna\/pagination/);
  assert.doesNotMatch(source, /<TableFooter\b/);
});

test("analysis list exposes the shared loaded-all ending", async () => {
  const source = await readFile(analysisPageUrl, "utf8");

  assert.match(source, /InfiniteScrollList/);
  assert.match(source, /useInfiniteList/);
});
