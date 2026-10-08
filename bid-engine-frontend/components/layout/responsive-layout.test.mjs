import assert from "node:assert/strict";
import test from "node:test";

import {
  AUTH_PAGE_GUTTERS,
  PAGE_GUTTERS,
  getResponsivePageLayout,
  resolveResponsiveControlLayout,
} from "./responsive-layout.mjs";

test("wide application pages keep almost all available canvas instead of recentering into a fixed column", () => {
  const layout = getResponsivePageLayout({
    viewportWidth: 2560,
    sidebarWidth: 240,
  });

  assert.equal(layout.content.maxWidth, "none");
  assert.equal(layout.content.marginInline, 0);
  assert.ok(layout.availableContentWidth >= 2200);
});

test("page gutters grow with the viewport while leaving a safe 320px canvas", () => {
  assert.deepEqual(PAGE_GUTTERS, {
    base: 3,
    sm: 4,
    md: 6,
    xl: 8,
    "2xl": 10,
  });
  assert.deepEqual(AUTH_PAGE_GUTTERS, {
    base: 4,
    md: 8,
    xl: 10,
    "2xl": 12,
  });

  const expectedWidths = [
    [320, 12, 296],
    [375, 12, 351],
    [414, 12, 390],
    [768, 24, 720],
  ];

  for (const [
    viewportWidth,
    gutterPx,
    availableContentWidth,
  ] of expectedWidths) {
    const layout = getResponsivePageLayout({ viewportWidth });
    assert.equal(layout.gutterPx, gutterPx);
    assert.equal(layout.availableContentWidth, availableContentWidth);
  }
});

test("scrolling pages clip the inline axis and use their parent height", () => {
  const layout = getResponsivePageLayout({ viewportWidth: 1440, scroll: true });

  assert.deepEqual(layout.viewport, {
    width: "full",
    height: "full",
    minHeight: 0,
    minWidth: 0,
    overflowX: "clip",
    overflowY: "auto",
  });

  assert.deepEqual(layout.shellContent, {
    width: "full",
    height: "full",
    minHeight: 0,
    minWidth: 0,
  });
});

test("fixed caller widths only apply after the mobile breakpoint", () => {
  const layout = resolveResponsiveControlLayout(
    { w: "22rem", ml: 4, mr: 2 },
    "17.5rem",
  );

  assert.deepEqual(layout, {
    w: { base: "full", md: "22rem" },
    ml: { base: 0, md: 4 },
    mr: { base: 0, md: 2 },
  });
});
