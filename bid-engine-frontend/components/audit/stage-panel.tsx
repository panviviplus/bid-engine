"use client";

/* Hallmark · component: stage-panel (bid-audit V2) · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · loading(单阶段重跑) · disabled · 空态
 * 设计要点：阶段以自适应小卡片栅格呈现（大屏多列、窄屏单列）；
 * 卡片正文只给"结论 + 工作量描述"，进度用底边细条承载，避免分子/分母读数造成误解。
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */

import React, { useState } from "react";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Flex,
  Icon,
  IconButton,
  Text,
  Tooltip,
} from "@chakra-ui/react";
import { useReducedMotion } from "framer-motion";
import {
  FiAlertTriangle,
  FiCheckCircle,
  FiChevronDown,
  FiChevronUp,
  FiCircle,
  FiClock,
  FiLoader,
  FiRefreshCw,
  FiSkipForward,
  FiSlash,
} from "react-icons/fi";
import { StageInfo } from "./types";

interface Props {
  stages: StageInfo[];
  projectStatus: string;
  runCount: number;
  startedAt?: string | null;
  finishedAt?: string | null;
  loadingStage?: string | null;
  onRetryStage: (stage: string) => void;
  onCancel: () => void;
}

type Level = "collapsed" | "partial" | "all";

const LEVELS: { key: Level; label: string; hint: string }[] = [
  { key: "collapsed", label: "收起", hint: "只保留标题行" },
  { key: "partial", label: "精简", hint: "只看前 4 个阶段" },
  { key: "all", label: "全部", hint: "显示全部阶段" },
];

// 各阶段"工作量"描述：只说本轮覆盖的范围，不渲染分子/分母，
// 且措辞与状态无关（避免出现"已完成 · 43 处待定位"这类自相矛盾的读数）。
const STAGE_UNIT: Record<string, string> = {
  tender_parse: "份招标文件",
  bid_parse: "份投标文件",
  checklist_build: "条审核清单",
  evidence_match: "个检查项（证据检索范围）",
  verdict: "个检查项（判定范围）",
  format_scan: "项版式检查",
  scoring: "个评分项（对标范围）",
};

function stageIcon(status: string) {
  switch (status) {
    case "succeeded":
      return { icon: FiCheckCircle, color: "success.500" };
    case "failed":
      return { icon: FiAlertTriangle, color: "error.500" };
    case "running":
      return { icon: FiLoader, color: "primary.500" };
    case "skipped":
      return { icon: FiSkipForward, color: "neutral.400" };
    default:
      return { icon: FiCircle, color: "neutral.300" };
  }
}

function statusLabel(status: string) {
  switch (status) {
    case "succeeded":
      return "已完成";
    case "failed":
      return "失败";
    case "running":
      return "执行中";
    case "skipped":
      return "已跳过";
    default:
      return "待执行";
  }
}

function statusColors(status: string) {
  switch (status) {
    case "succeeded":
      return { bg: "success.50", color: "success.600", track: "success.100", fill: "success.400" };
    case "failed":
      return { bg: "error.50", color: "error.600", track: "error.100", fill: "error.400" };
    case "running":
      return { bg: "primary.50", color: "primary.700", track: "primary.100", fill: "primary.500" };
    default:
      return { bg: "neutral.100", color: "neutral.500", track: "neutral.100", fill: "neutral.300" };
  }
}

function formatDuration(startedAt?: string | null, finishedAt?: string | null): string {
  if (!startedAt) return "";
  const start = new Date(startedAt).getTime();
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
  const seconds = Math.max(Math.round((end - start) / 1000), 0);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m${seconds % 60}s`;
  return `${Math.floor(minutes / 60)}h${minutes % 60}m`;
}

/** 阶段小卡片：结论 + 工作量描述 + 底边进度条 */
function StageCard({
  stage,
  busy,
  onRetry,
}: {
  stage: StageInfo;
  busy: boolean;
  onRetry: (stage: string) => void;
}) {
  const meta = stageIcon(stage.status);
  const colors = statusColors(stage.status);
  const unit = STAGE_UNIT[stage.stage] || "项";
  const workload = stage.total > 0 ? `${stage.total} ${unit}` : "";
  const duration = formatDuration(stage.started_at, stage.finished_at);
  const statusText = `${statusLabel(stage.status)}${
    stage.status === "running" && stage.progress > 0 ? ` ${stage.progress}%` : ""
  }`;
  const metaLine = [
    statusText,
    stage.status === "running" && stage.hint !== "执行中" ? stage.hint : "",
    workload,
    stage.attempts > 1 ? `第 ${stage.attempts} 次` : "",
    duration,
  ]
    .filter(Boolean)
    .join(" · ");

  // 底边进度条：已完成=满格、执行中=实际进度、失败=停在断点（至少留一小段表示中断）
  let fillPercent = 0;
  if (stage.status === "succeeded" || stage.status === "skipped") fillPercent = 100;
  else if (stage.status === "running") fillPercent = Math.max(stage.progress, 4);
  else if (stage.status === "failed") fillPercent = Math.max(stage.progress, 6);

  return (
    <Box
      position="relative"
      overflow="hidden"
      minW={0}
      px={3}
      pt={2.5}
      pb={3.5}
      borderRadius="12px"
      bg={stage.status === "pending" ? "neutral.50" : "white"}
      border="1px solid"
      borderColor={stage.status === "failed" ? "error.200" : "neutral.200"}
      transition="border-color 0.18s ease, box-shadow 0.18s ease"
      _hover={{ borderColor: stage.status === "failed" ? "error.300" : "gold.300", boxShadow: "sm" }}
    >
      <Flex align="center" gap={2} minW={0}>
        <Icon as={meta.icon} color={meta.color} boxSize={3.5} flexShrink={0} />
        <Text
          fontSize="12px"
          fontWeight={700}
          color="neutral.700"
          noOfLines={1}
          minW={0}
          flex="1 1 auto"
          title={`${stage.order}. ${stage.label}`}
        >
          {stage.order}. {stage.label}
        </Text>
        <Badge
          variant="subtle"
          bg={colors.bg}
          color={colors.color}
          fontSize="9px"
          borderRadius="full"
          px={1.5}
          flexShrink={0}
        >
          {statusLabel(stage.status)}
        </Badge>
      </Flex>

      <Flex mt={1} align="center" gap={1.5} minW={0}>
        <Text
          fontSize="10px"
          color={stage.status === "failed" ? "error.500" : "neutral.400"}
          noOfLines={1}
          minW={0}
          flex="1 1 auto"
          title={metaLine}
        >
          {metaLine}
        </Text>
        {stage.retryable && (
          <Tooltip
            label={`从${stage.label}重跑（该阶段及后续阶段重新执行）`}
            hasArrow
            openDelay={200}
          >
            <IconButton
              aria-label={`重跑${stage.label}阶段`}
              icon={<FiRefreshCw size={11} />}
              size="xs"
              variant="ghost"
              color="neutral.500"
              _hover={{ color: "primary.600", bg: "primary.50" }}
              isLoading={busy}
              onClick={() => onRetry(stage.stage)}
              flexShrink={0}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
            />
          </Tooltip>
        )}
      </Flex>

      {stage.last_error && (
        <Text
          mt={1}
          fontSize="10px"
          color="error.500"
          noOfLines={2}
          title={stage.last_error}
        >
          {stage.last_error}
        </Text>
      )}

      {/* 底边进度条：不占正文空间 */}
      <Box
        position="absolute"
        left={0}
        right={0}
        bottom={0}
        h="3px"
        bg={colors.track}
      >
        <Box
          h="100%"
          w={`${fillPercent}%`}
          bg={colors.fill}
          borderRadius="full"
          transition="width 0.4s cubic-bezier(0.16, 1, 0.3, 1)"
        />
      </Box>
    </Box>
  );
}

/** 档位胶囊开关：三档滑动（收起 / 精简 / 全部） */
function LevelSwitch({
  level,
  onChange,
}: {
  level: Level;
  onChange: (level: Level) => void;
}) {
  const reduceMotion = useReducedMotion();
  const index = LEVELS.findIndex((l) => l.key === level);
  return (
    <Flex
      role="radiogroup"
      aria-label="阶段展示档位"
      position="relative"
      bg="neutral.100"
      borderRadius="full"
      p="2px"
      flexShrink={0}
    >
      <Box
        aria-hidden
        position="absolute"
        top="2px"
        bottom="2px"
        left="2px"
        w="calc((100% - 4px) / 3)"
        bg="workbench.paper"
        borderRadius="full"
        boxShadow="sm"
        transform={`translateX(${index * 100}%)`}
        transition={reduceMotion ? undefined : "transform 0.22s cubic-bezier(0.65, 0, 0.35, 1)"}
      />
      {LEVELS.map((item) => {
        const active = item.key === level;
        return (
          <Tooltip key={item.key} label={item.hint} hasArrow openDelay={200}>
            <Box
              as="button"
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => onChange(item.key)}
              position="relative"
              zIndex={1}
              px={2.5}
              py="2px"
              fontSize="11px"
              fontWeight={active ? 700 : 500}
              color={active ? "workbench.control" : "neutral.500"}
              cursor="pointer"
              transition="color 0.18s ease"
              _hover={{ color: active ? "workbench.control" : "neutral.700" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
                borderRadius: "full",
              }}
            >
              {item.label}
            </Box>
          </Tooltip>
        );
      })}
    </Flex>
  );
}

export default function StagePanel({
  stages,
  projectStatus,
  runCount,
  startedAt,
  finishedAt,
  loadingStage,
  onRetryStage,
  onCancel,
}: Props) {
  const [level, setLevel] = useState<Level>("partial");
  const running = projectStatus === "running";
  const collapsed = level === "collapsed";
  const visible = level === "all" ? stages : stages.slice(0, 4);

  const succeededCount = stages.filter((st) => st.status === "succeeded").length;
  const focusStage =
    stages.find((st) => st.status === "running") ||
    stages.find((st) => st.status === "failed") ||
    stages.find((st) => st.status === "pending");
  const summary = [
    `第 ${runCount || 1} 次执行`,
    `已完成 ${succeededCount}/${stages.length} 个阶段`,
    focusStage ? `当前：${focusStage.label}（${statusLabel(focusStage.status)}）` : "",
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <Box
      bg="workbench.paper"
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="14px"
      px={{ base: 3, md: 4 }}
      py={3}
    >
      <Flex align="center" gap={2} flexWrap="wrap" rowGap={2}>
        <IconButton
          aria-label={collapsed ? "展开任务阶段" : "收起任务阶段"}
          icon={collapsed ? <FiChevronDown /> : <FiChevronUp />}
          size="xs"
          variant="ghost"
          onClick={() => setLevel(collapsed ? "partial" : "collapsed")}
          _focusVisible={{
            outline: "2px solid",
            outlineColor: "gold.400",
            outlineOffset: "2px",
          }}
        />
        <Icon as={FiClock} color="workbench.muted" />
        <Text fontSize="sm" fontWeight={700} color="workbench.text">
          任务阶段
        </Text>
        <Text fontSize="10px" color="neutral.400" noOfLines={1} minW={0} title={summary}>
          {summary}
          {startedAt && !collapsed ? ` · 开始 ${new Date(startedAt).toLocaleString()}` : ""}
          {finishedAt && !collapsed ? ` · 结束 ${new Date(finishedAt).toLocaleString()}` : ""}
        </Text>
        <Box flex={1} />
        <LevelSwitch level={level} onChange={setLevel} />
        {running && (
          <Button
            size="xs"
            variant="outline"
            colorScheme="error"
            leftIcon={<FiSlash size={11} />}
            onClick={onCancel}
            flexShrink={0}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.400",
              outlineOffset: "2px",
            }}
          >
            取消任务
          </Button>
        )}
      </Flex>

      <Collapse in={!collapsed} animateOpacity>
        <Box
          mt={3}
          display="grid"
          gridTemplateColumns={{
            base: "1fr",
            sm: "repeat(auto-fill, minmax(min(100%, 200px), 1fr))",
            xl: "repeat(auto-fill, minmax(min(100%, 170px), 1fr))",
          }}
          gap={2.5}
        >
          {visible.map((stage) => (
            <StageCard
              key={stage.stage}
              stage={stage}
              busy={loadingStage === stage.stage}
              onRetry={onRetryStage}
            />
          ))}
        </Box>
      </Collapse>
    </Box>
  );
}
