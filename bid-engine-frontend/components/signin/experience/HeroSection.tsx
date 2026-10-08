"use client";

import {
  Box,
  Container,
  Flex,
  Grid,
  Heading,
  Image,
  Text,
} from "@chakra-ui/react";
import {
  motion,
  type MotionValue,
  useMotionValue,
  useTransform,
} from "framer-motion";
import { useCallback, useEffect, useRef } from "react";
import { AUTH_PAGE_GUTTERS } from "@/components/layout/responsive-layout.mjs";

import SigninAuthCard from "./SigninAuthCard";
import { normalizeTimelinePositions } from "./rotation.mjs";
import { HERO_COPY, HERO_LABELS, SIGNIN_STAGES } from "./signin-content.mjs";

type HeroSectionProps = {
  activeStage: number;
  timelineProgress: MotionValue<number>;
  visibleBubbles: number[];
  reducedMotion: boolean;
  authTabIndex: number;
  unifiedSmsAuthEnabled: boolean;
  // eslint-disable-next-line no-unused-vars
  onStageSelect: (...args: [number]) => void;
  // eslint-disable-next-line no-unused-vars
  onStagePositionsChange: (...args: [number[]]) => void;
  // eslint-disable-next-line no-unused-vars
  onAuthTabChange: (...args: [number]) => void;
};

export default function HeroSection({
  activeStage,
  timelineProgress,
  visibleBubbles,
  reducedMotion,
  authTabIndex,
  unifiedSmsAuthEnabled,
  onStageSelect,
  onStagePositionsChange,
  onAuthTabChange,
}: HeroSectionProps) {
  const timelineGridRef = useRef<HTMLDivElement | null>(null);
  const timelineStartX = useMotionValue(0);
  const timelineEndX = useMotionValue(0);
  const runnerX = useTransform<number, number>(
    [timelineProgress, timelineStartX, timelineEndX],
    ([progress, start, end]) => start + progress * (end - start),
  );

  const measureTimeline = useCallback(() => {
    const grid = timelineGridRef.current;
    if (!grid) return;
    const nodes = Array.from(
      grid.querySelectorAll<HTMLElement>("[data-signin-stage-node]"),
    );
    if (nodes.length !== SIGNIN_STAGES.length) return;
    const gridRect = grid.getBoundingClientRect();
    const centers = nodes.map((node) => {
      const rect = node.getBoundingClientRect();
      return rect.left - gridRect.left + rect.width / 2;
    });
    const pathStart = centers[0];
    const pathEnd = Math.max(
      centers[centers.length - 1],
      gridRect.width * 0.97,
    );
    timelineStartX.set(pathStart);
    timelineEndX.set(pathEnd);
    onStagePositionsChange(
      normalizeTimelinePositions(centers, pathStart, pathEnd),
    );
  }, [onStagePositionsChange, timelineEndX, timelineStartX]);

  useEffect(() => {
    const grid = timelineGridRef.current;
    if (!grid) return undefined;
    const initialFrame = window.requestAnimationFrame(measureTimeline);
    const observer = new ResizeObserver(measureTimeline);
    observer.observe(grid);
    return () => {
      window.cancelAnimationFrame(initialFrame);
      observer.disconnect();
    };
  }, [measureTimeline]);

  return (
    <Box
      as="section"
      position="relative"
      minH={{ base: "auto", lg: "100svh" }}
      py={{ base: 6, md: 10, lg: 8 }}
      overflow="clip"
    >
      <Container
        w="full"
        maxW="none"
        px={AUTH_PAGE_GUTTERS}
        position="relative"
        zIndex={1}
      >
        <Grid
          templateColumns={{
            base: "minmax(0, 1fr)",
            xl: "minmax(0, 1fr) clamp(25rem, 25vw, 32rem)",
          }}
          alignItems="start"
          gap={{ base: 10, xl: 10, "2xl": 14 }}
        >
          <Box minW={0}>
            <Flex align="center" gap={3}>
              <Image
                src="/bid-engine-logo.ico"
                alt="标擎 BidEngine Logo"
                boxSize="46px"
                borderRadius="10px"
                objectFit="contain"
              />
              <Text color="white" fontSize="18px" fontWeight="800">
                标擎
              </Text>
            </Flex>

            <Box mt={{ base: 10, lg: 14 }}>
              <Heading
                color="white"
                fontSize={{ base: "4xl", md: "5xl", xl: "6xl" }}
                lineHeight="1.06"
                letterSpacing="-0.045em"
                overflowWrap="anywhere"
              >
                让每一次投标，
                <br />
                都有章可循
              </Heading>
              <Box mt={6} w="full" minW={0}>
                {HERO_COPY.split("\n").map((line) => (
                  <Text
                    key={line}
                    mt={line === HERO_COPY.split("\n")[0] ? 0 : 4}
                    color="signin.textMuted"
                    fontSize={{ base: "md", "2xl": "lg" }}
                    lineHeight="1.85"
                    sx={{ textWrap: "pretty" }}
                  >
                    {line}
                  </Text>
                ))}
              </Box>
              <Flex mt={6} gap={2.5} wrap="wrap">
                {HERO_LABELS.map((label) => (
                  <Text
                    key={label}
                    px={3.5}
                    py={2.5}
                    borderRadius="lg"
                    bg="signin.surface"
                    color="signin.text"
                    border="1px solid"
                    borderColor="signin.line"
                    fontSize="sm"
                    fontWeight="600"
                  >
                    {label}
                  </Text>
                ))}
              </Flex>
            </Box>
          </Box>

          <SigninAuthCard
            tabIndex={authTabIndex}
            onTabChange={onAuthTabChange}
            unifiedSmsAuthEnabled={unifiedSmsAuthEnabled}
          />
        </Grid>

        <Box
          mt={{ base: 10, lg: 8 }}
          p={{ base: 4, md: 5 }}
          borderRadius="16px"
          bg="signin.panel"
          border="1px solid"
          borderColor="signin.line"
        >
          <Flex justify="space-between" gap={4} wrap="wrap">
            <Text color="white" fontSize="sm" fontWeight="800">
              从发现机会到标书交付的完整投标流程
            </Text>
            <Text color="signin.textMuted" fontSize="sm">
              每个环节都可以独立开始
            </Text>
          </Flex>

          <Grid
            ref={timelineGridRef}
            mt={{ base: 4, lg: 12 }}
            templateColumns={{
              base: "minmax(0, 1fr)",
              sm: "repeat(2, minmax(0, 1fr))",
              lg: "repeat(6, minmax(0, 1fr))",
            }}
            gap={{ base: 3, lg: 2 }}
            position="relative"
            _before={{
              content: '""',
              display: { base: "none", lg: "block" },
              position: "absolute",
              left: "8%",
              right: "3%",
              top: "18px",
              height: "1px",
              bgGradient: "linear(to-r, transparent, primary.400, transparent)",
              opacity: 0.8,
            }}
          >
            <Box
              display={{ base: "none", lg: "block" }}
              position="absolute"
              inset={0}
              pointerEvents="none"
              aria-hidden="true"
              zIndex={4}
            >
              <motion.div
                data-signin-timeline-runner
                style={{
                  position: "absolute",
                  top: 12,
                  left: 0,
                  x: runnerX,
                  width: 14,
                  height: 14,
                  marginLeft: -7,
                  borderRadius: 999,
                  background: "var(--chakra-colors-gold-400)",
                  boxShadow: "var(--chakra-shadows-signinGlow)",
                }}
              />
            </Box>
            {SIGNIN_STAGES.map((stage, index) => {
              const active = index === activeStage;
              const bubbleVisible = visibleBubbles.includes(index);
              const desktopReviewOffset = index === 4 ? 72 : 0;
              const desktopActiveOffset = active && !reducedMotion ? -2 : 0;
              const desktopTransform =
                desktopReviewOffset || desktopActiveOffset
                  ? `translate(${desktopReviewOffset}px, ${desktopActiveOffset}px)`
                  : undefined;
              return (
                <Flex
                  key={stage.key}
                  data-signin-stage-node
                  as="button"
                  type="button"
                  minW={0}
                  minH={{ base: "76px", lg: "86px" }}
                  position="relative"
                  direction="column"
                  align={{ base: "flex-start", lg: "center" }}
                  justify={{ base: "center", lg: "flex-end" }}
                  px={{ base: 4, lg: 2 }}
                  py={3}
                  borderRadius="xl"
                  bg={{ base: "signin.surface", lg: "transparent" }}
                  border={{ base: "1px solid", lg: "0" }}
                  borderColor="signin.line"
                  color={active ? "white" : "signin.textMuted"}
                  opacity={{ base: 1, lg: active ? 1 : 0.52 }}
                  cursor="pointer"
                  transition="color 150ms var(--ease-out), opacity 420ms var(--ease-out), transform 420ms var(--ease-out)"
                  transform={{ base: undefined, lg: desktopTransform }}
                  onClick={() => onStageSelect(index)}
                  aria-pressed={active}
                  _focusVisible={{
                    outline: "none",
                    boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                  }}
                  aria-label={`查看${stage.title}阶段`}
                >
                  <Box
                    display={{ base: "none", lg: "block" }}
                    position="absolute"
                    top="14px"
                    left="50%"
                    transform="translateX(-50%)"
                    boxSize="10px"
                    borderRadius="full"
                    bg={active ? "gold.400" : "signin.canvas"}
                    border="2px solid"
                    borderColor={active ? "gold.300" : "primary.700"}
                    boxShadow={active ? "signinGlow" : "none"}
                    zIndex={2}
                  />
                  <Text fontSize="sm" fontWeight="800">
                    {stage.title}
                  </Text>
                  <Text mt={1} fontSize="xs" opacity={0.76}>
                    {stage.action}
                  </Text>

                  {stage.bubbles.length > 0 && (
                    <Flex
                      as={motion.div}
                      position={{ base: "static", lg: "absolute" }}
                      left={{ lg: "50%" }}
                      bottom={{ lg: "72px" }}
                      mt={{ base: 2, lg: 0 }}
                      justify="center"
                      gap={1.5}
                      minW={{ lg: "240px" }}
                      opacity={bubbleVisible || reducedMotion ? 1 : 0}
                      transform={
                        bubbleVisible || reducedMotion
                          ? { base: "none", lg: "translate(-50%, 0)" }
                          : { base: "none", lg: "translate(-50%, 6px)" }
                      }
                      pointerEvents="none"
                      transition="opacity 220ms var(--ease-out), transform 220ms var(--ease-out)"
                    >
                      {stage.bubbles.map((bubble) => (
                        <Text
                          key={bubble}
                          px={3}
                          py={2}
                          borderRadius="full"
                          bg="signin.surfaceStrong"
                          color="gold.200"
                          border="1px solid"
                          borderColor="gold.700"
                          fontSize="sm"
                          fontWeight="700"
                          whiteSpace="nowrap"
                        >
                          {bubble}
                        </Text>
                      ))}
                    </Flex>
                  )}
                </Flex>
              );
            })}
          </Grid>
        </Box>
      </Container>
    </Box>
  );
}
