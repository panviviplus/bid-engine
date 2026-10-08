"use client";

/* Hallmark · component: scroll-to-top · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 长列表（情报大厅 / 情报管理）滚过一屏后出现，点击平滑回到列表顶部。
 * states: hidden（不可见但占位 fixed，不拦截点击）· visible · hover · focus-visible · active
 * a11y: aria-label + Tooltip；prefers-reduced-motion 时改用即时定位
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React, { useEffect, useState } from "react";
import { Box, Icon, Tooltip } from "@chakra-ui/react";
import { FiArrowUp } from "react-icons/fi";
import { useReducedMotion } from "framer-motion";

export default function ScrollToTopButton({
  targetId,
  threshold = 400,
  label = "回到顶部",
}: {
  /** 滚动容器 id（页面自身的 PageViewport） */
  targetId: string;
  threshold?: number;
  label?: string;
}) {
  const [visible, setVisible] = useState(false);
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    const container = document.getElementById(targetId);
    if (!container) return undefined;
    const onScroll = () => setVisible(container.scrollTop > threshold);
    onScroll();
    container.addEventListener("scroll", onScroll, { passive: true });
    return () => container.removeEventListener("scroll", onScroll);
  }, [targetId, threshold]);

  const scrollToTop = () => {
    const container = document.getElementById(targetId);
    if (!container) return;
    if (typeof container.scrollTo === "function") {
      container.scrollTo({
        top: 0,
        behavior: reducedMotion ? "auto" : "smooth",
      });
      return;
    }
    // 老浏览器与测试环境（jsdom）没有 Element.scrollTo，退化为即时定位
    container.scrollTop = 0;
  };

  return (
    <Tooltip label={label} placement="left" openDelay={400}>
      <Box
        as="button"
        type="button"
        aria-label={label}
        aria-hidden={!visible}
        tabIndex={visible ? 0 : -1}
        position="fixed"
        right={{ base: 4, md: 6 }}
        bottom={{ base: 4, md: 6 }}
        zIndex="sticky"
        display="flex"
        alignItems="center"
        justifyContent="center"
        boxSize="44px"
        borderRadius="full"
        bg="workbench.control"
        color="white"
        boxShadow="0 10px 26px rgba(11, 27, 43, 0.28)"
        opacity={visible ? 1 : 0}
        transform={visible ? "translateY(0)" : "translateY(8px)"}
        pointerEvents={visible ? "auto" : "none"}
        transition={
          reducedMotion
            ? "none"
            : "opacity .18s cubic-bezier(0.16,1,0.3,1), transform .18s cubic-bezier(0.16,1,0.3,1), background-color .16s ease-out"
        }
        _hover={{
          bg: "workbench.controlRaised",
          transform: "translateY(-2px)",
        }}
        _active={{ bg: "workbench.controlRaised", transform: "scale(0.94)" }}
        _focusVisible={{
          outline: "none",
          boxShadow: "0 0 0 3px var(--chakra-colors-primary-300)",
        }}
        onClick={scrollToTop}
      >
        <Icon as={FiArrowUp} boxSize={5} aria-hidden />
      </Box>
    </Tooltip>
  );
}
