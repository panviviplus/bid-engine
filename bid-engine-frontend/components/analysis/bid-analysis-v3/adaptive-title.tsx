"use client";

/*
 * AdaptiveProjectTitle：招标解析列表卡片的项目名称标题。
 *
 * 交互约定（业界成熟的 text-fit + 条件 tooltip 模式）：
 *  1. 固定最多 2 行（-webkit-line-clamp），长名称优先自适应缩小字号，而不是直接省略；
 *  2. 字号上限 18px、下限 13px，按 1px 步进收敛，缩到下限仍放不下才保留省略号；
 *  3. 仅当文本被截断（溢出）时才渲染 Tooltip 显示完整名称；能渲染完整时绝无气泡。
 *
 * 测量要点：
 *  - 溢出检测复用 OverflowTooltip 的 scrollHeight/scrollWidth 与 clientHeight/clientWidth 对比思路；
 *  - 容器宽度变化（ResizeObserver，仅比较宽度）与字体加载完成后，从最大字号重新适配，
 *    避免“只缩不增”和高度变化触发观察器的死循环。
 */

import { memo, useCallback, useLayoutEffect, useRef, useState } from "react";
import { Box, Text, Tooltip } from "@chakra-ui/react";

const TITLE_MAX_FONT = 18;
const TITLE_MIN_FONT = 13;
const TITLE_STEP = 1;

function AdaptiveProjectTitle({ name }: { name: string }) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const textRef = useRef<HTMLParagraphElement>(null);
  const widthRef = useRef(0);
  const nameRef = useRef(name);
  const fontSizeRef = useRef(TITLE_MAX_FONT);
  const [fontSize, setFontSize] = useState(TITLE_MAX_FONT);
  const [truncated, setTruncated] = useState(false);

  const fit = useCallback(() => {
    const el = textRef.current;
    if (!el) {
      return;
    }
    const overflowing =
      el.scrollHeight - el.clientHeight > 1 || el.scrollWidth - el.clientWidth > 1;
    if (overflowing && fontSizeRef.current > TITLE_MIN_FONT) {
      fontSizeRef.current -= TITLE_STEP;
      setFontSize(fontSizeRef.current);
      return;
    }
    setTruncated(overflowing);
  }, []);

  useLayoutEffect(() => {
    if (nameRef.current !== name) {
      // 名称变化：从最大字号重新适配，避免沿用上一个长名称收敛后的小字号。
      nameRef.current = name;
      fontSizeRef.current = TITLE_MAX_FONT;
      setFontSize(TITLE_MAX_FONT);
      return;
    }
    fit();
  }, [fit, fontSize, name]);

  useLayoutEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap || typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect?.width ?? 0;
      // 仅宽度变化时重新适配；高度变化（缩字引起）不触发，避免死循环。
      if (Math.abs(width - widthRef.current) > 1) {
        widthRef.current = width;
        fontSizeRef.current = TITLE_MAX_FONT;
        setFontSize(TITLE_MAX_FONT);
      }
    });
    observer.observe(wrap);
    return () => observer.disconnect();
  }, []);

  useLayoutEffect(() => {
    // 字体加载完成后重新适配，避免 web 字体异步加载导致测量偏小。
    if (typeof document === "undefined" || !document.fonts?.ready) {
      return;
    }
    let cancelled = false;
    document.fonts.ready
      .then(() => {
        if (cancelled) {
          return;
        }
        fontSizeRef.current = TITLE_MAX_FONT;
        setFontSize(TITLE_MAX_FONT);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const content = (
    <Box ref={wrapRef} minW={0} wordBreak="break-word">
      <Text
        ref={textRef}
        fontSize={`${fontSize}px`}
        lineHeight="1.4"
        fontWeight="750"
        color="workbench.text"
        noOfLines={2}
      >
        {name}
      </Text>
    </Box>
  );

  // 能完整渲染时直接输出文本；仅截断时用气泡展示完整名称。
  if (!truncated) {
    return content;
  }
  return (
    <Tooltip hasArrow label={name} placement="top">
      {content}
    </Tooltip>
  );
}

export default memo(AdaptiveProjectTitle);
