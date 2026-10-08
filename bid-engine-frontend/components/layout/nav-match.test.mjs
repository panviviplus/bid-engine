/**
 * 左导航选中态守卫。
 *
 * 背景（真实反馈）：系统管理 → 招标情报管理 → 情报管理 里点进公告详情后，
 * 左导航被定位到“招标情报站 → 情报大厅”——因为当时的详情页挂在 `/intel/[id]`。
 * 详情页必须挂在自身模块路径下，匹配规则也要保持“子路径归属最近父级菜单”。
 */
import assert from "node:assert/strict";
import test from "node:test";

import { findBestMatchHref, nodeIsActive } from "./nav-match.mjs";

const NAV = [
  {
    label: "招标情报站",
    children: [
      { label: "情报大厅", href: "/intel" },
      { label: "订阅与提醒", href: "/intel/subscriptions" },
    ],
  },
  { label: "招标解析", href: "/bid-analysis" },
  {
    label: "系统管理",
    children: [
      { label: "模型配置", href: "/system/llm-config" },
      { label: "系统模型配置", href: "/system/llm-config/system" },
      { label: "招标情报管理", href: "/system/intel" },
    ],
  },
];

test("情报大厅详情归属情报大厅", () => {
  assert.equal(findBestMatchHref(NAV, "/intel/42"), "/intel");
  assert.equal(findBestMatchHref(NAV, "/intel"), "/intel");
});

test("情报管理详情归属招标情报管理，不会跳到情报大厅", () => {
  assert.equal(
    findBestMatchHref(NAV, "/system/intel/notices/42"),
    "/system/intel",
  );
  // 回归断言：/system/intel/notices/42 不应命中 /intel
  assert.notEqual(findBestMatchHref(NAV, "/system/intel/notices/42"), "/intel");
});

test("更具体的子菜单优先于父级前缀", () => {
  assert.equal(
    findBestMatchHref(NAV, "/intel/subscriptions"),
    "/intel/subscriptions",
  );
  // /intel/alerts 已并入订阅与提醒（仅保留服务端重定向），因此归到 /intel
  assert.equal(findBestMatchHref(NAV, "/intel/alerts"), "/intel");
  assert.equal(
    findBestMatchHref(NAV, "/system/llm-config/system"),
    "/system/llm-config/system",
  );
});

test("分组按激活的子孙菜单判定激活", () => {
  const best = findBestMatchHref(NAV, "/system/intel/notices/42");
  assert.equal(nodeIsActive(NAV[2], "/system/intel/notices/42", best), true);
  assert.equal(nodeIsActive(NAV[0], "/system/intel/notices/42", best), false);
});
