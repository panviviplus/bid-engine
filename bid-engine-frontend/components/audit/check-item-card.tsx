"use client";

/* Hallmark · component: check-item-card (bid-audit V2) · genre: modern-minimal · theme: BidEngine deep-sea/gold
 * states: default · hover · focus-visible · active · loading · empty-evidence
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */

/* eslint-disable no-nested-ternary */

import React, { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Divider,
  Flex,
  IconButton,
  Text,
  Textarea,
  Tooltip,
  useToast,
} from "@chakra-ui/react";
import { useReducedMotion } from "framer-motion";
import {
  FiCheck,
  FiChevronDown,
  FiChevronUp,
  FiClock,
  FiFilePlus,
  FiRefreshCw,
  FiSearch,
  FiTrash2,
  FiX,
} from "react-icons/fi";
import {
  ChecklistItem,
  DIMENSION_META,
  SOURCE_LABELS,
  REVIEW_STATUS_META,
  SEVERITY_META,
  findingMeta,
  toTraceRefs,
  TraceRef,
} from "./types";

interface Props {
  item: ChecklistItem;
  highlighted?: boolean;
  onTrace: (refs: TraceRef[], side: "tender" | "bid") => void;
  onUpdate: (
    itemId: number,
    reviewStatus: string,
    note: string,
  ) => Promise<void>;
  onRecheck: (itemId: number) => Promise<void>;
  onDelete: (itemId: number) => void;
  onSaveRule: (itemId: number) => Promise<void>;
}

// 人工确认状态：三档滑动开关（语义与配色一一对应，避免"黑白按钮"）
const REVIEW_OPTIONS = [
  {
    key: "pending",
    label: "待处理",
    hint: "尚未人工复核，保持待处理",
    icon: FiClock,
    pillBg: "workbench.paper",
    pillFg: "neutral.700",
    activeColor: "neutral.700",
    activeBg: "neutral.100",
  },
  {
    key: "confirmed",
    label: "已确认",
    hint: "确认该项结论无异议",
    icon: FiCheck,
    pillBg: "success.500",
    pillFg: "white",
    activeColor: "success.600",
    activeBg: "success.50",
  },
  {
    key: "rejected",
    label: "驳回",
    hint: "人工不认可该判定（需整改）",
    icon: FiX,
    pillBg: "error.500",
    pillFg: "white",
    activeColor: "error.600",
    activeBg: "error.50",
  },
];

/** 人工确认三档开关：滑动胶囊，选中档位自带语义色 */
function ReviewStatusSwitch({
  value,
  disabled,
  onChange,
}: {
  value: string;
  disabled?: boolean;
  // eslint-disable-next-line no-unused-vars
  onChange: (status: string) => void;
}) {
  const reduceMotion = useReducedMotion();
  const index = Math.max(
    REVIEW_OPTIONS.findIndex((o) => o.key === value),
    0,
  );
  const active = REVIEW_OPTIONS[index];
  return (
    <Flex
      role="radiogroup"
      aria-label="人工确认状态"
      position="relative"
      bg="neutral.100"
      borderRadius="full"
      p="2px"
      opacity={disabled ? 0.7 : 1}
      pointerEvents={disabled ? "none" : undefined}
    >
      <Box
        aria-hidden
        position="absolute"
        top="2px"
        bottom="2px"
        left="2px"
        w="calc((100% - 4px) / 3)"
        bg={active.pillBg}
        borderRadius="full"
        boxShadow="sm"
        transform={`translateX(${index * 100}%)`}
        transition={
          reduceMotion
            ? undefined
            : "transform 0.22s cubic-bezier(0.65, 0, 0.35, 1), background-color 0.18s ease"
        }
      />
      {REVIEW_OPTIONS.map((option) => {
        const isActive = option.key === value;
        const OptionIcon = option.icon;
        return (
          <Tooltip
            key={option.key}
            label={option.hint}
            hasArrow
            openDelay={200}
          >
            <Flex
              as="button"
              type="button"
              role="radio"
              aria-checked={isActive}
              onClick={() => onChange(option.key)}
              position="relative"
              zIndex={1}
              align="center"
              gap={1}
              px={3}
              py="3px"
              fontSize="11px"
              fontWeight={isActive ? 700 : 500}
              color={isActive ? option.pillFg : "neutral.600"}
              cursor="pointer"
              transition="color 0.18s ease"
              _hover={{ color: isActive ? option.pillFg : "neutral.800" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
                borderRadius: "full",
              }}
            >
              <Box as={OptionIcon} boxSize={3} />
              {option.label}
            </Flex>
          </Tooltip>
        );
      })}
    </Flex>
  );
}

export default function CheckItemCard({
  item,
  highlighted,
  onTrace,
  onUpdate,
  onRecheck,
  onDelete,
  onSaveRule,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const [note, setNote] = useState(item.review_note || "");
  const [updating, setUpdating] = useState(false);
  const [rechecking, setRechecking] = useState(false);
  const [savingRule, setSavingRule] = useState(false);
  const toast = useToast();

  useEffect(() => {
    setNote(item.review_note || "");
  }, [item.review_note]);

  useEffect(() => {
    if (highlighted) setExpanded(true);
  }, [highlighted]);

  const dim = DIMENSION_META[item.dimension] || {
    label: item.dimension,
    color: "neutral.600",
    bg: "neutral.50",
  };
  const finding = findingMeta(item.finding?.status);
  const review =
    REVIEW_STATUS_META[item.review_status] || REVIEW_STATUS_META.pending;
  const sev = SEVERITY_META[item.finding?.severity || item.severity];
  // 人工自建项不参与 AI 判定，不渲染"AI · 待判定"，避免语义误导
  const hasAiVerdict = Boolean(item.finding) || item.source !== "user";
  const tenderTrace = toTraceRefs(
    (item.evidences || []).filter((e) => e.side === "tender"),
  );
  const bidTrace = toTraceRefs(
    (item.evidences || []).filter((e) => e.side === "bid"),
  );

  const doUpdate = useCallback(
    async (status: string) => {
      setUpdating(true);
      try {
        await onUpdate(item.id, status, note);
        toast({ title: "已更新", status: "success", duration: 1500 });
      } catch (e: any) {
        toast({ title: e?.message || "更新失败", status: "error" });
      } finally {
        setUpdating(false);
      }
    },
    [item.id, note, onUpdate, toast],
  );

  const doRecheck = useCallback(async () => {
    setRechecking(true);
    try {
      await onRecheck(item.id);
      toast({ title: "复检完成", status: "success", duration: 1500 });
    } catch (e: any) {
      toast({ title: e?.message || "复检失败", status: "error" });
    } finally {
      setRechecking(false);
    }
  }, [item.id, onRecheck, toast]);

  const doSaveRule = useCallback(async () => {
    setSavingRule(true);
    try {
      await onSaveRule(item.id);
      toast({ title: "已沉淀为企业规则", status: "success" });
    } catch (e: any) {
      toast({ title: e?.message || "沉淀失败", status: "error" });
    } finally {
      setSavingRule(false);
    }
  }, [item.id, onSaveRule, toast]);

  const isAlert =
    item.finding?.status === "error" ||
    item.finding?.status === "warning" ||
    item.finding?.status === "not_found";

  return (
    <Box
      border="1px solid"
      borderColor={
        highlighted ? "gold.400" : isAlert ? "error.200" : "neutral.200"
      }
      borderLeft="3px solid"
      borderLeftColor={
        item.finding?.status === "error" || item.finding?.status === "not_found"
          ? "error.500"
          : item.finding?.status === "warning"
            ? "warning.500"
            : item.finding?.status === "pass"
              ? "success.500"
              : "neutral.300"
      }
      borderRadius="lg"
      bg="white"
      overflow="hidden"
      transition="all 0.2s"
      boxShadow={highlighted ? "0 0 0 3px rgba(212,168,83,0.18)" : undefined}
    >
      {/* 头部 */}
      <Flex
        align="center"
        gap={2}
        p={3}
        cursor="pointer"
        onClick={() => setExpanded((v) => !v)}
        _hover={{ bg: "neutral.50" }}
      >
        <Box flex={1} minW={0}>
          <Flex align="center" gap={1.5} flexWrap="wrap" mb={0.5}>
            <Badge
              variant="subtle"
              bg={dim.bg}
              color={dim.color}
              fontSize="10px"
              borderRadius="full"
              px={2}
            >
              {dim.label}
            </Badge>
            <Badge
              variant="outline"
              colorScheme="gray"
              fontSize="9px"
              borderRadius="full"
            >
              {SOURCE_LABELS[item.source] || item.source}
            </Badge>
            {item.full_score > 0 && (
              <Badge
                variant="subtle"
                bg="gold.50"
                color="gold.700"
                fontSize="9px"
                borderRadius="full"
                px={2}
              >
                满分 {item.full_score}
              </Badge>
            )}
            {sev?.label && (
              <Text fontSize="10px" fontWeight="700" color={sev.color}>
                {sev.label}
              </Text>
            )}
          </Flex>
          <Text
            fontSize="13px"
            fontWeight="600"
            color="neutral.800"
            noOfLines={1}
          >
            {item.title}
          </Text>
        </Box>
        {/* 两个独立维度：自动判定结论（AI 或规则引擎）与人工确认状态，分别标注避免误读 */}
        {hasAiVerdict && (
          <Tooltip
            label="AI 或规则判定结论（自动生成，与人工确认相互独立）"
            hasArrow
            openDelay={200}
          >
            <Badge
              variant="subtle"
              bg={finding.bg}
              color={finding.color}
              fontSize="11px"
              borderRadius="full"
              px={2.5}
              py={0.5}
              flexShrink={0}
            >
              AI · {finding.label}
            </Badge>
          </Tooltip>
        )}
        <Tooltip
          label="人工确认状态（复核结论，不改变 AI 判定）"
          hasArrow
          openDelay={200}
        >
          <Badge
            variant="subtle"
            bg={review.bg}
            color={review.color}
            fontSize="10px"
            borderRadius="full"
            px={2}
            flexShrink={0}
          >
            人工 · {review.label}
          </Badge>
        </Tooltip>
        <IconButton
          aria-label={expanded ? "收起" : "展开"}
          icon={expanded ? <FiChevronUp /> : <FiChevronDown />}
          size="sm"
          variant="ghost"
          color="neutral.400"
        />
      </Flex>

      <Collapse in={expanded} animateOpacity>
        <Box px={3} pb={3}>
          {/* 判定口径 */}
          <Flex gap={3} direction={{ base: "column", md: "row" }}>
            <Box flex={1} bg="neutral.50" borderRadius="md" p={2.5}>
              <Text fontSize="10px" fontWeight="600" color="primary.600" mb={1}>
                判定口径
              </Text>
              <Text fontSize="12px" color="neutral.700" whiteSpace="pre-wrap">
                {item.requirement || "—"}
              </Text>
            </Box>
            <Box flex={1} bg="neutral.50" borderRadius="md" p={2.5}>
              <Text fontSize="10px" fontWeight="600" color="info.600" mb={1}>
                期望证据
              </Text>
              <Text fontSize="12px" color="neutral.700" whiteSpace="pre-wrap">
                {item.expected_evidence || "—"}
              </Text>
            </Box>
          </Flex>

          {/* 招标依据 */}
          {(item.tender_quote || tenderTrace.length > 0) && (
            <Box
              mt={2.5}
              bg="primary.50"
              border="1px solid"
              borderColor="primary.100"
              borderRadius="md"
              p={2.5}
            >
              <Text
                fontSize="10px"
                fontWeight="600"
                color="primary.700"
                mb={0.5}
              >
                招标依据
              </Text>
              <Text fontSize="12px" color="neutral.700" whiteSpace="pre-wrap">
                {item.tender_quote || "（招标文件对应位置）"}
              </Text>
              {tenderTrace.length > 0 && (
                <Button
                  mt={1.5}
                  size="xs"
                  variant="outline"
                  // 主题的 outline 变体忽略 colorScheme，这里显式指定主色描边
                  color="primary.600"
                  borderColor="primary.200"
                  bg="white"
                  _hover={{ bg: "primary.50", borderColor: "primary.300" }}
                  leftIcon={<FiSearch size={11} />}
                  borderRadius="full"
                  onClick={() => onTrace(tenderTrace, "tender")}
                >
                  查看招标原文 P{tenderTrace[0].page}
                </Button>
              )}
            </Box>
          )}

          {/* 判定结论 */}
          {item.finding && (
            <Box mt={2.5} bg="neutral.50" borderRadius="md" p={2.5}>
              <Flex align="center" gap={2} mb={1} flexWrap="wrap">
                <Text fontSize="10px" fontWeight="600" color="neutral.600">
                  判定结论
                </Text>
                <Badge
                  variant="subtle"
                  bg={finding.bg}
                  color={finding.color}
                  fontSize="10px"
                  borderRadius="full"
                  px={2}
                >
                  {finding.label}
                </Badge>
                <Text fontSize="10px" color="neutral.400">
                  {item.finding.engine === "rule" ? "规则引擎" : "AI 判定"}
                  {item.finding.model ? ` · ${item.finding.model}` : ""}
                  {item.finding.confidence
                    ? ` · 置信度 ${item.finding.confidence}`
                    : ""}
                </Text>
              </Flex>
              <Text fontSize="12px" color="neutral.700" whiteSpace="pre-wrap">
                {item.finding.reason || "—"}
              </Text>
              {item.finding.suggestion && (
                <Box
                  mt={1.5}
                  bg="warning.50"
                  border="1px solid"
                  borderColor="warning.200"
                  borderRadius="md"
                  p={2}
                >
                  <Text
                    fontSize="10px"
                    fontWeight="600"
                    color="warning.700"
                    mb={0.5}
                  >
                    整改建议
                  </Text>
                  <Text
                    fontSize="12px"
                    color="neutral.700"
                    whiteSpace="pre-wrap"
                  >
                    {item.finding.suggestion}
                  </Text>
                </Box>
              )}
            </Box>
          )}

          {/* 证据列表 */}
          <Box mt={2.5}>
            <Flex align="center" justify="space-between" mb={1}>
              <Text fontSize="10px" fontWeight="600" color="neutral.600">
                证据（{item.evidences?.length || 0}）
              </Text>
              {bidTrace.length > 0 && (
                <Button
                  size="xs"
                  variant="outline"
                  color="primary.600"
                  borderColor="primary.200"
                  bg="white"
                  _hover={{ bg: "primary.50", borderColor: "primary.300" }}
                  leftIcon={<FiSearch size={11} />}
                  borderRadius="full"
                  onClick={() => onTrace(bidTrace, "bid")}
                >
                  查看投标原文 P{bidTrace[0].page}
                </Button>
              )}
            </Flex>
            {!item.evidences || item.evidences.length === 0 ? (
              <Text fontSize="11px" color="orange.600">
                未定位到证据（未找到依据），请人工确认是否缺失材料。
              </Text>
            ) : (
              <Flex direction="column" gap={1.5}>
                {item.evidences.map((e) => (
                  <Flex
                    key={e.id}
                    gap={2}
                    align="flex-start"
                    p={2}
                    borderRadius="md"
                    bg={e.side === "bid" ? "info.50" : "primary.50"}
                  >
                    <Badge
                      variant="subtle"
                      bg={e.side === "bid" ? "info.100" : "primary.100"}
                      color={e.side === "bid" ? "info.700" : "primary.700"}
                      fontSize="9px"
                      borderRadius="full"
                      px={1.5}
                      flexShrink={0}
                    >
                      {e.side === "bid" ? "投标" : "招标"}
                    </Badge>
                    <Text
                      flex={1}
                      minW={0}
                      fontSize="11px"
                      color="neutral.700"
                      whiteSpace="pre-wrap"
                    >
                      {e.quote || "（该位置无文本引用）"}
                      <Text
                        as="span"
                        fontSize="10px"
                        color="neutral.400"
                        ml={1}
                      >
                        {e.file_name ? `${e.file_name} ` : ""}
                        {e.page_no > 0 ? `P${e.page_no}` : ""}
                      </Text>
                    </Text>
                  </Flex>
                ))}
              </Flex>
            )}
          </Box>

          <Divider my={3} />

          {/* 人工确认：备注独占一行，状态用滑动开关，操作与元信息分层 */}
          <Box>
            <Flex align="center" gap={2} mb={1} flexWrap="wrap">
              <Text fontSize="10px" fontWeight={600} color="neutral.600">
                审核备注
              </Text>
              <Text fontSize="10px" color="neutral.400">
                备注与状态会一并保存，作为人工复核留痕
              </Text>
            </Flex>
            <Textarea
              size="sm"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="填写确认意见（可选）"
              borderRadius="md"
              rows={2}
              bg="neutral.50"
              borderColor="neutral.200"
              _focusVisible={{
                bg: "white",
                borderColor: "primary.400",
                boxShadow: "0 0 0 1px var(--chakra-colors-primary-400)",
              }}
            />
          </Box>

          <Flex
            mt={3}
            align="center"
            justify="space-between"
            gap={2}
            flexWrap="wrap"
            rowGap={2}
          >
            <Flex align="center" gap={2} minW={0}>
              <Text
                fontSize="10px"
                fontWeight={600}
                color="neutral.600"
                flexShrink={0}
              >
                人工确认
              </Text>
              <ReviewStatusSwitch
                value={item.review_status}
                disabled={updating}
                onChange={(status) => doUpdate(status)}
              />
            </Flex>
            <Flex gap={1.5} flexShrink={0}>
              <Tooltip label="复检该项（重新召回证据并判定）">
                <IconButton
                  aria-label="复检"
                  icon={<FiRefreshCw />}
                  size="xs"
                  variant="ghost"
                  color="primary.500"
                  isLoading={rechecking}
                  onClick={doRecheck}
                />
              </Tooltip>
              <Tooltip label="沉淀为企业审核规则">
                <IconButton
                  aria-label="沉淀为规则"
                  icon={<FiFilePlus />}
                  size="xs"
                  variant="ghost"
                  color="gold.600"
                  isLoading={savingRule}
                  onClick={doSaveRule}
                />
              </Tooltip>
              {item.source === "user" && (
                <Tooltip label="删除检查项">
                  <IconButton
                    aria-label="删除"
                    icon={<FiTrash2 />}
                    size="xs"
                    variant="ghost"
                    color="error.400"
                    onClick={() => onDelete(item.id)}
                  />
                </Tooltip>
              )}
            </Flex>
          </Flex>

          <Flex
            mt={2}
            align="center"
            justify="space-between"
            gap={2}
            flexWrap="wrap"
          >
            <Text fontSize="10px" color="neutral.400">
              来源：{SOURCE_LABELS[item.source] || item.source}
              {item.reviewed_at
                ? ` · 确认于 ${new Date(item.reviewed_at).toLocaleString()}`
                : " · 尚未人工确认"}
              {item.is_user_edited ? " · 已人工编辑" : ""}
            </Text>
          </Flex>
        </Box>
      </Collapse>
    </Box>
  );
}
