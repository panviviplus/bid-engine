export function getNextIndex(current, count) {
  if (!Number.isInteger(count) || count <= 0) return 0;
  return (current + 1) % count;
}

export function getVisibleBubbleIndexes(activatedAt, now, lingerMs) {
  return activatedAt.reduce((indexes, timestamp, index) => {
    if (
      typeof timestamp === "number" &&
      timestamp <= now &&
      now - timestamp < lingerMs
    ) {
      indexes.push(index);
    }
    return indexes;
  }, []);
}

export function getDownstreamIndexes(startIndex, count) {
  if (!Number.isInteger(count) || count <= 0) return [];
  const start = Math.max(0, Math.min(startIndex, count - 1));
  return Array.from({ length: count - start }, (_, offset) => start + offset);
}

export function shouldPauseRotation({
  disabled = false,
  reducedMotion = false,
  hovering = false,
  focusWithin = false,
  documentHidden = false,
} = {}) {
  return disabled || reducedMotion || hovering || focusWithin || documentHidden;
}

export function shouldDisableResponsiveMotion({
  requireDesktop = false,
  desktopMatches = false,
} = {}) {
  return requireDesktop && !desktopMatches;
}

export function normalizeTimelinePositions(
  positions,
  pathStart = positions?.[0],
  pathEnd = positions?.[positions.length - 1],
) {
  if (!Array.isArray(positions) || positions.length === 0) return [];
  const span = pathEnd - pathStart;
  if (!Number.isFinite(span) || span <= 0) {
    return positions.map(() => 0);
  }
  return positions.map((position) =>
    Math.max(0, Math.min((position - pathStart) / span, 1)),
  );
}

export function getTimelineStageIndex(progress, positions) {
  if (!Array.isArray(positions) || positions.length === 0) return 0;
  const safeProgress = Number.isFinite(progress)
    ? Math.max(0, Math.min(progress, 1))
    : 0;
  let activeIndex = 0;
  for (let index = 1; index < positions.length; index += 1) {
    if (safeProgress < positions[index]) break;
    activeIndex = index;
  }
  return activeIndex;
}

export function advanceTimelineProgress(
  currentProgress,
  elapsedMs,
  durationMs,
) {
  if (!Number.isFinite(durationMs) || durationMs <= 0) return 0;
  const safeProgress = Number.isFinite(currentProgress) ? currentProgress : 0;
  const safeElapsed = Number.isFinite(elapsedMs) ? Math.max(0, elapsedMs) : 0;
  const nextProgress = (safeProgress + safeElapsed / durationMs) % 1;
  return Number(nextProgress.toFixed(10));
}
