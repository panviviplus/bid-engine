import test from "node:test";
import assert from "node:assert/strict";

import {
  HOME_MODULES,
  HOME_MODULE_LABELS,
  MATERIAL_LINKS,
} from "./home-modules.mjs";

test("presents the six Home modules in the approved order", () => {
  assert.deepEqual(
    HOME_MODULES.map(({ label }) => label),
    ["招标情报站", "招标解析", "投标审核", "标书生成", "标书审核", "素材库"],
  );
});

test("keeps planned modules non-interactive and outside Home statistics", () => {
  const plannedModules = HOME_MODULES.filter(({ planned }) => planned);

  assert.deepEqual(
    plannedModules.map(({ label }) => label),
    ["招标情报站", "投标审核"],
  );
  plannedModules.forEach((module) => {
    assert.equal(module.href, undefined);
    assert.equal(module.statKey, undefined);
  });
});

test("preserves active module routes and material shortcuts", () => {
  assert.deepEqual(
    HOME_MODULES.filter(({ href }) => href).map(({ label, href, statKey }) => ({
      label,
      href,
      statKey,
    })),
    [
      { label: "招标解析", href: "/bid-analysis", statKey: "bid_analysis" },
      { label: "标书生成", href: "/file-gen", statKey: "bid_generation" },
      { label: "标书审核", href: "/bid-audit", statKey: "bid_review" },
    ],
  );
  assert.deepEqual(MATERIAL_LINKS, [
    { id: "qualification", label: "资质", href: "/material/qualification" },
    { id: "performance", label: "业绩", href: "/material/performance" },
    { id: "template", label: "模板", href: "/material/template" },
  ]);
});

test("uses short Home labels without codes, stages, or shared-asset language", () => {
  assert.deepEqual(HOME_MODULE_LABELS, {
    bid_analysis: "招标解析",
    bid_generation: "标书生成",
    bid_review: "标书审核",
    material: "素材库",
  });

  const userFacingContent = JSON.stringify(HOME_MODULES);
  assert.doesNotMatch(userFacingContent, /\b(?:AN|GN|RV|MT)\b/);
  assert.doesNotMatch(userFacingContent, /阶段\s*\d|共享资产/);
});
