/* Hallmark · genre: atmospheric · macrostructure: Split Studio · theme: chakra-smart-bid auth-night · enrichment: Tier-A product micro-demos · nav: N9 · footer: Ft2 */
/* Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V5 */

"use client";

import { ArrowUpIcon } from "@chakra-ui/icons";
import { Box, IconButton } from "@chakra-ui/react";
import { MotionConfig } from "framer-motion";
import { env } from "next-runtime-env";
import { useCallback, useEffect, useRef, useState } from "react";

import CapabilitySection from "./CapabilitySection";
import EntrySection from "./EntrySection";
import HeroSection from "./HeroSection";
import { isUnifiedSmsAuthEnabled } from "./feature-flags.mjs";
import { getVisibleBubbleIndexes } from "./rotation.mjs";
import { SIGNIN_STAGES } from "./signin-content.mjs";
import { useTimelineProgress } from "./use-timeline-progress";

const TIMELINE_DURATION_MS = 20_000;
const BUBBLE_LINGER_MS = 8_000;

export default function SigninExperience() {
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const activatedAtRef = useRef<Array<number | null>>(
    Array.from({ length: SIGNIN_STAGES.length }, () => null),
  );
  const bubbleTimersRef = useRef<Set<number>>(new Set());
  const [showBackToTop, setShowBackToTop] = useState(false);
  const [visibleBubbles, setVisibleBubbles] = useState<number[]>([]);
  const [authTabIndex, setAuthTabIndex] = useState(0);
  const unifiedSmsAuthEnabled = isUnifiedSmsAuthEnabled(
    env("NEXT_PUBLIC_UNIFIED_SMS_AUTH"),
  );
  const {
    index: activeStage,
    progress: timelineProgress,
    selectIndex: selectStage,
    reducedMotion,
    setStagePositions,
  } = useTimelineProgress({
    count: SIGNIN_STAGES.length,
    durationMs: TIMELINE_DURATION_MS,
    requireDesktop: true,
  });

  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return undefined;
    const onScroll = () =>
      setShowBackToTop(scroller.scrollTop > scroller.clientHeight * 0.85);
    onScroll();
    scroller.addEventListener("scroll", onScroll, { passive: true });
    return () => scroller.removeEventListener("scroll", onScroll);
  }, []);

  const refreshVisibleBubbles = useCallback(() => {
    setVisibleBubbles(
      getVisibleBubbleIndexes(
        activatedAtRef.current,
        Date.now(),
        BUBBLE_LINGER_MS,
      ),
    );
  }, []);

  useEffect(() => {
    if (reducedMotion || SIGNIN_STAGES[activeStage].bubbles.length === 0)
      return;
    activatedAtRef.current[activeStage] = Date.now();
    refreshVisibleBubbles();
    const timer = window.setTimeout(() => {
      bubbleTimersRef.current.delete(timer);
      refreshVisibleBubbles();
    }, BUBBLE_LINGER_MS);
    bubbleTimersRef.current.add(timer);
  }, [activeStage, reducedMotion, refreshVisibleBubbles]);

  useEffect(
    () => () => {
      bubbleTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      bubbleTimersRef.current.clear();
    },
    [],
  );

  const scrollToTop = useCallback((password = false) => {
    setAuthTabIndex(password ? 1 : 0);
    scrollerRef.current?.scrollTo({ top: 0, behavior: "smooth" });
  }, []);

  return (
    <MotionConfig reducedMotion="user">
      <Box
        ref={scrollerRef}
        h="100dvh"
        overflowY="auto"
        overflowX="clip"
        position="relative"
        bg="signin.canvas"
        bgImage="radial-gradient(circle at 18% 8%, var(--chakra-colors-signin-bloomPrimary), transparent 42%), radial-gradient(circle at 82% 88%, var(--chakra-colors-signin-bloomSecondary), transparent 38%)"
        className="thin-scrollbars"
      >
        <Box
          position="fixed"
          right={{ base: 4, md: 6 }}
          bottom={{ base: 5, md: 7 }}
          zIndex={50}
          opacity={showBackToTop ? 1 : 0}
          transform={showBackToTop ? "translateY(0)" : "translateY(10px)"}
          transition="opacity 180ms ease-out, transform 180ms ease-out"
          pointerEvents={showBackToTop ? "auto" : "none"}
        >
          <IconButton
            aria-label="回到顶部"
            icon={<ArrowUpIcon />}
            onClick={() => scrollToTop(false)}
            borderRadius="full"
            bg="signin.glass"
            color="white"
            border="1px solid"
            borderColor="signin.line"
            backdropFilter="blur(14px)"
            boxShadow="signinFloating"
            _hover={{ bg: "signin.glassHover", transform: "translateY(-1px)" }}
            _active={{ transform: "translateY(0)" }}
            _focusVisible={{
              outline: "none",
              boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
            }}
          />
        </Box>

        <HeroSection
          activeStage={activeStage}
          timelineProgress={timelineProgress}
          visibleBubbles={visibleBubbles}
          reducedMotion={reducedMotion}
          authTabIndex={authTabIndex}
          unifiedSmsAuthEnabled={unifiedSmsAuthEnabled}
          onStageSelect={selectStage}
          onStagePositionsChange={setStagePositions}
          onAuthTabChange={setAuthTabIndex}
        />
        <CapabilitySection />
        <EntrySection
          unifiedSmsAuthEnabled={unifiedSmsAuthEnabled}
          onLoginClick={() => scrollToTop(false)}
          onPasswordLoginClick={() => scrollToTop(true)}
        />
      </Box>
    </MotionConfig>
  );
}
