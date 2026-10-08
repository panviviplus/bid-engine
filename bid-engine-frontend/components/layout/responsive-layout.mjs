export const PAGE_GUTTERS = Object.freeze({
  base: 3,
  sm: 4,
  md: 6,
  xl: 8,
  "2xl": 10,
});

export const AUTH_PAGE_GUTTERS = Object.freeze({
  base: 4,
  md: 8,
  xl: 10,
  "2xl": 12,
});

/**
 * @typedef {Object} ResponsivePageLayoutOptions
 * @property {number} [viewportWidth]
 * @property {number} [sidebarWidth]
 * @property {boolean} [scroll]
 */

/**
 * @typedef {Object} ResponsivePageLayout
 * @property {number} gutterPx
 * @property {number} availableContentWidth
 * @property {{
 *   width: "full",
 *   height: "full",
 *   minHeight: number,
 *   minWidth: number
 * }} shellContent
 * @property {{
 *   width: "full",
 *   height: "full",
 *   minHeight: number,
 *   minWidth: number,
 *   overflowX: "clip",
 *   overflowY: "auto" | "hidden"
 * }} viewport
 * @property {{
 *   width: "full",
 *   minWidth: number,
 *   maxWidth: "none",
 *   marginInline: number,
 *   paddingInline: typeof PAGE_GUTTERS
 * }} content
 */

function resolveGutterPx(viewportWidth) {
  if (viewportWidth >= 1536) return 40;
  if (viewportWidth >= 1280) return 32;
  if (viewportWidth >= 768) return 24;
  if (viewportWidth >= 480) return 16;
  return 12;
}

/**
 * @param {ResponsivePageLayoutOptions} [options]
 * @returns {ResponsivePageLayout}
 */
export function getResponsivePageLayout(options = {}) {
  const { viewportWidth = 0, sidebarWidth = 0, scroll = true } = options;
  const safeViewportWidth = Math.max(0, Number(viewportWidth) || 0);
  const safeSidebarWidth = Math.max(0, Number(sidebarWidth) || 0);
  const gutterPx = resolveGutterPx(safeViewportWidth);

  return {
    gutterPx,
    availableContentWidth: Math.max(
      0,
      safeViewportWidth - safeSidebarWidth - gutterPx * 2,
    ),
    shellContent: {
      width: "full",
      height: "full",
      minHeight: 0,
      minWidth: 0,
    },
    viewport: {
      width: "full",
      height: "full",
      minHeight: 0,
      minWidth: 0,
      overflowX: "clip",
      overflowY: scroll ? "auto" : "hidden",
    },
    content: {
      width: "full",
      minWidth: 0,
      maxWidth: "none",
      marginInline: 0,
      paddingInline: PAGE_GUTTERS,
    },
  };
}

/**
 * Keep caller-supplied desktop measurements without allowing them to replace
 * the mobile full-width contract.
 *
 * @param {{
 *   w?: string | number,
 *   width?: string | number,
 *   ml?: string | number,
 *   mr?: string | number
 * }} [props]
 * @param {string | number} [defaultDesktopWidth]
 * @returns {{
 *   w: { base: "full", md: string | number },
 *   ml: { base: 0, md: string | number },
 *   mr: { base: 0, md: string | number }
 * }}
 */
export function resolveResponsiveControlLayout(
  props = {},
  defaultDesktopWidth = "auto",
) {
  const desktopWidth = props.w || props.width || defaultDesktopWidth;

  return {
    w: { base: "full", md: desktopWidth },
    ml: { base: 0, md: props.ml || 0 },
    mr: { base: 0, md: props.mr || 0 },
  };
}
