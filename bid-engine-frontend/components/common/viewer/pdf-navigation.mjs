/**
 * @param {Array<{
 *   top?: number;
 *   left?: number;
 *   width?: number;
 *   height?: number;
 *   page_index?: number;
 *   active?: boolean;
 * }>} highlightAreasSource
 */
export function mapHighlightAreas(highlightAreasSource = []) {
  return highlightAreasSource.map((area) => ({
    top: (area?.top || 0) * 100,
    left: (area?.left || 0) * 100,
    width: (area?.width || 0) * 100,
    height: (area?.height || 0) * 100,
    pageIndex: area?.page_index || 0,
    active: area?.active !== false,
  }));
}

/**
 * @param {Parameters<typeof mapHighlightAreas>[0]} highlightAreasSource
 * @param {boolean} isDocumentLoaded
 */
export function getHighlightNavigationTarget(
  highlightAreasSource,
  isDocumentLoaded,
) {
  if (!isDocumentLoaded) return null;
  const mappedAreas = mapHighlightAreas(highlightAreasSource);
  return mappedAreas.find((area) => area.active) || mappedAreas[0] || null;
}
