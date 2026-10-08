"use client";

/*
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 * Hallmark · macrostructure: deep-sea command deck + evidence split workspace
 * genre: modern-minimal · tone: professional/atmospheric · anchor: deep-sea navy + evidence gold
 * source audit: contrast pass · motion pass · honest copy pass · responsive rules pass
 */

import {
  Badge,
  Box,
  type BoxProps,
  Flex,
  HStack,
  Icon,
  Text,
  VStack,
} from "@chakra-ui/react";
import { motion, useReducedMotion } from "framer-motion";
import {
  FiActivity,
  FiAlertTriangle,
  FiCheck,
  FiPause,
  FiX,
} from "react-icons/fi";
import { forwardRef, type ReactNode } from "react";

const MotionBox = motion(Box);

export function WorkspaceShell({
  children,
  maxWidth = "none",
  contentProps,
  ...props
}: {
  children: ReactNode;
  maxWidth?: string;
  contentProps?: BoxProps;
  [key: string]: any;
}) {
  return (
    <Box
      minH="100%"
      minW={0}
      overflowX="clip"
      bg="workbench.canvas"
      p={{ base: 3, md: 6, xl: 8 }}
      {...props}
    >
      <Box
        w="full"
        minW={0}
        maxW={maxWidth}
        mx={maxWidth === "none" ? 0 : "auto"}
        {...contentProps}
      >
        {children}
      </Box>
    </Box>
  );
}

export function CommandBar({
  children,
  ...props
}: {
  children: ReactNode;
  [key: string]: any;
}) {
  return (
    <Flex
      gap={3}
      minH="52px"
      minW={0}
      px={3}
      py={2}
      align={{ base: "stretch", md: "center" }}
      justify="space-between"
      direction={{ base: "column", md: "row" }}
      bg="workbench.paper"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="12px"
      {...props}
    >
      {children}
    </Flex>
  );
}

export const STAGES = [
  { key: "document_preprocessing", label: "文档预处理", start: 0, end: 5 },
  { key: "document_parsing", label: "全文解析", start: 5, end: 40 },
  { key: "document_summary", label: "文档摘要", start: 40, end: 45 },
  { key: "chapter_identifying", label: "章节识别", start: 45, end: 55 },
  { key: "chapter_fact_extracting", label: "事实提取", start: 55, end: 90 },
  { key: "content_consolidating", label: "内容归并", start: 90, end: 100 },
] as const;

// 旧版本存量项目的阶段名展示兜底，避免已完成项目进度条显示异常。
const LEGACY_STAGE_LABELS: Record<string, string> = {
  summary_generating: "摘要生成",
};

export const stageLabel = (stage?: string) =>
  STAGES.find((item) => item.key === stage)?.label ||
  LEGACY_STAGE_LABELS[stage || ""] ||
  "等待调度";

export const stageProgressFloor = (stage?: string) =>
  STAGES.find((item) => item.key === stage)?.start || 0;

function stageDotAlignment(index: number) {
  if (index === 0) return "flex-start";
  if (index === STAGES.length - 1) return "flex-end";
  return "center";
}

export function SignalBadge({ status }: { status: string }) {
  const config = {
    running: { label: "解析运行中", color: "info", icon: FiActivity },
    paused: { label: "解析已暂停", color: "neutral", icon: FiPause },
    succeeded: { label: "解析完成", color: "green", icon: FiCheck },
    succeeded_with_warnings: {
      label: "完成 · 有告警",
      color: "orange",
      icon: FiAlertTriangle,
    },
    failed: { label: "解析失败", color: "red", icon: FiX },
  }[status] || { label: status || "未知", color: "gray", icon: FiActivity };
  return (
    <Badge
      display="inline-flex"
      alignItems="center"
      gap={1.5}
      px={2.5}
      py={1}
      borderRadius="full"
      colorScheme={config.color}
      variant="subtle"
      fontSize="10px"
      letterSpacing="0.04em"
    >
      <Icon as={config.icon} boxSize={3} />
      {config.label}
    </Badge>
  );
}

export function ProgressCircuit({
  stage,
  progress,
  status,
  compact = false,
}: {
  stage?: string;
  progress: number;
  status?: string;
  compact?: boolean;
}) {
  const reduced = useReducedMotion();
  const activeIndex = STAGES.findIndex((item) => item.key === stage);
  return (
    <Box role="group" aria-label={`解析进度 ${progress}%`}>
      <Flex align="center" position="relative" py={compact ? 1 : 2}>
        <Box
          position="absolute"
          left={1}
          right={1}
          h="1px"
          bg="whiteAlpha.300"
        />
        <Box
          position="absolute"
          left={1}
          w={`${Math.max(0, Math.min(100, progress))}%`}
          h="1px"
          bg="gold.400"
        />
        {STAGES.map(({ key, label }, index) => {
          const reached = index <= activeIndex || progress === 100;
          const active = key === stage && progress < 100;
          const failed = active && status === "failed";
          let background = "whiteAlpha.400";
          if (reached) background = "gold.400";
          if (failed) background = "error.400";
          return (
            <Box
              key={key}
              flex={1}
              display="flex"
              justifyContent={stageDotAlignment(index)}
              position="relative"
            >
              <MotionBox
                aria-label={label}
                title={label}
                w={active ? 2.5 : 2}
                h={active ? 2.5 : 2}
                borderRadius="full"
                bg={background}
                border="2px solid"
                borderColor="workbench.control"
                animate={
                  active && !failed && !reduced
                    ? { opacity: [0.55, 1, 0.55] }
                    : { opacity: 1 }
                }
                transition={{
                  duration: 1.4,
                  repeat: active && !failed && !reduced ? Infinity : 0,
                }}
              />
            </Box>
          );
        })}
      </Flex>
      {!compact && (
        <HStack justify="space-between" color="whiteAlpha.700" fontSize="xs">
          <Text>{stageLabel(stage)}</Text>
          <Text fontFamily="mono" sx={{ fontVariantNumeric: "tabular-nums" }}>
            {progress}%
          </Text>
        </HStack>
      )}
    </Box>
  );
}

export const DataSurface = forwardRef<
  HTMLDivElement,
  { children: ReactNode; [key: string]: any }
>(function DataSurface({ children, ...props }, ref) {
  return (
    <Box
      ref={ref}
      bg="workbench.paper"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="14px"
      boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
      {...props}
    >
      {children}
    </Box>
  );
});

export function AIStatusRail({
  status,
  stage,
  progress,
  metrics,
}: {
  status: string;
  stage?: string;
  progress: number;
  metrics: Array<{ label: string; value: string | number }>;
}) {
  return (
    <Box
      bg="workbench.control"
      color="white"
      px={{ base: 4, md: 6 }}
      py={4}
      borderRadius="14px"
    >
      <Flex
        gap={5}
        align={{ base: "stretch", md: "center" }}
        direction={{ base: "column", md: "row" }}
      >
        <VStack align="stretch" spacing={1} minW={{ md: "250px" }}>
          <HStack>
            <SignalBadge status={status} />
          </HStack>
          <ProgressCircuit stage={stage} progress={progress} status={status} />
        </VStack>
        <Flex flex={1} wrap="wrap" gap={2}>
          {metrics.map((metric) => (
            <Box
              key={metric.label}
              flex={{ base: "1 1 calc(50% - 4px)", sm: "1 1 112px" }}
              minW={0}
              px={3}
              py={2}
              bg="whiteAlpha.100"
              border="1px solid"
              borderColor="whiteAlpha.200"
              borderRadius="10px"
            >
              <Text
                fontSize="10px"
                color="whiteAlpha.600"
                letterSpacing="0.08em"
              >
                {metric.label}
              </Text>
              <Text
                mt={0.5}
                fontFamily="mono"
                fontSize="sm"
                fontWeight="700"
                sx={{ fontVariantNumeric: "tabular-nums" }}
              >
                {metric.value}
              </Text>
            </Box>
          ))}
        </Flex>
      </Flex>
    </Box>
  );
}

export { default as AdaptiveProjectTitle } from "./adaptive-title";
