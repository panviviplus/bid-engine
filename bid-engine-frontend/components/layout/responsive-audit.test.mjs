import assert from "node:assert/strict";
import test from "node:test";

import {
  auditResponsiveSource,
  findRouteCoverageGaps,
} from "../../scripts/responsive-layout-audit.mjs";

test("responsive audit rejects page-level fixed-width containers", () => {
  const violations = auditResponsiveSource(
    "app/(main)/example/page.tsx",
    '<Container maxW="container.xl"><Grid /></Container>',
  );

  assert.deepEqual(
    violations.map((item) => item.rule),
    ["fixed-page-width"],
  );
});

test("responsive audit rejects nested viewport heights in authenticated pages", () => {
  const violations = auditResponsiveSource(
    "app/(main)/example/page.tsx",
    '<Box h="100vh" overflow="auto" />',
  );

  assert.deepEqual(
    violations.map((item) => item.rule),
    ["nested-viewport-height"],
  );
});

test("responsive audit accepts the shared fluid page contract", () => {
  const violations = auditResponsiveSource(
    "app/(main)/example/page.tsx",
    "<PageViewport><PageContent><Grid /></PageContent></PageViewport>",
  );

  assert.deepEqual(violations, []);
});

test("responsive audit rejects desktop-sized minimum tracks inside detail workspaces", () => {
  const violations = auditResponsiveSource(
    "app/(main)/example/[id]/page.tsx",
    '<Box flex="1" minW="560px" />',
  );

  assert.deepEqual(
    violations.map((item) => item.rule),
    ["fixed-workspace-min-width"],
  );
});

test("responsive audit rejects legacy viewport arithmetic and 100vw canvases", () => {
  const violations = auditResponsiveSource(
    "components/example.tsx",
    '<Flex h="calc(100vh - 3.5rem)" w="100vw" />',
  );

  assert.deepEqual(
    violations.map((item) => item.rule),
    ["legacy-viewport-height", "viewport-inline-width"],
  );
});

test("responsive audit rejects filter controls that cannot shrink with the page", () => {
  const violations = auditResponsiveSource(
    "components/common-form/components/Input.tsx",
    '<FormControl w="fit-content"><InputGroup w="17.5rem" /></FormControl>',
  );

  assert.deepEqual(
    violations.map((item) => item.rule),
    ["fixed-filter-control-width", "fixed-filter-control-width"],
  );
});

test("responsive audit rejects legacy viewport units in auth and CSS roots", () => {
  const authViolations = auditResponsiveSource(
    "components/auth/AuthCanvas.tsx",
    '<Flex minH="100vh" />',
  );
  const cssViolations = auditResponsiveSource(
    "styles/globals.css",
    ".page { height: 100vh; width: 100vw; }",
  );

  assert.deepEqual(
    [...authViolations, ...cssViolations].map((item) => item.rule),
    [
      "nested-viewport-height",
      "legacy-viewport-height",
      "viewport-inline-width",
    ],
  );
});

test("route coverage reports both unmapped pages and stale route mappings", () => {
  const gaps = findRouteCoverageGaps(
    ["app/(main)/page.tsx", "app/(main)/new/page.tsx"],
    {
      "app/(main)/page.tsx": ["app/(main)/page.tsx"],
      "app/(main)/removed/page.tsx": ["app/(main)/removed/page.tsx"],
    },
  );

  assert.deepEqual(gaps, {
    unmapped: ["app/(main)/new/page.tsx"],
    stale: ["app/(main)/removed/page.tsx"],
  });
});
