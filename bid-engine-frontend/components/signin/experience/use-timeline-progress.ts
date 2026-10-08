"use client";

import {
  useAnimationFrame,
  useMotionValue,
  useReducedMotion,
} from "framer-motion";
import { useCallback, useEffect, useRef, useState } from "react";

import {
  advanceTimelineProgress,
  getTimelineStageIndex,
  shouldDisableResponsiveMotion,
  shouldPauseRotation,
} from "./rotation.mjs";

const DESKTOP_MOTION_QUERY = "(min-width: 48em)";

type TimelineProgressOptions = {
  count: number;
  durationMs: number;
  requireDesktop?: boolean;
};

export function useTimelineProgress({
  count,
  durationMs,
  requireDesktop = false,
}: TimelineProgressOptions) {
  const reducedMotion = Boolean(useReducedMotion());
  const progress = useMotionValue(0);
  const [index, setIndex] = useState(0);
  const [desktopMatches, setDesktopMatches] = useState(!requireDesktop);
  const [documentHidden, setDocumentHidden] = useState(false);
  const stagePositionsRef = useRef(
    Array.from({ length: count }, (_, stageIndex) =>
      count <= 1 ? 0 : stageIndex / (count - 1),
    ),
  );
  const lastFrameRef = useRef<number | null>(null);

  useEffect(() => {
    if (!requireDesktop) return undefined;
    const mediaQuery = window.matchMedia(DESKTOP_MOTION_QUERY);
    const updateDesktopMatch = () => setDesktopMatches(mediaQuery.matches);
    updateDesktopMatch();
    mediaQuery.addEventListener("change", updateDesktopMatch);
    return () => mediaQuery.removeEventListener("change", updateDesktopMatch);
  }, [requireDesktop]);

  useEffect(() => {
    const updateVisibility = () => setDocumentHidden(document.hidden);
    updateVisibility();
    document.addEventListener("visibilitychange", updateVisibility);
    return () =>
      document.removeEventListener("visibilitychange", updateVisibility);
  }, []);

  const paused = shouldPauseRotation({
    disabled: shouldDisableResponsiveMotion({
      requireDesktop,
      desktopMatches,
    }),
    reducedMotion,
    documentHidden,
  });

  useEffect(() => {
    lastFrameRef.current = null;
  }, [paused]);

  useAnimationFrame((time) => {
    if (paused || count <= 1 || durationMs <= 0) {
      lastFrameRef.current = null;
      return;
    }
    if (lastFrameRef.current === null) {
      lastFrameRef.current = time;
      return;
    }

    const elapsedMs = Math.min(Math.max(time - lastFrameRef.current, 0), 250);
    lastFrameRef.current = time;
    const nextProgress = advanceTimelineProgress(
      progress.get(),
      elapsedMs,
      durationMs,
    );
    progress.set(nextProgress);
    const nextIndex = getTimelineStageIndex(
      nextProgress,
      stagePositionsRef.current,
    );
    setIndex((currentIndex) =>
      currentIndex === nextIndex ? currentIndex : nextIndex,
    );
  });

  const setStagePositions = useCallback(
    (positions: number[]) => {
      if (positions.length !== count) return;
      stagePositionsRef.current = positions;
      const nextIndex = getTimelineStageIndex(progress.get(), positions);
      setIndex(nextIndex);
    },
    [count, progress],
  );

  const selectIndex = useCallback(
    (nextIndex: number) => {
      if (!Number.isInteger(nextIndex) || count <= 0) return;
      const safeIndex = Math.max(0, Math.min(nextIndex, count - 1));
      progress.set(
        stagePositionsRef.current[safeIndex] ??
          (count <= 1 ? 0 : safeIndex / (count - 1)),
      );
      setIndex(safeIndex);
      lastFrameRef.current = null;
    },
    [count, progress],
  );

  return {
    index,
    progress,
    reducedMotion,
    selectIndex,
    setStagePositions,
  };
}
