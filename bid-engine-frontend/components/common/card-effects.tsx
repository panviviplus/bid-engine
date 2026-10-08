"use client";

import { Box, keyframes, usePrefersReducedMotion } from "@chakra-ui/react";
import { useMemo } from "react";
import type { CSSProperties } from "react";

/**
 * Hallmark · component: card-effects · genre: modern-minimal · theme: Cobalt (adapted)
 * 卡片活动态氛围效果（招标解析 / 标书生成共用）：
 * - AmbientTopBar：顶部 3px 氛围渐变条（默认深蓝→金；gold 用于大纲待确认）
 * - WaveDots：底部 2px 流动像素点（蓝→青→紫，错峰上跳）
 */

// 底部微像素"流动波"：2px 圆点自左向右错峰向上轻跳（幅度约 6px < 5% 卡片高度）
const wave = keyframes`
  0%, 100% { transform: translateY(0) scale(1); opacity: 0.45; box-shadow: none; }
  50% { transform: translateY(-6px) scale(1.3); opacity: 0.95; box-shadow: 0 0 6px 1px var(--c); }
`;

const WAVE_STOPS: [number, number, number][] = [
  [127, 196, 255], // 蓝
  [94, 234, 212], // 青
  [167, 139, 250], // 紫
];

function buildWaveDots(count = 34) {
  const lerp = (a: number, b: number, t: number) => Math.round(a + (b - a) * t);
  const mix = (
    c1: [number, number, number],
    c2: [number, number, number],
    t: number,
  ) =>
    `rgb(${lerp(c1[0], c2[0], t)}, ${lerp(c1[1], c2[1], t)}, ${lerp(c1[2], c2[2], t)})`;
  const dots: {
    left: string;
    color: string;
    delay: string;
    duration: string;
  }[] = [];
  for (let i = 0; i < count; i += 1) {
    const x = count === 1 ? 0 : i / (count - 1);
    const left = 2 + x * 96;
    const color =
      x < 0.5
        ? mix(WAVE_STOPS[0], WAVE_STOPS[1], x * 2)
        : mix(WAVE_STOPS[1], WAVE_STOPS[2], (x - 0.5) * 2);
    dots.push({
      left: `${left.toFixed(2)}%`,
      color,
      delay: `${(i * 0.05).toFixed(2)}s`,
      duration: `${(2.4 + (i % 4) * 0.12).toFixed(2)}s`,
    });
  }
  return dots;
}

/** 卡片活动态顶部 3px 氛围渐变条 */
export function AmbientTopBar({
  tone = "default",
}: {
  tone?: "default" | "gold";
}) {
  return (
    <Box layerStyle="ambientTopBar" aria-hidden>
      <Box
        h="full"
        bgGradient={
          tone === "gold"
            ? "linear(to-r, gold.500, gold.300)"
            : "linear(to-r, primary.600, gold.500)"
        }
        opacity={tone === "gold" ? 1 : 0.7}
      />
    </Box>
  );
}

/** 卡片活动态底部流动像素点 */
export function WaveDots({ show }: { show: boolean }) {
  const prefersReducedMotion = usePrefersReducedMotion();
  const dots = useMemo(() => buildWaveDots(34), []);
  if (!show) return null;
  return (
    <Box
      position="absolute"
      left="10px"
      right="10px"
      bottom={0}
      height="10px"
      pointerEvents="none"
      aria-hidden
    >
      {dots.map((d, i) => (
        <Box
          key={i}
          position="absolute"
          bottom={0}
          width="2px"
          height="2px"
          borderRadius="1px"
          bg={d.color}
          opacity={0.45}
          animation={
            !prefersReducedMotion
              ? `${wave} ${d.duration} ease-in-out infinite`
              : undefined
          }
          style={
            {
              left: d.left,
              animationDelay: d.delay,
              "--c": d.color,
            } as CSSProperties
          }
        />
      ))}
    </Box>
  );
}
