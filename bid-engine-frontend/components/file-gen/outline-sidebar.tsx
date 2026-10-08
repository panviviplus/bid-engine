"use client";

/* eslint-disable no-unused-vars */
/* Hallmark · component: resizable-sidebar · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active · drag · keyboard
 * contrast: pass (primary.400 accent on white)
 */
import { useCallback, useRef, useState } from "react";
import type {
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
  ReactNode,
} from "react";
import { Box, BoxProps } from "@chakra-ui/react";

const MIN_WIDTH = 220;
const MAX_WIDTH = 480;
const HIT_AREA = 10; // 手柄热区宽度 px
const KEYBOARD_STEP = 8;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, Math.round(value)));
}

type OutlineSidebarProps = BoxProps & {
  /** 无记忆时的默认宽度 */
  defaultWidth?: number;
  /** localStorage key；不传则不持久化 */
  storageKey?: string;
  minWidth?: number;
  maxWidth?: number;
  /** 蓝图页 <md 全宽态：容器全宽、手柄隐藏 */
  fullWidthOnBase?: boolean;
  children: ReactNode;
};

export default function OutlineSidebar({
  defaultWidth = 280,
  storageKey,
  minWidth = MIN_WIDTH,
  maxWidth = MAX_WIDTH,
  fullWidthOnBase = false,
  children,
  ...rest
}: OutlineSidebarProps) {
  const [width, setWidth] = useState<number>(() => {
    if (!storageKey) return defaultWidth;
    try {
      const stored = Number(window.localStorage.getItem(storageKey));
      if (Number.isFinite(stored) && stored > 0) {
        return clamp(stored, minWidth, maxWidth);
      }
    } catch {
      // 忽略读取失败，回退默认宽度
    }
    return defaultWidth;
  });
  const widthRef = useRef(width);
  widthRef.current = width;

  const persist = useCallback(
    (value: number) => {
      if (!storageKey) return;
      try {
        window.localStorage.setItem(storageKey, String(value));
      } catch {
        // 忽略写入失败
      }
    },
    [storageKey],
  );

  const handleMouseDown = useCallback(
    (e: ReactMouseEvent<HTMLDivElement>) => {
      e.preventDefault();
      const startX = e.clientX;
      const startWidth = widthRef.current;
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      const onMove = (ev: globalThis.MouseEvent) => {
        const next = clamp(
          startWidth + (ev.clientX - startX),
          minWidth,
          maxWidth,
        );
        setWidth(next);
      };
      const onUp = () => {
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
        persist(widthRef.current);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [minWidth, maxWidth, persist],
  );

  const handleKeyDown = useCallback(
    (e: ReactKeyboardEvent<HTMLDivElement>) => {
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      e.preventDefault();
      e.stopPropagation();
      const delta = e.key === "ArrowRight" ? KEYBOARD_STEP : -KEYBOARD_STEP;
      const next = clamp(widthRef.current + delta, minWidth, maxWidth);
      setWidth(next);
      persist(next);
    },
    [minWidth, maxWidth, persist],
  );

  const handleElement = (
    <Box
      position="absolute"
      right="0"
      top="0"
      bottom="0"
      w={`${HIT_AREA}px`}
      zIndex={2}
      cursor="col-resize"
      role="separator"
      aria-orientation="vertical"
      aria-label="调整大纲栏宽度"
      aria-valuenow={width}
      aria-valuemin={minWidth}
      aria-valuemax={maxWidth}
      tabIndex={0}
      display={fullWidthOnBase ? { base: "none", md: "block" } : "block"}
      onMouseDown={handleMouseDown}
      onKeyDown={handleKeyDown}
      _focusVisible={{
        outline: "none",
        _after: { bg: "primary.400", w: "3px" },
      }}
      _hover={{
        _after: { bg: "primary.400", w: "3px" },
      }}
      _active={{
        _after: { bg: "primary.500", w: "3px" },
      }}
      _after={{
        content: '""',
        position: "absolute",
        right: "3px",
        top: 0,
        bottom: 0,
        w: "2px",
        bg: "neutral.200",
        borderRadius: "full",
        transition: "background-color 0.15s ease",
      }}
    />
  );

  return (
    <Box
      w={fullWidthOnBase ? { base: "100%", md: `${width}px` } : `${width}px`}
      minW={fullWidthOnBase ? { base: "0", md: `${width}px` } : `${width}px`}
      position="relative"
      flexShrink={0}
      {...rest}
    >
      {children}
      {handleElement}
    </Box>
  );
}
