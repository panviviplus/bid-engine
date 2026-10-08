import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const listPageUrl = new URL("./type-list-page.tsx", import.meta.url);

/** 去掉注释，避免说明文案里的关键字污染断言 */
function stripComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
}

test("material list keeps bounded card columns instead of stretching a lone card across the row", async () => {
  const source = stripComments(await readFile(listPageUrl, "utf8"));

  // auto-fit + 1fr 在条目少时会折叠空轨道，把剩余空间分给现有卡片（一张卡铺满整行）
  assert.doesNotMatch(source, /auto-fit/);
  assert.doesNotMatch(source, /minChildWidth/);

  assert.match(
    source,
    /repeat\(auto-fill, minmax\(min\(100%, 280px\), 340px\)\)/,
  );
  assert.match(source, /alignItems="start"/);
});
