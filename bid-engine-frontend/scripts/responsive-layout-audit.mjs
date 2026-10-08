import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const FIXED_PAGE_WIDTH =
  /(?:maxW|maxWidth)=["'](?:container\.(?:md|xl)|1440px|1480px|1540px|1680px|1720px)["']/g;
const NESTED_VIEWPORT_HEIGHT = /(?:h|minH)=["']100vh["']/g;
const FIXED_WORKSPACE_MIN_WIDTH = /minW=["'](?:[4-9]\d{2}|[1-9]\d{3,})px["']/g;
const LEGACY_VIEWPORT_HEIGHT =
  /(?:h|minH)=["']calc\(100vh[^"']*["']|height:\s*100vh/g;
const VIEWPORT_INLINE_WIDTH = /(?:w|width)=["']100vw["']|width:\s*100vw/g;
const FIXED_FILTER_CONTROL_WIDTH =
  /w=["'](?:fit-content|11\.5rem|17\.5rem)["']/g;

export const ROUTE_SURFACE_MAP = Object.freeze({
  "app/(auth)/signin/page.js": [
    "app/(auth)/signin/page.js",
    "components/signin/experience/SigninExperience.tsx",
    "components/signin/experience/HeroSection.tsx",
    "components/signin/experience/CapabilitySection.tsx",
    "components/signin/experience/EntrySection.tsx",
    "components/signin/experience/SigninAuthCard.tsx",
  ],
  "app/(auth)/signup/page.tsx": [
    "app/(auth)/signup/page.tsx",
    "components/auth/AuthCanvas.tsx",
  ],
  "app/(main)/page.tsx": ["app/(main)/page.tsx"],
  "app/(main)/bid-analysis/page.tsx": [
    "app/(main)/bid-analysis/page.tsx",
    "components/analysis/bid-analysis-v3/workspace.tsx",
  ],
  "app/(main)/bid-analysis/[id]/page.tsx": [
    "app/(main)/bid-analysis/[id]/page.tsx",
    "components/analysis/bid-analysis-v3/workspace.tsx",
    "components/analysis/bid-analysis-v3/detail/panels.tsx",
  ],
  "app/(main)/bid-audit/page.tsx": ["app/(main)/bid-audit/page.tsx"],
  "app/(main)/bid-audit/[id]/page.tsx": [
    "app/(main)/bid-audit/[id]/page.tsx",
    "components/audit/detail.tsx",
  ],
  "app/(main)/bid-audit/rules/page.tsx": [
    "app/(main)/bid-audit/rules/page.tsx",
    "components/audit/rule-library.tsx",
  ],
  "app/(main)/file-feedback/page.tsx": ["app/(main)/file-feedback/page.tsx"],
  "app/(main)/file-gen/page.tsx": ["app/(main)/file-gen/page.tsx"],
  "app/(main)/file-gen/[id]/page.tsx": [
    "app/(main)/file-gen/[id]/page.tsx",
    "components/file-gen/bid-editor.tsx",
    "components/file-gen/outline-review-gate.tsx",
  ],
  "app/(main)/material/page.tsx": ["app/(main)/material/page.tsx"],
  "app/(main)/material/[id]/page.tsx": [
    "app/(main)/material/[id]/page.tsx",
    "app/(main)/material/knowledge/[id]/page.tsx",
  ],
  "app/(main)/material/gallery/page.tsx": [
    "app/(main)/material/gallery/page.tsx",
  ],
  "app/(main)/material/knowledge/page.tsx": [
    "app/(main)/material/knowledge/page.tsx",
  ],
  "app/(main)/material/knowledge/[id]/page.tsx": [
    "app/(main)/material/knowledge/[id]/page.tsx",
  ],
  "app/(main)/material/performance/page.tsx": [
    "app/(main)/material/performance/page.tsx",
    "components/material/type-list-page.tsx",
  ],
  "app/(main)/material/performance/[id]/page.tsx": [
    "app/(main)/material/performance/[id]/page.tsx",
    "app/(main)/material/knowledge/[id]/page.tsx",
  ],
  "app/(main)/material/products/page.tsx": [
    "app/(main)/material/products/page.tsx",
  ],
  "app/(main)/material/qualification/page.tsx": [
    "app/(main)/material/qualification/page.tsx",
    "components/material/type-list-page.tsx",
  ],
  "app/(main)/material/qualification/[id]/page.tsx": [
    "app/(main)/material/qualification/[id]/page.tsx",
    "app/(main)/material/knowledge/[id]/page.tsx",
  ],
  "app/(main)/material/template/page.tsx": [
    "app/(main)/material/template/page.tsx",
    "components/material/type-list-page.tsx",
  ],
  "app/(main)/material/template/[id]/page.tsx": [
    "app/(main)/material/template/[id]/page.tsx",
    "app/(main)/material/knowledge/[id]/page.tsx",
  ],
  "app/(main)/material/templates/page.tsx": [
    "app/(main)/material/templates/page.tsx",
  ],
  "app/(main)/system/llm-config/page.tsx": [
    "app/(main)/system/llm-config/page.tsx",
  ],
  "app/(plain)/pdf-preview/[id]/page.tsx": [
    "app/(plain)/pdf-preview/[id]/page.tsx",
    "components/common/pdf-preview-client.tsx",
  ],
});

const SHARED_LAYOUT_FILES = [
  "app/layout.js",
  "app/(auth)/layout.tsx",
  "app/(main)/layout.tsx",
  "app/(plain)/layout.tsx",
  "components/common/page-header.tsx",
  "components/common-form/components/Input.tsx",
  "components/common-form/components/SelectBase.tsx",
  "components/common-form/components/SelectPaging.tsx",
  "components/common-form/components/data-range-picker.tsx",
  "components/common-form/index.tsx",
  "components/layout/index.js",
  "components/layout/responsive-page.tsx",
  "styles/globals.css",
];

export const AUDITED_FILES = Object.freeze(
  Array.from(
    new Set([
      ...Object.values(ROUTE_SURFACE_MAP).flat(),
      ...SHARED_LAYOUT_FILES,
    ]),
  ).sort(),
);

export function findRouteCoverageGaps(actualRoutes, routeSurfaceMap) {
  const actual = new Set(actualRoutes);
  const mapped = new Set(Object.keys(routeSurfaceMap));

  return {
    unmapped: actualRoutes.filter((route) => !mapped.has(route)).sort(),
    stale: Array.from(mapped)
      .filter((route) => !actual.has(route))
      .sort(),
  };
}

async function discoverRouteFiles(directory, projectRoot) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (entry) => {
      const absolute = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        return discoverRouteFiles(absolute, projectRoot);
      }
      if (!/^page\.(?:js|jsx|ts|tsx|md|mdx)$/.test(entry.name)) return [];
      return [path.relative(projectRoot, absolute).split(path.sep).join("/")];
    }),
  );
  return nested.flat().sort();
}

function lineForIndex(source, index) {
  return source.slice(0, index).split("\n").length;
}

function collectMatches(source, pattern, rule, message) {
  return Array.from(source.matchAll(pattern), (match) => ({
    rule,
    line: lineForIndex(source, match.index || 0),
    message,
  }));
}

export function auditResponsiveSource(filePath, source) {
  const violations = collectMatches(
    source,
    FIXED_PAGE_WIDTH,
    "fixed-page-width",
    "Page-level content must use the full available canvas.",
  );

  if (
    filePath.includes("/(main)/") ||
    filePath.includes("components/material/") ||
    filePath.includes("components/auth/")
  ) {
    violations.push(
      ...collectMatches(
        source,
        NESTED_VIEWPORT_HEIGHT,
        "nested-viewport-height",
        "Authenticated pages inherit their height from the application shell; use full/100dvh only at the root shell.",
      ),
    );
  }

  violations.push(
    ...collectMatches(
      source,
      FIXED_WORKSPACE_MIN_WIDTH,
      "fixed-workspace-min-width",
      "Detail workspace tracks must shrink to the available inline size.",
    ),
    ...collectMatches(
      source,
      LEGACY_VIEWPORT_HEIGHT,
      "legacy-viewport-height",
      "Use the parent shell height or dynamic viewport units instead of 100vh arithmetic.",
    ),
    ...collectMatches(
      source,
      VIEWPORT_INLINE_WIDTH,
      "viewport-inline-width",
      "Use width: 100% so the scrollbar is not included in the page width.",
    ),
  );

  if (filePath.includes("components/common-form/")) {
    violations.push(
      ...collectMatches(
        source,
        FIXED_FILTER_CONTROL_WIDTH,
        "fixed-filter-control-width",
        "Filter controls must use the available width before adopting their desktop measure.",
      ),
    );
  }

  return violations;
}

async function runCli() {
  const scriptPath = fileURLToPath(import.meta.url);
  const projectRoot = path.resolve(path.dirname(scriptPath), "..");
  const sourceFindings = (
    await Promise.all(
      AUDITED_FILES.map(async (file) => {
        const source = await readFile(path.join(projectRoot, file), "utf8");
        return auditResponsiveSource(file, source).map((violation) => ({
          file,
          ...violation,
        }));
      }),
    )
  ).flat();
  const actualRoutes = await discoverRouteFiles(
    path.join(projectRoot, "app"),
    projectRoot,
  );
  const routeGaps = findRouteCoverageGaps(actualRoutes, ROUTE_SURFACE_MAP);
  const findings = [
    ...sourceFindings,
    ...routeGaps.unmapped.map((file) => ({
      file,
      line: 1,
      rule: "unmapped-route",
      message: "Route is not mapped to an audited responsive surface.",
    })),
    ...routeGaps.stale.map((file) => ({
      file,
      line: 1,
      rule: "stale-route-map",
      message: "Responsive route map references a page that no longer exists.",
    })),
  ];

  if (findings.length) {
    for (const finding of findings) {
      process.stderr.write(
        `${finding.file}:${finding.line} [${finding.rule}] ${finding.message}\n`,
      );
    }
    process.exitCode = 1;
    return;
  }

  process.stdout.write(
    `Responsive layout audit passed for ${actualRoutes.length} routes across ${AUDITED_FILES.length} surfaces.\n`,
  );
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  await runCli();
}
