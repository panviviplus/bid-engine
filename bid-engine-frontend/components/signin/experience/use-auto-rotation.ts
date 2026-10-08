"use client";

import {
  type FocusEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useReducedMotion } from "framer-motion";

import {
  getNextIndex,
  shouldDisableResponsiveMotion,
  shouldPauseRotation,
} from "./rotation.mjs";

const DESKTOP_MOTION_QUERY = "(min-width: 48em)";

type AutoRotationOptions = {
  count: number;
  intervalMs: number;
  initialIndex?: number;
  disabled?: boolean;
  requireDesktop?: boolean;
};

export function useAutoRotation({
  count,
  intervalMs,
  initialIndex = 0,
  disabled = false,
  requireDesktop = false,
}: AutoRotationOptions) {
  const reducedMotion = Boolean(useReducedMotion());
  const [index, setIndex] = useState(initialIndex);
  const [hovering, setHovering] = useState(false);
  const [focusWithin, setFocusWithin] = useState(false);
  const [documentHidden, setDocumentHidden] = useState(false);
  const [desktopMatches, setDesktopMatches] = useState(false);

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
    disabled:
      disabled ||
      shouldDisableResponsiveMotion({ requireDesktop, desktopMatches }),
    reducedMotion,
    hovering,
    focusWithin,
    documentHidden,
  });

  useEffect(() => {
    if (paused || count <= 1) return undefined;
    const timer = window.setInterval(
      () => setIndex((current) => getNextIndex(current, count)),
      intervalMs,
    );
    return () => window.clearInterval(timer);
  }, [count, intervalMs, paused]);

  const selectIndex = useCallback(
    (nextIndex: number) => {
      if (!Number.isInteger(nextIndex) || count <= 0) return;
      setIndex(Math.max(0, Math.min(nextIndex, count - 1)));
    },
    [count],
  );

  const interactionProps = useMemo(
    () => ({
      onMouseEnter: () => setHovering(true),
      onMouseLeave: () => setHovering(false),
      onFocusCapture: () => setFocusWithin(true),
      onBlurCapture: (event: FocusEvent<HTMLElement>) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setFocusWithin(false);
        }
      },
    }),
    [],
  );

  return {
    index,
    selectIndex,
    paused,
    reducedMotion,
    interactionProps,
  };
}
