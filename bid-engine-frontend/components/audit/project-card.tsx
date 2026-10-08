"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · component: project-card (bid-audit) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default · hover · focus-visible · active (delete); progress bar hover · focus · active · pinned
 */

import React, { Fragment, useEffect, useRef, useState } from "react";
import {
  Box,
  Flex,
  Text,
  Badge,
  Button,
  IconButton,
  Tooltip,
  keyframes,
} from "@chakra-ui/react";
import { useReducedMotion } from "framer-motion";
import { FiFileText, FiFolder, FiTrash2, FiRefreshCw } from "react-icons/fi";
import { format } from "date-fns";
import { useRouter } from "next/navigation";
import {
  AdaptiveProjectTitle,
  DataSurface,
} from "@/components/analysis/bid-analysis-v3/workspace";
import {
  ReviewProject,
  StageStep,
  STAGE_LABELS,
  DIMENSION_META,
  buildStageSteps,
  getRetryableStage,
} from "./types";

const blink = keyframes`
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
`;

// 刚创建（0%）时空轨道的横向扫光
const shimmer = keyframes`
  0% { transform: translateX(-150%); }
  100% { transform: translateX(450%); }
`;

// 进度条前缘光点的轻微呼吸（克制，不闪烁）
const breathe = keyframes`
  0%, 100% { transform: translateY(-50%) scale(1); opacity: 1; }
  50% { transform: translateY(-50%) scale(0.82); opacity: 0.68; }
`;

function statusMeta(status: string, errorItems: number, warningItems: number) {
  if (status === "running") {
    return {
      label: "审核中",
      color: "info.600",
      bg: "info.50",
      border: "info.200",
    };
  }
  if (status === "failed") {
    return {
      label: "审核失败",
      color: "error.600",
      bg: "error.50",
      border: "error.200",
    };
  }
  if (status === "cancelled") {
    return {
      label: "已取消",
      color: "neutral.600",
      bg: "neutral.100",
      border: "neutral.300",
    };
  }
  if (errorItems > 0) {
    return {
      label: "存在告警",
      color: "error.600",
      bg: "error.50",
      border: "error.200",
    };
  }
  if (warningItems > 0) {
    return {
      label: "需关注",
      color: "warning.600",
      bg: "warning.50",
      border: "warning.200",
    };
  }
  return {
    label: "已通过",
    color: "success.600",
    bg: "success.50",
    border: "success.200",
  };
}

// 生命周期气泡：阶段圆点 + 连接线 + 箭头。
// 审核阶段较多（8 个），因此：节点下方不再放固定文案（避免挤压成省略号），
// 改为悬停节点弹出阶段名与状态；节点条横向可滑动，气泡宽度不超出卡片。
function AuditStageBubble({
  steps,
  progress,
  onHoverIn,
  onHoverOut,
}: {
  steps: StageStep[];
  progress?: number;
  onHoverIn: () => void;
  onHoverOut: () => void;
}) {
  const reduceMotion = useReducedMotion();
  const scrollRef = useRef<HTMLDivElement>(null);
  const nodeRefs = useRef<Record<string, HTMLDivElement | null>>({});

  // 定位当前节点（执行中；无则取最后一个已完成），用于横向自动滚动到可视区
  const focusStage = (() => {
    const running = steps.find((st) => st.status === "running");
    if (running) return running.stage;
    for (let i = steps.length - 1; i >= 0; i -= 1) {
      if (steps[i].status === "succeeded") return steps[i].stage;
    }
    return "";
  })();
  const focusLabel = steps.find((st) => st.stage === focusStage)?.label || "";

  useEffect(() => {
    const container = scrollRef.current;
    const node = focusStage ? nodeRefs.current[focusStage] : null;
    if (!container || !node) return;
    const target =
      node.offsetLeft - container.clientWidth / 2 + node.clientWidth / 2;
    container.scrollTo({
      left: Math.max(target, 0),
      behavior: reduceMotion ? ("auto" as ScrollBehavior) : "smooth",
    });
  }, [focusStage, reduceMotion]);

  return (
    <Box
      position="absolute"
      bottom="calc(100% + 10px)"
      left={0}
      width="100%"
      maxW="100%"
      zIndex={70}
      bg="white"
      border="1px solid"
      borderColor="neutral.200"
      borderRadius="xl"
      boxShadow="0 12px 32px rgba(16,24,40,0.16), 0 2px 8px rgba(16,24,40,0.08)"
      px={3}
      py={2.5}
      onMouseEnter={onHoverIn}
      onMouseLeave={onHoverOut}
    >
      {/* 小箭头 */}
      <Box
        position="absolute"
        bottom="-6px"
        left="22px"
        w={3}
        h={3}
        bg="white"
        borderRight="1px solid"
        borderBottom="1px solid"
        borderColor="neutral.200"
        transform="rotate(45deg)"
      />
      <Flex align="center" justify="space-between" mb={2.5}>
        <Text
          fontSize="xs"
          fontWeight="semibold"
          color="neutral.600"
          noOfLines={1}
          minW={0}
        >
          审核进度
          {focusLabel && (
            <Text as="span" fontSize="xs" fontWeight={500} color="neutral.400">
              {" · "}
              {focusLabel}
            </Text>
          )}
        </Text>
        <Text fontSize="xs" color="neutral.400">
          {progress || 0}% · 点击可钉住
        </Text>
      </Flex>
      <Flex
        ref={scrollRef}
        position="relative"
        align="flex-start"
        overflowX="auto"
        overflowY="hidden"
        pb={1}
        sx={{
          scrollbarWidth: "thin",
          "&::-webkit-scrollbar": { height: "4px" },
          "&::-webkit-scrollbar-thumb": {
            background: "var(--chakra-colors-neutral-300)",
            borderRadius: "full",
          },
          "&::-webkit-scrollbar-track": { background: "transparent" },
        }}
      >
        {steps.map((st, idx) => {
          const isDone = st.status === "succeeded";
          const isFailed = st.status === "failed";
          const isCurrent = st.status === "running";
          const prevDone = idx > 0 && steps[idx - 1].status === "succeeded";
          const dotBg = isDone
            ? "success.500"
            : isFailed
              ? "error.500"
              : isCurrent
                ? "info.500"
                : "neutral.200";
          const dotColor =
            isDone || isFailed || isCurrent ? "workbench.paper" : "neutral.400";
          const icon = isDone ? "✓" : isFailed ? "✗" : isCurrent ? "…" : "○";
          const statusText = isDone
            ? "已完成"
            : isFailed
              ? "失败"
              : isCurrent
                ? `执行中${progress ? ` ${progress}%` : ""}`
                : st.status === "skipped"
                  ? "已跳过"
                  : "待执行";
          return (
            <Fragment key={st.stage}>
              {idx > 0 && (
                <Box
                  flex="1 1 auto"
                  minW="12px"
                  maxW="30px"
                  h="2px"
                  bg={prevDone ? "success.500" : "neutral.200"}
                  mt="10px"
                />
              )}
              <Tooltip
                label={`${st.label} · ${statusText}`}
                placement="top"
                hasArrow
                openDelay={160}
                closeOnClick={false}
              >
                <Flex
                  ref={(el: HTMLDivElement | null) => {
                    nodeRefs.current[st.stage] = el;
                  }}
                  flex="none"
                  w="22px"
                  h="22px"
                  borderRadius="full"
                  alignItems="center"
                  justifyContent="center"
                  fontSize="11px"
                  fontWeight="bold"
                  color={dotColor}
                  bg={dotBg}
                  cursor="default"
                  boxShadow={
                    isCurrent ? "0 0 0 4px rgba(49,130,206,0.15)" : undefined
                  }
                  animation={
                    isCurrent && !reduceMotion
                      ? `${blink} 1.4s ease-in-out infinite`
                      : undefined
                  }
                >
                  {icon}
                </Flex>
              </Tooltip>
            </Fragment>
          );
        })}
      </Flex>
      {steps.length >= 6 && (
        <Text mt={1} fontSize="10px" color="neutral.400" noOfLines={1}>
          共 {steps.length} 个阶段 · 左右滑动查看 · 悬停节点看阶段名
        </Text>
      )}
    </Box>
  );
}

// 检查项统计：横向胶囊标签（右侧右对齐横排，窄卡自动右对齐换行）
function ReviewStatsPills({
  errorItems,
  warningItems,
  passedItems,
  scoringTotal,
  scoringMax,
}: {
  errorItems: number;
  warningItems: number;
  passedItems: number;
  scoringTotal: number;
  scoringMax: number;
}) {
  const pills = [
    {
      key: "error",
      color: "error.600",
      bg: "error.50",
      value: String(errorItems),
      label: "高风险",
    },
    {
      key: "warning",
      color: "warning.600",
      bg: "warning.50",
      value: String(warningItems),
      label: "需整改",
    },
    {
      key: "success",
      color: "success.600",
      bg: "success.50",
      value: String(passedItems),
      label: "通过",
    },
    // 竞争力得分只在有评分办法时出现，避免空标签占位
    ...(scoringMax > 0
      ? [
          {
            key: "scoring",
            color: "gold.700",
            bg: "gold.50",
            value: `${Math.round(scoringTotal)}/${Math.round(scoringMax)}`,
            label: "竞争力",
          },
        ]
      : []),
  ];
  return (
    <Flex mt={2} align="center" gap={1.5} flexWrap="wrap" rowGap={1.5}>
      {pills.map((p) => (
        <Flex
          key={p.key}
          align="center"
          gap={1}
          bg={p.bg}
          color={p.color}
          borderRadius="full"
          px={2.5}
          py={0.5}
        >
          <Box w={1.5} h={1.5} borderRadius="full" bg={p.color} />
          <Text
            fontSize="11px"
            fontWeight="700"
            style={{ fontVariantNumeric: "tabular-nums" }}
          >
            {p.value}
          </Text>
          <Text fontSize="10px" fontWeight="500">
            {p.label}
          </Text>
        </Flex>
      ))}
    </Flex>
  );
}

export default function ProjectCard({
  project,
  onDelete,
  onRetry,
}: {
  project: ReviewProject;
  // eslint-disable-next-line no-unused-vars
  onDelete?: (p: ReviewProject) => void;
  // eslint-disable-next-line no-unused-vars
  onRetry?: (p: ReviewProject) => void;
}) {
  const router = useRouter();
  const reduceMotion = useReducedMotion();
  const meta = statusMeta(
    project.status,
    project.error_items,
    project.warning_items,
  );
  const steps = buildStageSteps(
    project.stage_status,
    project.progress,
    project.status,
  );
  const retryableStage = getRetryableStage(project);
  const failedStage = retryableStage;

  // 生命周期气泡：进度条 hover 打开 + 点击钉住（对齐招标解析卡片交互）
  const showLifecycle =
    project.status === "running" || project.status === "failed";
  const stageBlockRef = useRef<HTMLDivElement>(null);
  const stageHideTimer = useRef<number | null>(null);
  const [stageOpen, setStageOpen] = useState(false);
  const [pinned, setPinned] = useState(false);
  const showPopover = stageOpen || pinned;
  // 气泡打开时（悬浮/钉住）将整张卡片提到最上层，避免被后续卡片遮挡
  const cardTopZ = showPopover ? 60 : undefined;

  // 200ms 停留延迟唤起（仅鼠标 hover）；焦点/钉住即时显示；离开 150ms 后关闭（保留桥接）
  const stageShowTimer = useRef<number | null>(null);
  const stageShow = () => {
    if (stageHideTimer.current) window.clearTimeout(stageHideTimer.current);
    if (stageShowTimer.current) window.clearTimeout(stageShowTimer.current);
    stageShowTimer.current = window.setTimeout(() => setStageOpen(true), 200);
  };
  const stageShowNow = () => {
    if (stageHideTimer.current) window.clearTimeout(stageHideTimer.current);
    if (stageShowTimer.current) window.clearTimeout(stageShowTimer.current);
    setStageOpen(true);
  };
  const stageHide = () => {
    if (stageShowTimer.current) window.clearTimeout(stageShowTimer.current);
    if (stageHideTimer.current) window.clearTimeout(stageHideTimer.current);
    stageHideTimer.current = window.setTimeout(() => {
      if (!pinned) setStageOpen(false);
    }, 150);
  };
  const togglePin = (e: React.MouseEvent | React.KeyboardEvent) => {
    e.preventDefault();
    e.stopPropagation(); // 避免冒泡触发卡片跳转
    setPinned((v) => !v);
    setStageOpen(true);
  };
  const onProgressKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") togglePin(e);
  };

  useEffect(() => {
    if (!pinned) return;
    const onDocDown = (e: MouseEvent) => {
      if (
        stageBlockRef.current &&
        !stageBlockRef.current.contains(e.target as Node)
      ) {
        setPinned(false);
        setStageOpen(false);
      }
    };
    document.addEventListener("mousedown", onDocDown);
    return () => document.removeEventListener("mousedown", onDocDown);
  }, [pinned]);

  useEffect(
    () => () => {
      if (stageHideTimer.current) window.clearTimeout(stageHideTimer.current);
      if (stageShowTimer.current) window.clearTimeout(stageShowTimer.current);
    },
    [],
  );

  // 进度条填充与常驻文案
  const progress = project.progress || 0;
  const isFailed = project.status === "failed";
  let caption = "";
  // 阶段名规整：去掉装饰引号；汇总完成阶段名做动词化（汇总中 / 汇总失败）
  const stageVerb = (s: string) => (s.endsWith("完成") ? s.slice(0, -2) : s);
  if (isFailed) {
    const label = failedStage
      ? stageVerb(STAGE_LABELS[failedStage] || failedStage)
      : "";
    caption = label ? `${label}失败` : "审核失败";
  } else if (project.status === "running") {
    const currentStep = steps.find((s) => s.status === "running");
    const label = currentStep
      ? stageVerb(STAGE_LABELS[currentStep.stage] || currentStep.stage)
      : "";
    const roundPrefix = (project.run_count || 0) > 1 ? `第${project.run_count}次 · ` : "";
    caption = progress === 0 ? `${roundPrefix}准备中` : `${roundPrefix}${label || "审核"}中`;
  }
  const fillGradient = isFailed
    ? "linear-gradient(90deg, error.400, error.500)"
    : "linear-gradient(90deg, primary.500, primary.400)";
  const glowColor = isFailed
    ? "rgba(251, 113, 133, 0.45)"
    : "rgba(64, 110, 195, 0.4)";

  const handleClick = () => {
    router.push(`/bid-audit/${project.id}`);
  };

  return (
    <DataSurface
      as="article"
      onClick={handleClick}
      cursor="pointer"
      position="relative"
      zIndex={cardTopZ}
      // 高度随内容走：完成后不再保留进度区块，卡片自动收紧，不留大片空白
      borderColor={meta.border}
      transition="border-color 0.22s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
      _hover={{
        borderColor: project.status === "running" ? "info.300" : "neutral.300",
      }}
      _active={{ transform: "translateY(1px)" }}
      _focusVisible={{
        outline: "2px solid",
        outlineColor: "gold.400",
        outlineOffset: "2px",
      }}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleClick();
        }
      }}
    >
      <Box p={5} role="group">
        {/* 顶行：项目名（删除按钮移至右下角） */}
        <Box minW={0}>
          <AdaptiveProjectTitle name={project.name} />
        </Box>

        {/* 第二行：身份徽章（创建方式 + 状态）独占一行，不与指标抢空间 */}
        <Flex mt={2.5} gap={2} align="center" flexWrap="wrap" minW={0}>
          <Badge
            variant="subtle"
            colorScheme="gray"
            fontSize="10px"
            borderRadius="full"
            px={2}
          >
            {project.create_type === "gen" ? "从标书生成" : "上传创建"}
          </Badge>
          <Badge
            variant="subtle"
            bg={meta.bg}
            color={meta.color}
            fontSize="10px"
            borderRadius="full"
            px={2}
          >
            {meta.label}
          </Badge>
        </Flex>

        {/* 第三行：结论指标（高风险 / 需整改 / 通过 / 竞争力） */}
        <ReviewStatsPills
          errorItems={project.error_items}
          warningItems={project.warning_items}
          passedItems={project.passed_items}
          scoringTotal={project.scoring_total}
          scoringMax={project.scoring_max}
        />

        {/* 进度区块（running/failed）：仅进度条行触发 hover/点击钉住气泡；下方常驻文案不在触发区内 */}
        {showLifecycle && (
          <Box
            mt={4}
            p={3}
            borderRadius="11px"
            bg="workbench.control"
            color="workbench.paper"
          >
            <Box
              ref={stageBlockRef}
              role="button"
              tabIndex={0}
              aria-haspopup="dialog"
              aria-expanded={showPopover}
              position="relative"
              px={1}
              py={1}
              borderRadius="md"
              cursor="pointer"
              onClick={togglePin}
              onMouseEnter={stageShow}
              onMouseLeave={stageHide}
              onFocus={stageShowNow}
              onBlur={stageHide}
              onKeyDown={onProgressKeyDown}
              _hover={{ bg: "whiteAlpha.100" }}
              _focusVisible={{
                boxShadow: "0 0 0 3px var(--chakra-colors-primary-300)",
              }}
            >
              <Flex align="center" gap={2.5}>
                <Box flex={1} position="relative" h="8px">
                  {/* 轨道 */}
                  <Box
                    position="absolute"
                    inset={0}
                    borderRadius="full"
                    bg="neutral.100"
                    overflow="hidden"
                  >
                    {/* 0% 扫光（仅运行中准备态） */}
                    {progress === 0 && !isFailed && (
                      <Box
                        position="absolute"
                        top={0}
                        bottom={0}
                        w="40%"
                        bgGradient="linear(to-r, transparent, rgba(255,255,255,0.95), transparent)"
                        animation={
                          reduceMotion
                            ? undefined
                            : `${shimmer} 1.8s ease-in-out infinite`
                        }
                      />
                    )}
                  </Box>
                  {/* 填充 */}
                  <Box
                    position="absolute"
                    top={0}
                    bottom={0}
                    left={0}
                    width={`${Math.min(100, Math.max(0, progress))}%`}
                    borderRadius="full"
                    bgGradient={fillGradient}
                    transition={
                      reduceMotion
                        ? "opacity 0.15s ease"
                        : "width 0.6s cubic-bezier(0.16, 1, 0.3, 1)"
                    }
                  />
                  {/* 前缘光点 */}
                  {(progress > 0 || isFailed) && (
                    <Box
                      position="absolute"
                      top="50%"
                      transform="translateY(-50%)"
                      left={progress === 0 ? 0 : `calc(${progress}% - 4px)`}
                      w="8px"
                      h="8px"
                      borderRadius="full"
                      bg="white"
                      boxShadow={`0 0 0 3px ${glowColor}, 0 0 8px ${glowColor}`}
                      animation={
                        reduceMotion
                          ? undefined
                          : `${breathe} 1.6s ease-in-out infinite`
                      }
                    />
                  )}
                </Box>
                <Text
                  fontSize="xs"
                  color="whiteAlpha.700"
                  style={{ fontVariantNumeric: "tabular-nums" }}
                  flex="none"
                >
                  {progress}%
                </Text>
              </Flex>

              {showPopover && (
                <AuditStageBubble
                  steps={steps}
                  progress={project.progress}
                  onHoverIn={stageShowNow}
                  onHoverOut={stageHide}
                />
              )}
            </Box>
            <Text
              mt={1}
              ml={1}
              fontSize="xs"
              fontWeight={isFailed ? 600 : 500}
              color={isFailed ? "error.200" : "whiteAlpha.700"}
            >
              {caption}
            </Text>
          </Box>
        )}

        {/* 底部：文件计数 + 时间（左） / 删除（右下角） */}
        <Flex
          mt={3}
          pt={3}
          borderTop="1px dashed"
          borderColor="neutral.200"
          justify="space-between"
          align="center"
          gap={2}
          flexWrap="wrap"
        >
          <Flex
            gap={3}
            color="neutral.500"
            fontSize="11px"
            align="center"
            flexWrap="wrap"
          >
            <Flex align="center" gap={1}>
              <FiFolder size={12} />
              <Text>招标×{project.tender_file_count}</Text>
            </Flex>
            <Flex align="center" gap={1}>
              <FiFileText size={12} />
              <Text>投标×{project.bid_file_count}</Text>
            </Flex>
            {project.source_bid_gen_project_id > 0 && (
              <Text color="neutral.400">
                关联标书 #{project.source_bid_gen_project_id}
              </Text>
            )}
            <Text color="neutral.400">
              {format(new Date(project.created_at), "yyyy-MM-dd HH:mm")}
            </Text>
          </Flex>
          <Flex align="center" gap={1}>
            {isFailed && retryableStage && onRetry && (
              <Button
                size="xs"
                variant="outline"
                colorScheme="error"
                leftIcon={<FiRefreshCw size={11} />}
                onClick={(e) => {
                  e.stopPropagation();
                  onRetry(project);
                }}
                _focusVisible={{
                  boxShadow: "0 0 0 3px var(--chakra-colors-error-300)",
                }}
              >
                重试
              </Button>
            )}
            {onDelete && (
              <Tooltip label="删除">
                <IconButton
                  aria-label="删除项目"
                  icon={<FiTrash2 />}
                  size="sm"
                  variant="ghost"
                  color="error.500"
                  _hover={{ color: "error.600", bg: "error.50" }}
                  _active={{ bg: "error.100", transform: "scale(0.92)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 3px var(--chakra-colors-error-300)",
                  }}
                  transition="background-color 0.16s cubic-bezier(0.16, 1, 0.3, 1), color 0.16s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
                  onClick={(e) => {
                    e.stopPropagation();
                    onDelete(project);
                  }}
                />
              </Tooltip>
            )}
          </Flex>
        </Flex>
      </Box>
    </DataSurface>
  );
}
