"use client";

/* eslint-disable no-use-before-define, no-nested-ternary, no-unused-vars */

/* Hallmark · genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */

import {
  Accordion,
  AccordionButton,
  AccordionIcon,
  AccordionItem,
  AccordionPanel,
  Badge,
  Box,
  Button,
  Flex,
  Grid,
  HStack,
  Icon,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  SimpleGrid,
  Skeleton,
  SkeletonText,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tooltip,
  Tr,
  useDisclosure,
  useToast,
  VStack,
} from "@chakra-ui/react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useRouter } from "next/navigation";
import { useState } from "react";
import {
  FiAlertTriangle,
  FiBell,
  FiBookOpen,
  FiCalendar,
  FiCheck,
  FiChevronDown,
  FiEdit3,
  FiEye,
  FiEyeOff,
  FiFileText,
  FiHelpCircle,
  FiLink2,
  FiPlus,
  FiRefreshCw,
  FiSearch,
  FiStar,
  FiX,
} from "react-icons/fi";

import { DataSurface } from "@/components/analysis/bid-analysis-v3/workspace";
import { MarkdownContent } from "./markdown-content";

import {
  CLAUSE_NOISE_TITLE_RE,
  groupSummaryRisks,
  pickKeyDates,
} from "./model.mjs";
import { BlueprintTree } from "./blueprint-tree";
import type {
  BlueprintPanelProps,
  Category,
  Chapter,
  Clause,
  DerivedTable,
  Evidence,
  Field,
  FieldValue,
  FollowItem,
  Summary,
  V3BlueprintNode,
  WarningGroup,
} from "./types";

const MotionBox = motion(Box);
const MOTION_EASE = [0.16, 1, 0.3, 1] as const;

// 相邻条款/字段卡片的交替浅阴影：静止状态即区分相邻卡片，hover 时不改变阴影。
const CARD_SHADOW_EVEN = "0 1px 2px rgba(11, 27, 43, 0.04)";
const CARD_SHADOW_ODD = "0 2px 6px rgba(11, 27, 43, 0.07)";

const WARNING_GROUP_LABELS: Record<string, string> = {
  chapter_extract_failed: "章节提取失败",
  dynamic_second_pass_failed: "动态字段二次检索失败",
  global_fixed_extract_failed: "固定字段抽取失败",
  candidate_validation_failed: "字段校验失败",
  chapter_fallback: "章节识别兜底",
  chapter_outline_refine_failed: "章节识别告警",
  conflict_resolution_failed: "字段冲突归并失败",
  dynamic_fields_below_target: "动态字段不足",
  dynamic_fields_ranked: "动态字段已精简",
  summary_failed: "摘要生成",
  summary_llm_fallback: "摘要降级",
  stage_skipped_by_user: "阶段被跳过",
  ai_interpretation_failed: "AI 解读生成失败",
  chapter_extract_partial_enumeration: "章节内容较多",
  fixed_field_evidence_invalid: "固定字段提取异常",
  fixed_field_not_found_in_document: "固定字段未找到明确值",
};

function warningGroupLabel(group: WarningGroup) {
  return WARNING_GROUP_LABELS[group.code] || group.code;
}

function severityMeta(severity: string) {
  if (severity === "critical") {
    return { scheme: "red", color: "error.600", icon: FiAlertTriangle };
  }
  if (severity === "info") {
    return { scheme: "gray", color: "gray.500", icon: FiFileText };
  }
  return { scheme: "orange", color: "warning.600", icon: FiAlertTriangle };
}

const HIGHLIGHT_TOKEN_RE =
  /(\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日(?:\s*\d{1,2}\s*[时:：]\s*\d{0,2}\s*分?)?(?:（[^）]*）)?|\d+(?:\.\d+)?\s*[万亿]?\s*元|\d+(?:\.\d+)?\s*%)/;

function splitParagraphs(text: string) {
  const tokens = text.split(/([。；！？!?])/);
  const paragraphs: string[] = [];
  for (let index = 0; index < tokens.length; index += 2) {
    const sentence = (tokens[index] || "") + (tokens[index + 1] || "");
    if (sentence.trim()) paragraphs.push(sentence.trim());
  }
  return paragraphs.length ? paragraphs : [text];
}

function renderHighlighted(text: string) {
  const re = new RegExp(HIGHLIGHT_TOKEN_RE.source, "g");
  const segments: { text: string; highlighted: boolean }[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = re.exec(text)) !== null) {
    if (match.index > last) {
      segments.push({ text: text.slice(last, match.index), highlighted: false });
    }
    segments.push({ text: match[0], highlighted: true });
    last = match.index + match[0].length;
  }
  if (last < text.length) {
    segments.push({ text: text.slice(last), highlighted: false });
  }
  return segments.map((segment, index) =>
    segment.highlighted ? (
      <Text key={index} as="span" color="warning.700" fontWeight="700">
        {segment.text}
      </Text>
    ) : (
      <Text key={index} as="span">
        {segment.text}
      </Text>
    ),
  );
}

export function OverviewPanel({
  summary,
  categories,
  clauseCount,
  warningCount,
  warningGroups = [],
  retryingStage = "",
  resolvingWarning = null,
  onRetryStage,
  onResolveWarning,
  onResolveAllInfo,
  onJumpChapter,
  onJumpFields,
  onResolveRisk,
  resolvingRiskIndex = null,
  follows = [],
  followingKey = null,
  onRemoveFollow,
  onJumpClause,
}: {
  summary: Summary;
  categories: Category[];
  clauseCount: number;
  warningCount: number;
  warningGroups?: WarningGroup[];
  retryingStage?: string;
  resolvingWarning?: string | null;
  onRetryStage?: (stage: string) => void;
  onResolveWarning?: (group: WarningGroup) => void;
  onResolveAllInfo?: () => void;
  onJumpChapter?: (chapterId: number) => void;
  onJumpFields?: () => void;
  onResolveRisk?: (index: number, resolved: boolean, kind?: string) => void;
  resolvingRiskIndex?: number | null;
  follows?: FollowItem[];
  followingKey?: string | null;
  onRemoveFollow?: (followId: number) => void;
  onJumpClause?: (clauseId: number) => void;
}) {
  const toast = useToast();
  const riskItems = summary.risks || [];
  const riskGroups = groupSummaryRisks(riskItems);
  const openRisks = riskGroups.risks.filter((item) => !item.resolved);
  const resolvedRisks = riskGroups.risks.filter((item) => item.resolved);
  const openConfirms = riskGroups.confirms.filter((item) => !item.resolved);
  const resolvedConfirms = riskGroups.confirms.filter((item) => item.resolved);
  const legacyOpen = riskGroups.legacy.filter((item) => !item.resolved);
  const legacyResolved = riskGroups.legacy.filter((item) => item.resolved);
  const hasTypedItems =
    riskGroups.risks.length > 0 || riskGroups.confirms.length > 0;
  const { valid: keyDates, placeholders: placeholderDates } =
    pickKeyDates(categories);
  const hasKeyDates = keyDates.length > 0 || placeholderDates.length > 0;
  const summaryParagraphs = splitParagraphs(summary.overview || "");
  const alertGroups = warningGroups.filter(
    (group) => group.severity !== "info",
  );
  const infoGroups = warningGroups.filter((group) => group.severity === "info");
  const hasWarnings = warningCount > 0 || alertGroups.length > 0;
  const maxSeverity = alertGroups.some(
    (group) => group.severity === "critical",
  )
    ? "critical"
    : alertGroups.some((group) => group.severity === "warning")
      ? "warning"
      : null;
  const statusTone = hasWarnings
    ? maxSeverity === "critical"
      ? "error"
      : "warning"
    : "success";

  return (
    <VStack align="stretch" spacing={4}>
      <DataSurface p={{ base: 5, md: 6 }} boxShadow="none">
            <Flex align="center" gap={2.5}>
              <Flex
                w="36px"
                h="36px"
                borderRadius="10px"
                align="center"
                justify="center"
                bg="primary.50"
                color="primary.600"
                border="1px solid"
                borderColor="primary.100"
              >
                <Icon as={FiFileText} boxSize={4} />
              </Flex>
              <Box>
                <Text fontSize="lg" fontWeight="760" color="workbench.text">
                  招标摘要
                </Text>
                <Text mt={0.5} fontSize="xs" color="workbench.muted">
                  AI 基于招标原文生成的全局概述
                </Text>
              </Box>
            </Flex>
            <Box mt={4}>
              {summaryParagraphs.length > 0 ? (
                summaryParagraphs.map((paragraph, index) => (
                  <Text
                    key={index}
                    fontSize="sm"
                    lineHeight="1.9"
                    color="workbench.text"
                    mb={index < summaryParagraphs.length - 1 ? 2.5 : 0}
                  >
                    {renderHighlighted(paragraph)}
                  </Text>
                ))
              ) : (
                <Text fontSize="sm" color="workbench.muted" lineHeight="1.85">
                  招标文件摘要尚未生成，请结合字段、条款与原文完成核验。
                </Text>
              )}
            </Box>
          </DataSurface>

      {follows.length > 0 && (
        <DataSurface p={5} boxShadow="none">
          <Flex align="center" justify="space-between" gap={3}>
            <Flex align="center" gap={2.5}>
              <Flex
                w="36px"
                h="36px"
                borderRadius="10px"
                align="center"
                justify="center"
                bg="gold.100"
                color="gold.700"
              >
                <Icon as={FiStar} boxSize={4} />
              </Flex>
              <Box>
                <Text fontWeight="740">我的关注</Text>
                <Text mt={0.5} fontSize="xs" color="workbench.muted">
                  在字段 / 条款页签添加的关注项
                </Text>
              </Box>
            </Flex>
            <Badge colorScheme="gold" variant="subtle" flexShrink={0}>
              {follows.length} 项
            </Badge>
          </Flex>
          <SimpleGrid mt={4} columns={{ base: 1, xl: 2 }} spacing={3}>
            {follows.map((follow) => (
              <Flex
                key={follow.id}
                align="flex-start"
                gap={3}
                p={3}
                borderRadius="10px"
                bg="neutral.50"
                border="1px solid"
                borderColor="workbench.line"
              >
                <Flex
                  w="28px"
                  h="28px"
                  flexShrink={0}
                  borderRadius="8px"
                  align="center"
                  justify="center"
                  bg={
                    follow.target_type === "field"
                      ? "primary.50"
                      : "success.50"
                  }
                  color={
                    follow.target_type === "field"
                      ? "primary.600"
                      : "success.600"
                  }
                >
                  <Icon
                    as={follow.target_type === "field" ? FiFileText : FiBookOpen}
                    boxSize={3.5}
                  />
                </Flex>
                <Box flex={1} minW={0}>
                  <HStack spacing={2} wrap="wrap">
                    <Badge
                      variant="subtle"
                      colorScheme={
                        follow.target_type === "field" ? "blue" : "green"
                      }
                    >
                      {follow.target_type === "field" ? "字段" : "条款"}
                    </Badge>
                    <Text fontWeight="700" fontSize="sm">
                      {follow.title}
                    </Text>
                  </HStack>
                  {follow.content && (
                    <Text
                      mt={1.5}
                      fontSize="sm"
                      color="workbench.muted"
                      lineHeight="1.7"
                      noOfLines={2}
                    >
                      {follow.content}
                    </Text>
                  )}
                </Box>
                <HStack flexShrink={0} spacing={1}>
                  <Button
                    size="sm"
                    minH="40px"
                    variant="ghost"
                    onClick={() =>
                      follow.target_type === "field"
                        ? onJumpFields?.()
                        : onJumpClause?.(follow.target_id)
                    }
                  >
                    查看
                  </Button>
                  <IconButton
                    aria-label="取消关注"
                    icon={<FiX />}
                    size="sm"
                    minW="40px"
                    minH="40px"
                    variant="ghost"
                    color="workbench.muted"
                    isLoading={followingKey === `follow:${follow.id}`}
                    onClick={() => onRemoveFollow?.(follow.id)}
                  />
                </HStack>
              </Flex>
            ))}
          </SimpleGrid>
        </DataSurface>
      )}

      <DataSurface p={5} boxShadow="none">
          <Flex align="center" gap={2.5}>
            <Flex
              w="36px"
              h="36px"
              borderRadius="10px"
              align="center"
              justify="center"
              bg="success.50"
              color="success.600"
              border="1px solid"
              borderColor="success.100"
            >
              <Icon as={FiCheck} boxSize={4} />
            </Flex>
            <Box>
              <Text fontWeight="740">关键信息</Text>
              <Text mt={0.5} fontSize="xs" color="workbench.muted">
                摘要提炼的核心要点
              </Text>
            </Box>
          </Flex>
          <SimpleGrid mt={4} columns={{ base: 1, md: 2 }} spacing={3}>
            {(summary.key_points || []).map((item, index) => (
              <Flex
                key={`${item}-${index}`}
                align="flex-start"
                gap={2.5}
                p={2.5}
                borderRadius="10px"
                bg="neutral.50"
                border="1px solid"
                borderColor="workbench.line"
              >
                <Flex
                  w="20px"
                  h="20px"
                  flexShrink={0}
                  borderRadius="full"
                  align="center"
                  justify="center"
                  bg="primary.50"
                  color="primary.600"
                  fontSize="xs"
                  fontWeight="700"
                  mt="1px"
                >
                  {index + 1}
                </Flex>
                <Text fontSize="sm" lineHeight="1.7" flex={1} minW={0}>
                  {item}
                </Text>
              </Flex>
            ))}
          </SimpleGrid>
          {!summary.key_points?.length && (
            <Text mt={4} fontSize="sm" color="workbench.muted">
              暂无摘要要点。
            </Text>
          )}
        </DataSurface>

        {hasKeyDates && (
          <DataSurface
            p={5}
            boxShadow="none"
            bg="info.50"
            border="1px solid"
            borderColor="info.200"
          >
            <Flex align="center" justify="space-between" gap={3}>
              <Flex align="center" gap={2.5}>
                <Flex
                  w="36px"
                  h="36px"
                  borderRadius="10px"
                  align="center"
                  justify="center"
                  bg="info.100"
                  color="info.600"
                >
                  <Icon as={FiCalendar} boxSize={4} />
                </Flex>
                <Box>
                  <Text fontWeight="740">重要日期</Text>
                  <Text mt={0.5} fontSize="xs" color="workbench.muted">
                    关键时间节点，可设置提醒
                  </Text>
                </Box>
              </Flex>
              {keyDates.length > 0 ? (
                <Badge colorScheme="info" variant="subtle" flexShrink={0}>
                  已提取 {keyDates.length} 项
                </Badge>
              ) : (
                <Badge colorScheme="gray" variant="subtle" flexShrink={0}>
                  {placeholderDates.length} 项缺少有效值
                </Badge>
              )}
            </Flex>
            {keyDates.length > 0 && (
              <SimpleGrid
                mt={4}
                columns={{ base: 1, sm: 2, xl: 4 }}
                spacing={3}
              >
                {keyDates.map((item) => (
                  <Box
                    key={item.fieldKey}
                    p={3}
                    borderRadius="10px"
                    bg="white"
                    border="1px solid"
                    borderColor="info.100"
                  >
                    <Flex align="center" justify="space-between" gap={2}>
                      <Text fontSize="xs" color="workbench.muted">
                        {item.label}
                      </Text>
                      <Tooltip label="添加提醒（功能即将上线）">
                        <IconButton
                          aria-label={`为${item.label}添加提醒`}
                          icon={<FiBell />}
                          size="sm"
                          minW="34px"
                          minH="34px"
                          variant="ghost"
                          color="info.600"
                          _hover={{ bg: "info.100" }}
                          onClick={() =>
                            toast({
                              status: "info",
                              title: "提醒功能即将上线",
                              description: `已记录：${item.label}`,
                              duration: 2000,
                            })
                          }
                        />
                      </Tooltip>
                    </Flex>
                    <Text
                      mt={1.5}
                      fontSize="md"
                      fontWeight="720"
                      fontFamily="mono"
                      color="workbench.text"
                      lineHeight="1.5"
                    >
                      {renderHighlighted(item.value)}
                    </Text>
                  </Box>
                ))}
              </SimpleGrid>
            )}
            {placeholderDates.length > 0 && (
              <VStack mt={4} align="stretch" spacing={2}>
                {placeholderDates.map((item) => (
                  <Flex
                    key={item.fieldKey}
                    align="center"
                    justify="space-between"
                    gap={3}
                    p={3}
                    borderRadius="10px"
                    bg="white"
                    border="1px solid"
                    borderColor="info.100"
                  >
                    <HStack spacing={2.5} minW={0}>
                      <Text fontSize="sm" fontWeight="680" flexShrink={0}>
                        {item.label}
                      </Text>
                      <Badge colorScheme="gray" variant="subtle" flexShrink={0}>
                        缺少有效值
                      </Badge>
                    </HStack>
                    <Button
                      size="sm"
                      minH="36px"
                      flexShrink={0}
                      variant="ghost"
                      colorScheme="primary"
                      onClick={onJumpFields}
                    >
                      去字段页核对
                    </Button>
                  </Flex>
                ))}
              </VStack>
            )}
          </DataSurface>
        )}

      {riskGroups.legacy.length > 0 && (
        <DataSurface p={5} boxShadow="none">
          <Flex align="center" justify="space-between" gap={3}>
            <Flex align="center" gap={2.5}>
              <Flex
                w="36px"
                h="36px"
                borderRadius="10px"
                align="center"
                justify="center"
                bg="warning.50"
                color="warning.600"
                border="1px solid"
                borderColor="warning.100"
              >
                <Icon as={FiAlertTriangle} boxSize={4} />
              </Flex>
              <Box>
                <Text fontWeight="740">风险与待确认事项</Text>
                <Text mt={0.5} fontSize="xs" color="workbench.muted">
                  结合招标原文标注的风险与待确认项
                </Text>
              </Box>
            </Flex>
            {legacyOpen.length > 0 && (
              <Badge colorScheme="orange" variant="subtle" flexShrink={0}>
                未解决 {legacyOpen.length}
              </Badge>
            )}
          </Flex>
          {legacyOpen.length > 0 ? (
            <Grid
              mt={4}
              templateColumns={{ base: "1fr", xl: "repeat(2,minmax(0,1fr))" }}
              gap={3}
            >
              {legacyOpen.map((item) => (
                <Flex
                  key={item.index}
                  align="flex-start"
                  gap={3}
                  p={3}
                  borderRadius="10px"
                  bg="warning.50"
                  border="1px solid"
                  borderColor="warning.100"
                >
                  <Icon
                    as={FiAlertTriangle}
                    mt={1}
                    color="warning.600"
                    flexShrink={0}
                  />
                  <Text fontSize="sm" lineHeight="1.75" flex={1} minW={0}>
                    {item.text}
                  </Text>
                  <Button
                    size="sm"
                    minH="44px"
                    flexShrink={0}
                    variant="outline"
                    colorScheme="primary"
                    isLoading={resolvingRiskIndex === item.index}
                    loadingText="保存中"
                    onClick={() => onResolveRisk?.(item.index, true, item.kind)}
                  >
                    标记已解决
                  </Button>
                </Flex>
              ))}
            </Grid>
          ) : (
            <Text mt={4} fontSize="sm" color="workbench.muted" lineHeight="1.75">
              当前没有待确认事项。
            </Text>
          )}
          {legacyResolved.length > 0 && (
            <Accordion mt={4} allowToggle reduceMotion>
              <AccordionItem
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="12px"
                overflow="hidden"
              >
                <AccordionButton minH="44px" px={3.5} py={2.5}>
                  <Text
                    flex={1}
                    textAlign="left"
                    fontSize="sm"
                    fontWeight="680"
                  >
                    已解决 {legacyResolved.length} 项
                  </Text>
                  <AccordionIcon color="workbench.muted" />
                </AccordionButton>
                <AccordionPanel px={3.5} py={3}>
                  <VStack align="stretch" spacing={2}>
                    {legacyResolved.map((item) => (
                      <Flex key={item.index} align="flex-start" gap={3}>
                        <Icon
                          as={FiCheck}
                          mt={1}
                          color="success.600"
                          flexShrink={0}
                        />
                        <Text
                          fontSize="sm"
                          lineHeight="1.75"
                          flex={1}
                          minW={0}
                          color="workbench.muted"
                        >
                          {item.text}
                        </Text>
                        <Button
                          size="sm"
                          minH="40px"
                          flexShrink={0}
                          variant="ghost"
                          isLoading={resolvingRiskIndex === item.index}
                          loadingText="处理中"
                          onClick={() =>
                            onResolveRisk?.(item.index, false, item.kind)
                          }
                        >
                          撤销
                        </Button>
                      </Flex>
                    ))}
                  </VStack>
                </AccordionPanel>
              </AccordionItem>
            </Accordion>
          )}
        </DataSurface>
      )}

      {hasTypedItems && (
        <DataSurface
          p={5}
          boxShadow="none"
          bg="error.50"
          border="1px solid"
          borderColor="error.100"
        >
          <Flex align="center" justify="space-between" gap={3}>
            <Flex align="center" gap={2.5}>
              <Flex
                w="36px"
                h="36px"
                borderRadius="10px"
                align="center"
                justify="center"
                bg="error.100"
                color="error.600"
              >
                <Icon as={FiAlertTriangle} boxSize={4} />
              </Flex>
              <Box>
                <Text fontWeight="740">风险</Text>
                <Text mt={0.5} fontSize="xs" color="workbench.muted">
                  招标条件中已明确写出的不利或约束事项，需评估影响并提前准备预案
                </Text>
              </Box>
            </Flex>
            {openRisks.length > 0 && (
              <Badge colorScheme="red" variant="subtle" flexShrink={0}>
                待处理 {openRisks.length}
              </Badge>
            )}
          </Flex>
          {openRisks.length > 0 ? (
            <Grid
              mt={4}
              templateColumns={{ base: "1fr", xl: "repeat(2,minmax(0,1fr))" }}
              gap={3}
            >
              {openRisks.map((item) => (
                <Flex
                  key={item.index}
                  align="flex-start"
                  gap={3}
                  p={3}
                  borderRadius="10px"
                  bg="white"
                  border="1px solid"
                  borderColor="error.100"
                >
                  <Icon
                    as={FiAlertTriangle}
                    mt={1}
                    color="error.600"
                    flexShrink={0}
                  />
                  <Box flex={1} minW={0}>
                    {item.label && (
                      <Badge
                        colorScheme="red"
                        variant="subtle"
                        mb={1.5}
                        flexShrink={0}
                      >
                        {item.label}
                      </Badge>
                    )}
                    <Text fontSize="sm" lineHeight="1.75">
                      {item.text}
                    </Text>
                  </Box>
                  <Button
                    size="sm"
                    minH="44px"
                    flexShrink={0}
                    variant="outline"
                    colorScheme="primary"
                    isLoading={resolvingRiskIndex === item.index}
                    loadingText="保存中"
                    onClick={() => onResolveRisk?.(item.index, true, item.kind)}
                  >
                    已知悉
                  </Button>
                </Flex>
              ))}
            </Grid>
          ) : (
            <Text mt={4} fontSize="sm" color="workbench.muted" lineHeight="1.75">
              暂无已识别的风险事项。
            </Text>
          )}
          {resolvedRisks.length > 0 && (
            <Accordion mt={4} allowToggle reduceMotion>
              <AccordionItem
                border="1px solid"
                borderColor="error.100"
                borderRadius="12px"
                overflow="hidden"
              >
                <AccordionButton minH="44px" px={3.5} py={2.5}>
                  <Text
                    flex={1}
                    textAlign="left"
                    fontSize="sm"
                    fontWeight="680"
                  >
                    已知悉 {resolvedRisks.length} 项
                  </Text>
                  <AccordionIcon color="workbench.muted" />
                </AccordionButton>
                <AccordionPanel px={3.5} py={3}>
                  <VStack align="stretch" spacing={2}>
                    {resolvedRisks.map((item) => (
                      <Flex key={item.index} align="flex-start" gap={3}>
                        <Icon
                          as={FiCheck}
                          mt={1}
                          color="success.600"
                          flexShrink={0}
                        />
                        <Box flex={1} minW={0}>
                          {item.label && (
                            <Badge
                              colorScheme="red"
                              variant="subtle"
                              mr={1.5}
                            >
                              {item.label}
                            </Badge>
                          )}
                          <Text
                            fontSize="sm"
                            lineHeight="1.75"
                            color="workbench.muted"
                          >
                            {item.text}
                          </Text>
                        </Box>
                        <Button
                          size="sm"
                          minH="40px"
                          flexShrink={0}
                          variant="ghost"
                          isLoading={resolvingRiskIndex === item.index}
                          loadingText="处理中"
                          onClick={() =>
                            onResolveRisk?.(item.index, false, item.kind)
                          }
                        >
                          撤销
                        </Button>
                      </Flex>
                    ))}
                  </VStack>
                </AccordionPanel>
              </AccordionItem>
            </Accordion>
          )}
        </DataSurface>
      )}

      {hasTypedItems && (
        <DataSurface
          p={5}
          boxShadow="none"
          bg="primary.50"
          border="1px solid"
          borderColor="primary.100"
        >
          <Flex align="center" justify="space-between" gap={3}>
            <Flex align="center" gap={2.5}>
              <Flex
                w="36px"
                h="36px"
                borderRadius="10px"
                align="center"
                justify="center"
                bg="primary.100"
                color="primary.600"
              >
                <Icon as={FiHelpCircle} boxSize={4} />
              </Flex>
              <Box>
                <Text fontWeight="740">待确认事项</Text>
                <Text mt={0.5} fontSize="xs" color="workbench.muted">
                  招标文件表述模糊、信息缺失或有歧义，需向招标人澄清后确认的事项
                </Text>
              </Box>
            </Flex>
            {openConfirms.length > 0 && (
              <Badge colorScheme="blue" variant="subtle" flexShrink={0}>
                待确认 {openConfirms.length}
              </Badge>
            )}
          </Flex>
          {openConfirms.length > 0 ? (
            <Grid
              mt={4}
              templateColumns={{ base: "1fr", xl: "repeat(2,minmax(0,1fr))" }}
              gap={3}
            >
              {openConfirms.map((item) => (
                <Flex
                  key={item.index}
                  align="flex-start"
                  gap={3}
                  p={3}
                  borderRadius="10px"
                  bg="white"
                  border="1px solid"
                  borderColor="primary.100"
                >
                  <Icon
                    as={FiHelpCircle}
                    mt={1}
                    color="primary.600"
                    flexShrink={0}
                  />
                  <Box flex={1} minW={0}>
                    {item.label && (
                      <Badge
                        colorScheme="blue"
                        variant="subtle"
                        mb={1.5}
                        flexShrink={0}
                      >
                        {item.label}
                      </Badge>
                    )}
                    <Text fontSize="sm" lineHeight="1.75">
                      {item.text}
                    </Text>
                  </Box>
                  <Button
                    size="sm"
                    minH="44px"
                    flexShrink={0}
                    variant="outline"
                    colorScheme="primary"
                    isLoading={resolvingRiskIndex === item.index}
                    loadingText="保存中"
                    onClick={() => onResolveRisk?.(item.index, true, item.kind)}
                  >
                    标记已解决
                  </Button>
                </Flex>
              ))}
            </Grid>
          ) : (
            <Text mt={4} fontSize="sm" color="workbench.muted" lineHeight="1.75">
              暂无待确认事项。
            </Text>
          )}
          {resolvedConfirms.length > 0 && (
            <Accordion mt={4} allowToggle reduceMotion>
              <AccordionItem
                border="1px solid"
                borderColor="primary.100"
                borderRadius="12px"
                overflow="hidden"
              >
                <AccordionButton minH="44px" px={3.5} py={2.5}>
                  <Text
                    flex={1}
                    textAlign="left"
                    fontSize="sm"
                    fontWeight="680"
                  >
                    已解决 {resolvedConfirms.length} 项
                  </Text>
                  <AccordionIcon color="workbench.muted" />
                </AccordionButton>
                <AccordionPanel px={3.5} py={3}>
                  <VStack align="stretch" spacing={2}>
                    {resolvedConfirms.map((item) => (
                      <Flex key={item.index} align="flex-start" gap={3}>
                        <Icon
                          as={FiCheck}
                          mt={1}
                          color="success.600"
                          flexShrink={0}
                        />
                        <Box flex={1} minW={0}>
                          {item.label && (
                            <Badge
                              colorScheme="blue"
                              variant="subtle"
                              mr={1.5}
                            >
                              {item.label}
                            </Badge>
                          )}
                          <Text
                            fontSize="sm"
                            lineHeight="1.75"
                            color="workbench.muted"
                          >
                            {item.text}
                          </Text>
                        </Box>
                        <Button
                          size="sm"
                          minH="40px"
                          flexShrink={0}
                          variant="ghost"
                          isLoading={resolvingRiskIndex === item.index}
                          loadingText="处理中"
                          onClick={() =>
                            onResolveRisk?.(item.index, false, item.kind)
                          }
                        >
                          撤销
                        </Button>
                      </Flex>
                    ))}
                  </VStack>
                </AccordionPanel>
              </AccordionItem>
            </Accordion>
          )}
        </DataSurface>
      )}

        <DataSurface
          p={{ base: 5, md: 6 }}
          boxShadow="none"
          bg={
            hasWarnings
              ? "warning.50"
              : infoGroups.length
                ? "gray.50"
                : "success.50"
          }
          borderColor={
            hasWarnings
              ? "warning.200"
              : infoGroups.length
                ? "gray.200"
                : "success.200"
          }
        >
          <Flex align="center" gap={3}>
            <Flex
              w="42px"
              h="42px"
              borderRadius="12px"
              align="center"
              justify="center"
              bg={
                hasWarnings
                  ? "warning.100"
                  : infoGroups.length
                    ? "gray.100"
                    : "success.100"
              }
              color={
                statusTone === "error"
                  ? "error.600"
                  : hasWarnings
                    ? "warning.700"
                    : infoGroups.length
                      ? "gray.500"
                      : "success.700"
              }
            >
              <Icon
                as={
                  hasWarnings
                    ? FiAlertTriangle
                    : infoGroups.length
                      ? FiBell
                      : FiCheck
                }
                boxSize={5}
              />
            </Flex>
            <Box>
              <Text fontWeight="740" color="workbench.text">
                {hasWarnings ? "解析告警" : "解析状态"}
              </Text>
              <Text mt={0.5} fontSize="xs" color="workbench.muted">
                {hasWarnings
                  ? "按根因分组，可逐项处理、解决或重试"
                  : infoGroups.length
                    ? "解析完成，以下为系统提示，不影响结果"
                    : "未产生解析告警"}
              </Text>
            </Box>
          </Flex>
          <Text
            mt={5}
            fontSize="4xl"
            lineHeight="1"
            fontWeight="800"
            fontFamily="mono"
            color={
              statusTone === "error"
                ? "error.800"
                : hasWarnings
                  ? "warning.800"
                  : infoGroups.length
                    ? "gray.600"
                    : "success.800"
            }
          >
            {warningCount}
          </Text>
          <Text
            mt={2}
            color={
              hasWarnings
                ? "warning.800"
                : infoGroups.length
                  ? "gray.600"
                  : "success.800"
            }
            fontSize="sm"
          >
            {hasWarnings
              ? "条未解决告警"
              : infoGroups.length
                ? "无未解决告警"
                : "当前无需处理解析告警"}
          </Text>
          {hasWarnings && alertGroups.length > 0 && (
            <Accordion allowMultiple mt={4} reduceMotion>
              {alertGroups.map((group) => {
                const meta = severityMeta(group.severity);
                const occurrences = group.occurrences || group.count;
                return (
                  <AccordionItem
                    key={`${group.code}-${group.stage}`}
                    border="1px solid"
                    borderColor="warning.200"
                    borderRadius="12px"
                    mb={2}
                    overflow="hidden"
                  >
                    <AccordionButton
                      px={3.5}
                      py={3}
                      _expanded={{ bg: "warning.100" }}
                      _hover={{ bg: "warning.100" }}
                    >
                      <HStack flex={1} spacing={2.5} minW={0}>
                        <Icon
                          as={meta.icon}
                          color={meta.color}
                          flexShrink={0}
                        />
                        <Text
                          fontWeight="680"
                          flex={1}
                          minW={0}
                          noOfLines={1}
                          textAlign="left"
                        >
                          {warningGroupLabel(group)}
                        </Text>
                        <Badge
                          colorScheme={meta.scheme}
                          variant="subtle"
                          flexShrink={0}
                        >
                          ×{occurrences}
                        </Badge>
                      </HStack>
                      <AccordionIcon color="workbench.muted" />
                    </AccordionButton>
                    <AccordionPanel
                      px={3.5}
                      py={3.5}
                      bg="white"
                      borderTop="1px solid"
                      borderColor="warning.100"
                    >
                      <Text
                        fontSize="sm"
                        color="workbench.muted"
                        lineHeight="1.75"
                      >
                        {group.sample_message}
                      </Text>
                      {group.reasons && Object.keys(group.reasons).length > 0 && (
                        <VStack mt={3} align="stretch" spacing={1.5}>
                          <Text
                            fontSize="xs"
                            fontWeight="700"
                            color="workbench.muted"
                          >
                            被忽略原因分布
                          </Text>
                          {Object.entries(group.reasons)
                            .sort((a, b) => b[1] - a[1])
                            .map(([reason, n]) => (
                              <Flex
                                key={reason}
                                justify="space-between"
                                fontSize="sm"
                                lineHeight="1.5"
                              >
                                <Text color="workbench.muted">{reason}</Text>
                                <Text fontWeight="680" color="workbench.text">
                                  {n} 个
                                </Text>
                              </Flex>
                            ))}
                        </VStack>
                      )}
                      {group.affected_chapters?.length > 0 && (
                        <Flex mt={3} gap={2} wrap="wrap">
                          {group.affected_chapters
                            .slice(0, 6)
                            .map((chapter) => (
                              <Button
                                key={chapter.id}
                                size="xs"
                                variant="outline"
                                colorScheme="primary"
                                leftIcon={<FiBookOpen />}
                                onClick={() => onJumpChapter?.(chapter.id)}
                              >
                                {chapter.title || `第 ${chapter.page_start} 页`}{" "}
                                · P{chapter.page_start}
                              </Button>
                            ))}
                          {group.affected_chapters.length > 6 && (
                            <Text
                              fontSize="xs"
                              color="workbench.muted"
                              alignSelf="center"
                            >
                              等 {group.affected_chapters.length} 个章节
                            </Text>
                          )}
                        </Flex>
                      )}
                      <Flex mt={3.5} gap={2} justify="flex-end" wrap="wrap">
                        {onJumpFields && (
                          <Button
                            size="sm"
                            variant="ghost"
                            leftIcon={<FiFileText />}
                            onClick={onJumpFields}
                          >
                            查看字段
                          </Button>
                        )}
                        {onResolveWarning && (
                          <Button
                            size="sm"
                            variant="outline"
                            colorScheme="primary"
                            leftIcon={<FiCheck />}
                            isLoading={
                              resolvingWarning ===
                              (group.group_key || `id:${group.id}`)
                            }
                            loadingText="处理中"
                            onClick={() => onResolveWarning(group)}
                          >
                            标记已解决
                          </Button>
                        )}
                        {group.retry_target_stage && onRetryStage && (
                          <Button
                            size="sm"
                            colorScheme="primary"
                            leftIcon={<FiRefreshCw />}
                            isLoading={
                              retryingStage === group.retry_target_stage
                            }
                            loadingText="重试中"
                            onClick={() =>
                              onRetryStage(group.retry_target_stage)
                            }
                          >
                            重试该阶段
                          </Button>
                        )}
                      </Flex>
                    </AccordionPanel>
                  </AccordionItem>
                );
              })}
            </Accordion>
          )}
          {hasWarnings && alertGroups.length === 0 && (
            <Text
              mt={4}
              fontSize="sm"
              color="workbench.muted"
              lineHeight="1.75"
            >
              解析过程产生了告警，建议结合字段、条款与招标原文完成核验。
            </Text>
          )}
          {infoGroups.length > 0 && (
            <Box mt={4}>
              <Flex align="center" justify="space-between" mb={2}>
                <Text
                  fontSize="xs"
                  fontWeight="700"
                  color="workbench.muted"
                  letterSpacing="0.02em"
                >
                  系统提示
                </Text>
                {onResolveAllInfo && (
                  <Button
                    size="xs"
                    variant="ghost"
                    colorScheme="gray"
                    leftIcon={<FiCheck />}
                    isLoading={resolvingWarning === "all-info"}
                    loadingText="忽略中"
                    onClick={onResolveAllInfo}
                  >
                    忽略全部提示
                  </Button>
                )}
              </Flex>
              <VStack align="stretch" spacing={2}>
                {infoGroups.map((group) => {
                  const meta = severityMeta(group.severity);
                  return (
                    <Flex
                      key={`${group.code}-${group.stage}`}
                      align="flex-start"
                      gap={2.5}
                      p={3}
                      borderRadius="10px"
                      bg="white"
                      border="1px solid"
                      borderColor="gray.200"
                    >
                      <Icon
                        as={meta.icon}
                        color={meta.color}
                        mt={0.5}
                        flexShrink={0}
                      />
                      <Box flex={1} minW={0}>
                        <Text fontSize="sm" fontWeight="650">
                          {warningGroupLabel(group)}
                        </Text>
                        <Text
                          mt={0.5}
                          fontSize="xs"
                          color="workbench.muted"
                          lineHeight="1.7"
                        >
                          {group.sample_message}
                        </Text>
                      </Box>
                      {onResolveWarning && (
                        <Button
                          size="xs"
                          variant="ghost"
                          colorScheme="gray"
                          leftIcon={<FiCheck />}
                          isLoading={
                            resolvingWarning ===
                            (group.group_key || `id:${group.id}`)
                          }
                          loadingText="处理中"
                          onClick={() => onResolveWarning(group)}
                        >
                          忽略
                        </Button>
                      )}
                    </Flex>
                  );
                })}
              </VStack>
            </Box>
          )}
        </DataSurface>

    </VStack>
  );
}

export function FieldsPanel({
  categories,
  activeKey,
  onCategory,
  activeCategory,
  notFoundFields,
  activeValueId,
  sourceOpen,
  onEvidence,
  onEdit,
  followedFieldIds = new Set<number>(),
  onToggleFollow,
  followingKey = null,
}: {
  categories: Category[];
  activeKey: string;
  onCategory: (key: string) => void;
  activeCategory?: Category;
  notFoundFields: Field[];
  activeValueId?: number;
  sourceOpen: boolean;
  onEvidence: (
    label: string,
    evidences: Evidence[],
    value?: FieldValue,
  ) => void;
  onEdit: (value: FieldValue) => void;
  followedFieldIds?: Set<number>;
  onToggleFollow?: (field: Field) => void;
  followingKey?: string | null;
}) {
  const visibleFields =
    activeCategory?.fields.filter(
      (field) =>
        field.extract_status !== "not_found" || Boolean(field.ai_interpretation),
    ) || [];
  const [collapsedFieldIds, setCollapsedFieldIds] = useState<Set<number>>(
    new Set(),
  );
  const toggleFieldCollapse = (fieldId: number) => {
    setCollapsedFieldIds((current) => {
      const next = new Set(current);
      if (next.has(fieldId)) next.delete(fieldId);
      else next.add(fieldId);
      return next;
    });
  };
  const [aiHidden, setAiHidden] = useState(false);
  const [hiddenFieldInterpretationIds, setHiddenFieldInterpretationIds] =
    useState<Set<number>>(new Set());
  const toggleFieldInterpretation = (fieldId: number) => {
    setHiddenFieldInterpretationIds((current) => {
      const next = new Set(current);
      if (next.has(fieldId)) next.delete(fieldId);
      else next.add(fieldId);
      return next;
    });
  };
  const hasInterpretations = categories.some((category) =>
    category.fields.some((field) => Boolean(field.ai_interpretation)),
  );

  return (
    <Box>
      <Flex gap={2} align="center">
        <Flex
          flex={1}
          minW={0}
          gap={2}
          overflowX="auto"
          pb={2}
          className="thin-scrollbars"
          aria-label="字段分类"
        >
          {categories.map((category) => {
          const active = activeKey === category.key;
          const count = category.fields.filter(
            (field) =>
              field.extract_status !== "not_found" ||
              Boolean(field.ai_interpretation),
          ).length;
          return (
            <Button
              key={category.key}
              size="sm"
              minH="44px"
              flexShrink={0}
              border="1px solid"
              borderColor={active ? "workbench.control" : "workbench.line"}
              bg={active ? "workbench.control" : "white"}
              color={active ? "white" : "workbench.muted"}
              aria-pressed={active}
              onClick={() => onCategory(category.key)}
              transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1), color 160ms cubic-bezier(0.16,1,0.3,1), transform 100ms cubic-bezier(0.16,1,0.3,1)"
              _hover={{
                bg: active ? "workbench.controlRaised" : "primary.50",
                borderColor: active ? "workbench.controlRaised" : "primary.200",
              }}
              _active={{ transform: "translateY(1px)" }}
              _focusVisible={{
                boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
              }}
            >
              {category.name}
              <Text
                as="span"
                ml={2}
                minW="20px"
                px={1.5}
                borderRadius="full"
                bg={active ? "whiteAlpha.200" : "neutral.100"}
                color={active ? "white" : "neutral.600"}
                fontFamily="mono"
                fontSize="xs"
              >
                {count}
              </Text>
            </Button>
          );
          })}
        </Flex>
        <Button
          size="sm"
          minH="40px"
          flexShrink={0}
          variant="outline"
          leftIcon={aiHidden ? <FiEye /> : <FiEyeOff />}
          isDisabled={!hasInterpretations}
          onClick={() => setAiHidden((value) => !value)}
          _focusVisible={{
            boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
          }}
        >
          {aiHidden ? "展示AI解读" : "隐藏AI解读"}
        </Button>
      </Flex>

      <AnimatePresence mode="wait">
        <MotionBox
          key={activeCategory?.key || "empty"}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15, ease: MOTION_EASE }}
        >
          <DataSurface mt={2} p={0} overflow="hidden" boxShadow="none">
            <Box px={{ base: 4, md: 5 }} py={4} bg="neutral.50">
              <Flex
                justify="space-between"
                gap={3}
                align={{ base: "flex-start", sm: "center" }}
                direction={{ base: "column", sm: "row" }}
              >
                <Box>
                  <Text fontWeight="750">
                    {activeCategory?.name || "字段分类"}
                  </Text>
                  <Text mt={1} fontSize="sm" color="workbench.muted">
                    {activeCategory?.description || "选择分类查看已提取字段。"}
                  </Text>
                </Box>
                <Badge variant="subtle" colorScheme="blue" flexShrink={0}>
                  已提取 {visibleFields.length}
                </Badge>
              </Flex>
            </Box>

            <VStack align="stretch" spacing={2.5} p={{ base: 3, md: 4 }}>
              {visibleFields.map((field) => {
                const collapsed = collapsedFieldIds.has(field.id);
                return (
                  <Box
                    key={field.id}
                    border="1px solid"
                    borderColor="workbench.line"
                    borderRadius="12px"
                    bg="white"
                    overflow="hidden"
                    transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1)"
                    _hover={{
                      bg: "neutral.50",
                      borderColor: "primary.200",
                    }}
                  >
                    <Flex
                      justify="space-between"
                      gap={3}
                      align="center"
                      px={{ base: 4, md: 5 }}
                      py={3.5}
                    >
                      <HStack spacing={2} wrap="wrap">
                        <Text fontWeight="730">{field.display_name}</Text>
                        <Badge
                          variant="subtle"
                          colorScheme={
                            field.origin === "user"
                              ? "blue"
                              : field.origin === "dynamic"
                                ? "purple"
                                : "gray"
                          }
                        >
                          {field.origin === "user"
                            ? "人工"
                            : field.origin === "dynamic"
                              ? "动态"
                              : "固定"}
                        </Badge>
                      </HStack>
                      <HStack flexShrink={0} spacing={1}>
                        {!field.values.length && !field.ai_interpretation && (
                          <Badge colorScheme="orange">待核验</Badge>
                        )}
                        <IconButton
                          aria-label={collapsed ? "展开字段" : "折叠字段"}
                          icon={
                            <Icon
                              as={FiChevronDown}
                              transform={
                                collapsed ? "rotate(-90deg)" : undefined
                              }
                              transition="transform 160ms cubic-bezier(0.16,1,0.3,1)"
                            />
                          }
                          size="sm"
                          minW="40px"
                          minH="40px"
                          variant="ghost"
                          color="workbench.muted"
                          onClick={() => toggleFieldCollapse(field.id)}
                        />
                        <IconButton
                          aria-label={
                            followedFieldIds.has(field.id)
                              ? "取消关注"
                              : "添加关注"
                          }
                          icon={<FiStar />}
                          size="sm"
                          minW="40px"
                          minH="40px"
                          variant="ghost"
                          color={
                            followedFieldIds.has(field.id)
                              ? "gold.500"
                              : "workbench.muted"
                          }
                          isLoading={followingKey === `field:${field.id}`}
                          onClick={() => onToggleFollow?.(field)}
                        />
                      </HStack>
                    </Flex>

                    {!collapsed && (
                      <Box
                        px={{ base: 4, md: 5 }}
                        pb={4}
                        borderTop="1px solid"
                        borderColor="workbench.line"
                      >
                        <VStack mt={3} align="stretch" spacing={2}>
                  {field.values.map((value) => {
                    const canOpen = value.evidences.length > 0;
                    const selected = sourceOpen && activeValueId === value.id;
                    // 表格型字段：优先把字段值本身的 Markdown 表格渲染成主题表格
                    const valueMatrix =
                      field.value_type === "table"
                        ? matrixFromMarkdownTable(value.display_value)
                        : null;
                    return (
                      <Box
                        key={value.id}
                        p={2}
                        borderRadius="10px"
                        border="1px solid"
                        borderColor={selected ? "primary.200" : "transparent"}
                        bg={
                          selected
                            ? "primary.50"
                            : value.origin === "user"
                              ? "primary.50"
                              : "neutral.50"
                        }
                        transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1)"
                        _hover={
                          canOpen
                            ? { bg: "primary.50", borderColor: "primary.100" }
                            : undefined
                        }
                        _focusWithin={{
                          borderColor: "gold.400",
                          boxShadow: "0 0 0 2px var(--chakra-colors-gold-200)",
                        }}
                      >
                        <Flex
                          gap={3}
                          align={{ base: "stretch", sm: "center" }}
                          justify="space-between"
                          direction={{ base: "column", sm: "row" }}
                        >
                          <Box
                            as="button"
                            type="button"
                            minW={0}
                            flex={1}
                            minH="44px"
                            px={1}
                            textAlign="left"
                            cursor={canOpen ? "pointer" : "default"}
                            disabled={!canOpen}
                            aria-pressed={selected}
                            aria-label={
                              canOpen
                                ? `查看${field.display_name}的原文证据`
                                : `${field.display_name}暂无可查看证据`
                            }
                            onClick={() =>
                              canOpen &&
                              onEvidence(
                                `${field.display_name}：${value.display_value}`,
                                value.evidences,
                                value,
                              )
                            }
                            _focusVisible={{ outline: "none" }}
                          >
                            <HStack spacing={2} align="flex-start">
                              {selected && (
                                <Box
                                  mt="7px"
                                  w="7px"
                                  h="7px"
                                  flexShrink={0}
                                  borderRadius="full"
                                  bg="workbench.evidence"
                                />
                              )}
                              <Box minW={0}>
                                {valueMatrix ? (
                                  <ThemedTable
                                    matrix={valueMatrix}
                                    maxH="360px"
                                  />
                                ) : field.value_type === "table" &&
                                  (value.derived_tables?.length ?? 0) > 0 ? (
                                  <Text
                                    fontSize="sm"
                                    color="workbench.text"
                                    lineHeight="1.7"
                                  >
                                    {tableValueSummary(value)}
                                  </Text>
                                ) : (
                                  <MarkdownContent
                                    content={value.display_value}
                                  />
                                )}
                                <HStack
                                  mt={1}
                                  color="workbench.muted"
                                  fontSize="xs"
                                  wrap="wrap"
                                >
                                  <Text>
                                    {value.origin === "user"
                                      ? "人工值"
                                      : "AI 提取"}
                                  </Text>
                                  <Text aria-hidden>·</Text>
                                  <Text>
                                    {value.evidence_count ||
                                      value.evidences.length}{" "}
                                    处证据
                                  </Text>
                                  {value.needs_evidence && (
                                    <Badge colorScheme="orange">
                                      待绑定证据
                                    </Badge>
                                  )}
                                </HStack>
                              </Box>
                            </HStack>
                          </Box>
                          <HStack
                            flexShrink={0}
                            alignSelf={{ base: "flex-end", sm: "center" }}
                          >
                            <Button
                              size="sm"
                              minH="44px"
                              leftIcon={<FiLink2 />}
                              variant="outline"
                              isDisabled={!canOpen}
                              onClick={() =>
                                onEvidence(
                                  `${field.display_name}：${value.display_value}`,
                                  value.evidences,
                                  value,
                                )
                              }
                              transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1), transform 100ms cubic-bezier(0.16,1,0.3,1)"
                            >
                              查看来源
                            </Button>
                            <IconButton
                              aria-label={`修订${field.display_name}`}
                              icon={<FiEdit3 />}
                              size="sm"
                              minW="44px"
                              minH="44px"
                              variant="ghost"
                              onClick={() => onEdit(value)}
                            />
                          </HStack>
                        </Flex>
                        {/* 字段值本身已是表格时不再重复渲染原表，避免同屏出现两张表 */}
                        {!valueMatrix && (
                          <DerivedTables
                            tables={value.derived_tables || []}
                          />
                        )}
                      </Box>
                    );
                        })}
                        </VStack>
                        {!aiHidden &&
                          !hiddenFieldInterpretationIds.has(field.id) &&
                          field.values.length === 0 &&
                          fieldInterpretationText(field) && (
                            <AiInterpretationNote
                              text={fieldInterpretationText(field)}
                              primary
                              onHide={() =>
                                toggleFieldInterpretation(field.id)
                              }
                            />
                          )}
                        {!aiHidden &&
                          !hiddenFieldInterpretationIds.has(field.id) &&
                          field.values.length > 0 &&
                          fieldInterpretationText(field) && (
                            <AiInterpretationNote
                              text={fieldInterpretationText(field)}
                              onHide={() =>
                                toggleFieldInterpretation(field.id)
                              }
                            />
                          )}
                        {!aiHidden &&
                          hiddenFieldInterpretationIds.has(field.id) &&
                          fieldInterpretationText(field) && (
                            <Button
                              mt={2}
                              size="xs"
                              minH="32px"
                              variant="ghost"
                              leftIcon={<FiEye />}
                              onClick={() =>
                                toggleFieldInterpretation(field.id)
                              }
                              _focusVisible={{
                                boxShadow:
                                  "0 0 0 3px var(--chakra-colors-gold-300)",
                              }}
                            >
                              展示AI解读
                            </Button>
                          )}
                      </Box>
                    )}
                  </Box>
                );
              })}
            </VStack>

            {!visibleFields.length && (
              <Flex
                minH="180px"
                px={5}
                py={8}
                direction="column"
                align="center"
                justify="center"
                textAlign="center"
              >
                <Icon as={FiSearch} boxSize={6} color="neutral.400" />
                <Text mt={3} fontWeight="700">
                  该分类暂无已提取字段
                </Text>
                <Text mt={1} fontSize="sm" color="workbench.muted">
                  可在下方展开待核验字段，结合原文继续确认。
                </Text>
              </Flex>
            )}
          </DataSurface>
        </MotionBox>
      </AnimatePresence>

      {notFoundFields.length > 0 && (
        <Accordion mt={4} allowToggle>
          <AccordionItem
            border="1px solid"
            borderColor="warning.200"
            borderRadius="12px"
            bg="warning.50"
            overflow="hidden"
          >
            <AccordionButton
              minH="52px"
              _focusVisible={{
                boxShadow: "inset 0 0 0 2px var(--chakra-colors-gold-400)",
              }}
            >
              <Box flex={1} textAlign="left">
                <Text fontWeight="720">
                  待核验字段
                  <Text as="span" ml={2} color="warning.700" fontFamily="mono">
                    {notFoundFields.length}
                  </Text>
                </Text>
              </Box>
              <AccordionIcon />
            </AccordionButton>
            <AccordionPanel borderTop="1px solid" borderColor="warning.200">
              <Text fontSize="sm" color="warning.800" lineHeight="1.75">
                这些固定字段在全文范围内没有找到可信取值，可结合原文进一步核验。
              </Text>
              <Flex mt={3} gap={2} wrap="wrap">
                {notFoundFields.map((field) => (
                  <Badge
                    key={field.id}
                    px={2.5}
                    py={1.5}
                    colorScheme="orange"
                    variant="subtle"
                  >
                    {field.display_name}
                  </Badge>
                ))}
              </Flex>
            </AccordionPanel>
          </AccordionItem>
        </Accordion>
      )}
    </Box>
  );
}

/**
 * 主题化表格：深海军蓝标题行 + 斑马纹数据行 + 粘性表头。
 * 评分标准、业绩、分标段等表格型字段值共用同一套呈现。
 */
function ThemedTable({
  matrix,
  maxH = "440px",
}: {
  matrix: TableMatrix;
  maxH?: string;
}) {
  return (
    <TableContainer maxH={maxH} overflowY="auto">
      <Table size="sm" variant="simple" minW="max-content">
        <Thead>
          <Tr>
            {matrix.columns.map((column, columnIndex) => (
              <Th
                key={columnIndex}
                position="sticky"
                top={0}
                zIndex={1}
                bg="workbench.control"
                color="workbench.paper"
                borderColor="workbench.controlRaised"
                fontSize="xs"
                fontWeight="600"
                letterSpacing="0.02em"
                py={2.5}
                whiteSpace="normal"
                minW="120px"
                verticalAlign="middle"
              >
                {column}
              </Th>
            ))}
          </Tr>
        </Thead>
        <Tbody>
          {matrix.rows.map((row, rowIndex) => {
            // 合并单元格在解析后常被复制到每一列：整行同值时渲染为跨列分组标题
            const firstCell = cleanTableCell(row[0]);
            const mergedGroup =
              firstCell !== "" &&
              row.every((cell) => cleanTableCell(cell) === firstCell);
            if (mergedGroup) {
              return (
                <Tr
                  key={rowIndex}
                  bg="neutral.100"
                  _hover={{ bg: "neutral.200" }}
                >
                  <Td
                    colSpan={matrix.columns.length}
                    fontSize="sm"
                    fontWeight="600"
                    color="workbench.text"
                    borderColor="workbench.line"
                    whiteSpace="normal"
                  >
                    {firstCell}
                  </Td>
                </Tr>
              );
            }
            return (
              <Tr
                key={rowIndex}
                bg={rowIndex % 2 === 1 ? "neutral.50" : "white"}
                _hover={{ bg: "primary.50" }}
              >
                {row.map((cell, cellIndex) => (
                  <Td
                    key={`${rowIndex}-${cellIndex}`}
                    whiteSpace="normal"
                    minW="120px"
                    fontSize="sm"
                    lineHeight="1.7"
                    verticalAlign="top"
                    borderColor="workbench.line"
                    color="workbench.text"
                  >
                    {cleanTableCell(cell)}
                  </Td>
                ))}
              </Tr>
            );
          })}
        </Tbody>
      </Table>
    </TableContainer>
  );
}

function DerivedTables({ tables }: { tables: DerivedTable[] }) {
  if (!tables.length) return null;
  return (
    <VStack mt={3} align="stretch" spacing={3}>
      {tables.map((table) => {
        const matrix = tableMatrix(table.data);
        return (
          <Box
            key={table.id}
            bg="white"
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="9px"
            overflow="hidden"
          >
            <Flex
              px={3}
              py={2}
              justify="space-between"
              align="center"
              gap={3}
              bg="neutral.50"
              borderBottom="1px solid"
              borderColor="workbench.line"
            >
              <Text fontSize="sm" fontWeight="700" color="workbench.text">
                {table.title || "归并表格"}
              </Text>
              <Text fontSize="xs" color="workbench.muted" flexShrink={0}>
                {matrix.rows.length} 行 · {matrix.columns.length} 列 ·{" "}
                {table.source_count} 处来源
              </Text>
            </Flex>
            <ThemedTable matrix={matrix} />
          </Box>
        );
      })}
    </VStack>
  );
}

function AiInterpretationNote({
  text,
  primary = false,
  onHide,
}: {
  text: string;
  primary?: boolean;
  onHide?: () => void;
}) {
  if (primary) {
    return (
      <Box
        mt={3}
        p={2.5}
        borderRadius="10px"
        bg="gold.50"
        border="1px solid"
        borderColor="gold.200"
      >
        <HStack spacing={2} align="flex-start" justify="space-between">
          <HStack spacing={2} align="flex-start">
            <Badge colorScheme="gold" variant="subtle" flexShrink={0}>
              AI 提炼 · 无原文值
            </Badge>
            <Text
              fontSize="sm"
              fontWeight="670"
              overflowWrap="anywhere"
              color="workbench.text"
              lineHeight="1.75"
            >
              {text}
            </Text>
          </HStack>
          {onHide && (
            <Button
              flexShrink={0}
              size="xs"
              minH="32px"
              variant="ghost"
              leftIcon={<FiEyeOff />}
              onClick={onHide}
              _focusVisible={{
                boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
              }}
            >
              隐藏
            </Button>
          )}
        </HStack>
        <Text mt={1.5} fontSize="xs" color="workbench.muted">
          AI 基于全文理解给出的判断值，请结合原文核验
        </Text>
      </Box>
    );
  }
  return (
    <Box
      mt={2}
      p={2.5}
      borderRadius="10px"
      bg="gold.50"
      border="1px solid"
      borderColor="gold.100"
    >
      <HStack spacing={2} justify="space-between">
        <HStack spacing={2}>
          <Badge colorScheme="gold" variant="subtle" flexShrink={0}>
            AI 解读
          </Badge>
          <Text fontSize="xs" color="workbench.muted">
            基于招标文档理解
          </Text>
        </HStack>
        {onHide && (
          <Button
            flexShrink={0}
            size="xs"
            minH="32px"
            variant="ghost"
            leftIcon={<FiEyeOff />}
            onClick={onHide}
            _focusVisible={{
              boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
            }}
          >
            隐藏
          </Button>
        )}
      </HStack>
      <Text
        mt={1.5}
        fontSize="sm"
        color="neutral.600"
        lineHeight="1.75"
        whiteSpace="pre-wrap"
      >
        {text}
      </Text>
    </Box>
  );
}

// CJK 单元格清洗：OCR/Docling 常在汉字之间插入空格（序 号），
// 展示前去掉汉字之间的空隙，保留英文与数字之间的正常空格。
type TableMatrix = { columns: string[]; rows: unknown[][] };

function cleanTableCell(value: unknown): string {
  const text = String(value ?? "").trim();
  if (!text) return "";
  return text.replace(
    /([\u3400-\u4dbf\u4e00-\u9fff])\s+(?=[\u3400-\u4dbf\u4e00-\u9fff])/g,
    "$1",
  );
}

// 表格型字段值是否已经包含原格式的 Markdown 表格（含折叠成一行的情况）
function containsMarkdownTable(text: string): boolean {
  const value = String(text ?? "");
  if (!value.includes("|")) return false;
  return /\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)+\|?/.test(value);
}

// 表格型字段的字段值只用一句说明代替原格式字符串：
// 完整表格由下方 DerivedTables 渲染，避免同一份内容重复出现。
function tableValueSummary(value: {
  derived_tables?: Array<{ row_count?: number; column_count?: number }>;
}): string {
  const table = value?.derived_tables?.[0];
  const rows = Number(table?.row_count || 0);
  if (rows > 0) {
    return `表格型数据 · 共 ${rows} 行，完整内容见下方表格。`;
  }
  return "表格型数据 · 完整内容见下方表格。";
}

function isSeparatorCells(cells: string[]): boolean {
  return (
    cells.length > 0 &&
    cells.every((cell) => /^:?-+:?$/.test(cell.replace(/\s/g, "")))
  );
}

function splitMarkdownRow(chunk: string): string[] {
  let text = chunk.trim();
  if (text.startsWith("|")) text = text.slice(1);
  if (text.endsWith("|")) text = text.slice(0, -1);
  if (!text.trim()) return [];
  return text.split("|").map((cell) => cell.trim());
}

/**
 * 把 Markdown 表格文本解析成列名 + 数据行。
 * 兼容两种形态：标准逐行表格，以及被模型压缩成一行（行之间是 `| |`）的表格。
 * 解析不出表格时返回 null，由调用方回退到其它渲染方式。
 */
function matrixFromMarkdownTable(text: string): TableMatrix | null {
  const raw = String(text ?? "").trim();
  if (!raw || !raw.includes("|")) return null;

  let rows = raw
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.includes("|"))
    .map(splitMarkdownRow)
    .filter((cells) => cells.length > 0);
  let separatorIndex = rows.findIndex(isSeparatorCells);

  if (separatorIndex <= 0) {
    rows = raw
      .replace(/\n/g, "")
      .split(/\|\s*\|/)
      .map(splitMarkdownRow)
      .filter((cells) => cells.length > 0);
    separatorIndex = rows.findIndex(isSeparatorCells);
  }
  if (separatorIndex <= 0) return null;

  const columns = rows[separatorIndex].length;
  const header = rows[separatorIndex - 1];
  if (columns < 2 || header.length !== columns) return null;

  const body: string[][] = [];
  for (const row of rows.slice(separatorIndex + 1)) {
    // 行尾可能跟随说明文字：列数不足即认为表格结束
    if (row.length < columns) break;
    body.push(row.slice(0, columns));
  }
  if (!body.length) return null;
  return { columns: header, rows: body };
}

// 表格型字段的 AI 解读：兼容历史数据里原格式表格 + 说明的旧格式，
// 只保留说明部分，避免把表格字符串再渲染一遍。
function fieldInterpretationText(field: { value_type?: string; ai_interpretation?: string }): string {
  const raw = String(field?.ai_interpretation || "");
  if (field?.value_type !== "table") return raw;
  const marker = raw.lastIndexOf("说明：");
  if (marker >= 0) return raw.slice(marker + "说明：".length).trim();
  if (containsMarkdownTable(raw)) return "";
  return raw.trim();
}

function tableMatrix(data: unknown): TableMatrix {
  const source =
    data &&
    typeof data === "object" &&
    !Array.isArray(data) &&
    Array.isArray((data as any).rows)
      ? (data as any).rows
      : data;
  if (!Array.isArray(source) || source.length === 0)
    return { columns: ["内容"], rows: [["暂无表格数据"]] };
  if (source.every((row) => Array.isArray(row))) {
    const rows = source as unknown[][];
    // 表格数据首行是列名：作为表头渲染，不再重复出现在数据行里
    const header = rows[0] ?? [];
    // 列宽取表头与数据行的较大值：解析结果会把换行单元格拆成额外列，
    // 直接按表头截断会丢掉分值等真实数据，因此保留列但只表头覆盖到的列才有列名。
    const width = Math.max(
      header.length,
      ...rows.map((row) => row.length),
    );
    return {
      columns: Array.from(
        { length: width },
        (_, index) => cleanTableCell(header[index]),
      ),
      rows: rows
        .slice(1)
        .map((row) =>
          Array.from({ length: width }, (_, index) => row[index] ?? ""),
        ),
    };
  }
  const columns = Array.from(
    new Set(
      source.flatMap((row) =>
        row && typeof row === "object" ? Object.keys(row) : [],
      ),
    ),
  );
  return {
    columns: columns.length ? columns : ["内容"],
    rows: source.map((row) =>
      row && typeof row === "object"
        ? columns.map((column) => (row as any)[column])
        : [row],
    ),
  };
}

export function ClausesPanel({
  chapters,
  activeChapterId,
  onChapter,
  activeClauseId,
  highlightedClauseId,
  sourceOpen,
  onEvidence,
  onOpenFields,
  onOpenSource,
  followedClauseIds = new Set<number>(),
  onToggleFollow,
  followingKey = null,
}: {
  chapters: Chapter[];
  activeChapterId?: number;
  onChapter?: (chapterId: number) => void;
  activeClauseId?: number;
  highlightedClauseId?: number;
  sourceOpen: boolean;
  onEvidence: (clause: Clause) => void;
  onOpenFields: () => void;
  onOpenSource: () => void;
  followedClauseIds?: Set<number>;
  onToggleFollow?: (clause: Clause) => void;
  followingKey?: string | null;
}) {
  const [aiHidden, setAiHidden] = useState(false);
  const [hiddenClauseIds, setHiddenClauseIds] = useState<Set<number>>(
    new Set(),
  );
  const toggleClauseInterpretation = (clauseId: number) => {
    setHiddenClauseIds((current) => {
      const next = new Set(current);
      if (next.has(clauseId)) next.delete(clauseId);
      else next.add(clauseId);
      return next;
    });
  };
  const [collapsedClauseIds, setCollapsedClauseIds] = useState<Set<number>>(
    new Set(),
  );
  const toggleClauseCollapse = (clauseId: number) => {
    setCollapsedClauseIds((current) => {
      const next = new Set(current);
      if (next.has(clauseId)) next.delete(clauseId);
      else next.add(clauseId);
      return next;
    });
  };
  const withClauses = chapters
    .map((chapter) => ({
      ...chapter,
      clauses: (chapter.clauses || []).filter(
        (clause) => !CLAUSE_NOISE_TITLE_RE.test(clause.title),
      ),
    }))
    .filter((chapter) => chapter.clauses.length > 0);
  const hasInterpretations = withClauses.some((chapter) =>
    chapter.clauses.some((clause) => Boolean(clause.ai_interpretation)),
  );
  if (!withClauses.length) {
    return (
      <DataSurface
        minH="290px"
        px={{ base: 5, md: 8 }}
        py={10}
        boxShadow="none"
        display="flex"
        alignItems="center"
        justifyContent="center"
      >
        <Flex direction="column" align="center" maxW="560px" textAlign="center">
          <Flex
            w="52px"
            h="52px"
            borderRadius="14px"
            align="center"
            justify="center"
            bg="primary.50"
            color="primary.600"
          >
            <Icon as={FiSearch} boxSize={6} />
          </Flex>
          <Text mt={5} fontSize="lg" fontWeight="750">
            当前未识别到需单列的关键条款
          </Text>
          <Text mt={2} color="workbench.muted" fontSize="sm" lineHeight="1.8">
            这不等同于确认无约束。建议结合待核验字段和招标原文，继续检查响应要求与否决风险。
          </Text>
          <Flex mt={6} gap={3} wrap="wrap" justify="center">
            <Button minH="44px" colorScheme="primary" onClick={onOpenFields}>
              查看字段
            </Button>
            <Button
              minH="44px"
              variant="outline"
              leftIcon={<FiEye />}
              onClick={onOpenSource}
            >
              查看原文
            </Button>
          </Flex>
        </Flex>
      </DataSurface>
    );
  }

  const activeChapter =
    withClauses.find((chapter) => chapter.id === activeChapterId) ||
    withClauses[0];
  if (!activeChapter) {
    return null;
  }

  return (
    <VStack align="stretch" spacing={3}>
      <Flex gap={2} align="center">
        <Flex
          flex={1}
          minW={0}
          gap={2}
          overflowX="auto"
          pb={1}
          className="thin-scrollbars"
          aria-label="条款章节"
        >
          {withClauses.map((chapter) => {
            const active = activeChapter.id === chapter.id;
            return (
              <Button
                key={chapter.id}
                size="sm"
                minH="44px"
                flexShrink={0}
                border="1px solid"
                borderColor={active ? "workbench.control" : "workbench.line"}
                bg={active ? "workbench.control" : "white"}
                color={active ? "white" : "workbench.muted"}
                aria-pressed={active}
                onClick={() => onChapter?.(chapter.id)}
                transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1), color 160ms cubic-bezier(0.16,1,0.3,1), transform 100ms cubic-bezier(0.16,1,0.3,1)"
                _hover={{
                  bg: active ? "workbench.controlRaised" : "primary.50",
                  borderColor: active
                    ? "workbench.controlRaised"
                    : "primary.200",
                }}
                _active={{ transform: "translateY(1px)" }}
                _focusVisible={{
                  boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
                }}
              >
                {chapter.title}
                <Text
                  as="span"
                  ml={2}
                  minW="20px"
                  px={1}
                  borderRadius="full"
                  bg={active ? "whiteAlpha.200" : "neutral.100"}
                  color={active ? "white" : "neutral.600"}
                  fontFamily="mono"
                  fontSize="xs"
                >
                  {chapter.clauses.length}
                </Text>
              </Button>
            );
          })}
        </Flex>
        <Button
          size="sm"
          minH="40px"
          flexShrink={0}
          variant="outline"
          leftIcon={aiHidden ? <FiEye /> : <FiEyeOff />}
          isDisabled={!hasInterpretations}
          onClick={() => setAiHidden((value) => !value)}
          _focusVisible={{
            boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
          }}
        >
          {aiHidden ? "展示AI解读" : "隐藏AI解读"}
        </Button>
      </Flex>

      <DataSurface p={0} boxShadow="none" overflow="hidden">
        <Flex
          px={{ base: 4, md: 5 }}
          py={3.5}
          justify="space-between"
          gap={3}
          align="center"
          bg="neutral.50"
        >
          <Text fontWeight="750">{activeChapter.title}</Text>
          <HStack spacing={3} flexShrink={0}>
            <Text color="workbench.muted" fontSize="xs" fontFamily="mono">
              P{activeChapter.page_start}–{activeChapter.page_end}
            </Text>
            <Badge variant="subtle" colorScheme="blue" flexShrink={0}>
              已提取 {activeChapter.clauses.length}
            </Badge>
          </HStack>
        </Flex>
        <VStack align="stretch" spacing={2.5} p={{ base: 3, md: 4 }}>
          {activeChapter.clauses.map((clause, index) => {
            const collapsed = collapsedClauseIds.has(clause.id);
            const canOpen = Boolean(clause.evidences?.length);
            const selected = sourceOpen && activeClauseId === clause.id;
            const highlighted = highlightedClauseId === clause.id;
            const tone =
              clause.importance === "critical"
                ? { scheme: "red", label: "重点", icon: FiAlertTriangle }
                : clause.importance === "high"
                  ? { scheme: "orange", label: "重要", icon: FiAlertTriangle }
                  : { scheme: "gray", label: "一般", icon: FiFileText };
            return (
              <Box
                id={`clause-${clause.id}`}
                key={clause.id}
                scrollMarginTop="92px"
                border="1px solid"
                borderColor={
                  highlighted
                    ? "gold.300"
                    : selected
                      ? "primary.200"
                      : "workbench.line"
                }
                bg={
                  selected ? "primary.50" : highlighted ? "gold.50" : "white"
                }
                boxShadow={
                  index % 2 === 0 ? CARD_SHADOW_EVEN : CARD_SHADOW_ODD
                }
                borderRadius="12px"
                overflow="hidden"
                transition="background-color 160ms cubic-bezier(0.16,1,0.3,1), border-color 160ms cubic-bezier(0.16,1,0.3,1)"
                _hover={{
                  bg: selected
                    ? "primary.50"
                    : highlighted
                      ? "gold.50"
                      : "neutral.50",
                  borderColor: highlighted ? "gold.300" : "primary.200",
                }}
              >
                <Flex
                  gap={3}
                  align={{ base: "stretch", md: "center" }}
                  direction={{ base: "column", md: "row" }}
                  px={{ base: 4, md: 5 }}
                  py={3.5}
                >
                  <Box
                    as="button"
                    type="button"
                    flex={1}
                    minW={0}
                    minH="44px"
                    textAlign="left"
                    cursor={canOpen ? "pointer" : "default"}
                    disabled={!canOpen}
                    aria-pressed={selected}
                    onClick={() => canOpen && onEvidence(clause)}
                    _focusVisible={{
                      outline: "2px solid var(--chakra-colors-gold-400)",
                      outlineOffset: "3px",
                    }}
                  >
                    <HStack spacing={2} wrap="wrap">
                      <Text fontWeight="720">{clause.title}</Text>
                      <Badge
                        colorScheme={tone.scheme}
                        display="inline-flex"
                        alignItems="center"
                        gap={1}
                      >
                        <Icon as={tone.icon} boxSize={3} />
                        {tone.label}
                      </Badge>
                    </HStack>
                  </Box>
                  <HStack flexShrink={0} spacing={2}>
                    <IconButton
                      aria-label={collapsed ? "展开条款" : "折叠条款"}
                      icon={
                        <Icon
                          as={FiChevronDown}
                          transform={collapsed ? "rotate(-90deg)" : undefined}
                          transition="transform 160ms cubic-bezier(0.16,1,0.3,1)"
                        />
                      }
                      size="sm"
                      minW="44px"
                      minH="44px"
                      variant="ghost"
                      color="workbench.muted"
                      onClick={() => toggleClauseCollapse(clause.id)}
                    />
                    <IconButton
                      aria-label={
                        followedClauseIds.has(clause.id)
                          ? "取消关注"
                          : "添加关注"
                      }
                      icon={<FiStar />}
                      size="sm"
                      minW="44px"
                      minH="44px"
                      variant="ghost"
                      color={
                        followedClauseIds.has(clause.id)
                          ? "gold.500"
                          : "workbench.muted"
                      }
                      isLoading={followingKey === `clause:${clause.id}`}
                      onClick={() => onToggleFollow?.(clause)}
                    />
                    <Button
                      flexShrink={0}
                      size="sm"
                      minH="44px"
                      leftIcon={<FiLink2 />}
                      variant="outline"
                      isDisabled={!canOpen}
                      onClick={() => onEvidence(clause)}
                    >
                      查看来源
                    </Button>
                  </HStack>
                </Flex>
                {!collapsed && (
                  <Box
                    px={{ base: 4, md: 5 }}
                    pb={4}
                    borderTop="1px solid"
                    borderColor="workbench.line"
                  >
                    <Box mt={3}>
                      <MarkdownContent content={clause.content} />
                    </Box>
                    {!aiHidden &&
                      !hiddenClauseIds.has(clause.id) &&
                      clause.ai_interpretation && (
                        <Flex
                          mt={3}
                          p={2.5}
                          gap={3}
                          align={{ base: "stretch", md: "center" }}
                          direction={{ base: "column", md: "row" }}
                          borderRadius="10px"
                          bg="gold.50"
                          border="1px solid"
                          borderColor="gold.100"
                        >
                          <Box flex={1} minW={0}>
                            <HStack spacing={2}>
                              <Badge
                                colorScheme="gold"
                                variant="subtle"
                                flexShrink={0}
                              >
                                AI 解读
                              </Badge>
                              <Text fontSize="xs" color="workbench.muted">
                                基于招标文档理解
                              </Text>
                            </HStack>
                            <Text
                              mt={1.5}
                              fontSize="sm"
                              color="neutral.600"
                              lineHeight="1.75"
                              whiteSpace="pre-wrap"
                            >
                              {clause.ai_interpretation}
                            </Text>
                          </Box>
                          <Button
                            flexShrink={0}
                            size="xs"
                            minH="32px"
                            variant="ghost"
                            leftIcon={<FiEyeOff />}
                            onClick={() =>
                              toggleClauseInterpretation(clause.id)
                            }
                            _focusVisible={{
                              boxShadow:
                                "0 0 0 3px var(--chakra-colors-gold-300)",
                            }}
                          >
                            隐藏
                          </Button>
                        </Flex>
                      )}
                    {!aiHidden &&
                      hiddenClauseIds.has(clause.id) &&
                      clause.ai_interpretation && (
                        <Button
                          mt={2}
                          size="xs"
                          minH="32px"
                          variant="ghost"
                          leftIcon={<FiEye />}
                          onClick={() => toggleClauseInterpretation(clause.id)}
                          _focusVisible={{
                            boxShadow:
                              "0 0 0 3px var(--chakra-colors-gold-300)",
                          }}
                        >
                          展示AI解读
                        </Button>
                      )}
                  </Box>
                )}
              </Box>
            );
          })}
        </VStack>
      </DataSurface>
    </VStack>
  );
}

export function BlueprintPanel({
  projectId,
  data,
  loading,
  loadError,
  mutate,
  mutating,
  reload,
  onOpenClause,
  onOpenEvidence,
}: BlueprintPanelProps) {
  const router = useRouter();
  const toast = useToast();
  const reducedMotion = useReducedMotion();
  const [addParent, setAddParent] = useState(0);
  const [addTitle, setAddTitle] = useState("");
  const [creatingBid, setCreatingBid] = useState(false);
  const addDialog = useDisclosure();
  const regenDialog = useDisclosure();

  const action = async (url: string, method?: string, payload?: any) => {
    try {
      await mutate({ url, method: method || "POST", data: payload });
      await reload();
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "操作失败",
      });
    }
  };

  const saveNode = async (
    node: V3BlueprintNode,
    nextTitle = node.title,
    parentId = node.parent_id,
    sortOrder = node.sort_order,
  ) => {
    await action(`/zb/v3/blueprint-nodes/${node.id}`, "PATCH", {
      title: nextTitle,
      parent_id: parentId,
      sort_order: sortOrder,
    });
  };

  const addNode = async () => {
    if (!addTitle.trim()) return;
    await action(`/zb/v3/projects/${projectId}/blueprint/nodes`, "POST", {
      title: addTitle.trim(),
      parent_id: addParent,
      sort_order: (data?.nodes?.length || 0) + 1,
    });
    addDialog.onClose();
    setAddTitle("");
  };

  const createBid = async () => {
    if (creatingBid) return;
    setCreatingBid(true);
    try {
      const response = await mutate({
        url: `/zb/v3/projects/${projectId}/blueprint/create-bid`,
        method: "POST",
      });
      const bidId = response?.data?.data?.id;
      if (!bidId) {
        setCreatingBid(false);
        toast({ status: "error", title: "创建投标书失败，请重试" });
        return;
      }
      // 创建接口本身很快（约百毫秒），标书工作台首屏还要等路由与数据就绪。
      // 这里不复位 loading，让按钮一直保持“创建中”直到页面切换完成。
      router.push(`/file-gen/${bidId}`);
    } catch (error: any) {
      setCreatingBid(false);
      toast({
        status: "error",
        title: error?.response?.data?.msg || "创建投标书失败",
      });
    }
  };

  const regenerateBlueprint = async () => {
    regenDialog.onClose();
    await action(`/zb/v3/projects/${projectId}/blueprint`, "POST", {
      regenerate: true,
    });
  };

  if (loading) {
    return (
      <DataSurface p={{ base: 5, md: 7 }} boxShadow="none">
        <Skeleton h="24px" w="180px" borderRadius="6px" />
        <SkeletonText mt={6} noOfLines={6} spacing={4} />
      </DataSurface>
    );
  }

  if (loadError) {
    return (
      <DataSurface
        p={{ base: 6, md: 8 }}
        bg="error.50"
        borderColor="error.200"
        boxShadow="none"
      >
        <HStack>
          <Icon as={FiAlertTriangle} color="error.600" />
          <Text fontWeight="750">蓝图状态读取失败</Text>
        </HStack>
        <Text mt={3} color="error.800" fontSize="sm" lineHeight="1.75">
          {loadError}
        </Text>
        <Button
          mt={5}
          minH="44px"
          leftIcon={<FiRefreshCw />}
          colorScheme="red"
          variant="outline"
          onClick={reload}
        >
          重新加载
        </Button>
      </DataSurface>
    );
  }

  if (
    !data ||
    data.status === "not_generated" ||
    data.status === "invalidated"
  ) {
    return (
      <DataSurface p={{ base: 6, md: 9 }} textAlign="center" boxShadow="none">
        <Flex
          mx="auto"
          w="56px"
          h="56px"
          align="center"
          justify="center"
          borderRadius="16px"
          bg="primary.50"
          color="primary.600"
        >
          <Icon as={FiBookOpen} boxSize={7} />
        </Flex>
        <Text mt={5} fontSize="xl" fontWeight="760">
          生成投标书大纲
        </Text>
        <Text
          mt={2}
          maxW="620px"
          mx="auto"
          color="workbench.muted"
          fontSize="sm"
          lineHeight="1.8"
        >
          基于招标文件解析结果，自动生成规范标书目录。<br/>硬性章节与必备材料一目了然，AI 补充项可按需采纳或移除。
        </Text>
        <Button
          mt={6}
          minH="48px"
          bg="workbench.control"
          color="white"
          leftIcon={<FiBookOpen />}
          isLoading={mutating}
          onClick={() => action(`/zb/v3/projects/${projectId}/blueprint`)}
          _hover={{ bg: "workbench.controlRaised" }}
          _focusVisible={{
            boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
          }}
        >
          生成标书蓝图
        </Button>
      </DataSurface>
    );
  }

  if (data.status === "pending" || data.status === "running") {
    return (
      <DataSurface p={{ base: 5, md: 7 }} boxShadow="none">
        <HStack>
          <MotionBox
            w="10px"
            h="10px"
            borderRadius="full"
            bg="gold.400"
            animate={reducedMotion ? undefined : { opacity: [0.35, 1, 0.35] }}
            transition={
              reducedMotion ? undefined : { repeat: Infinity, duration: 1.5 }
            }
          />
          <Text fontWeight="740" aria-live="polite">
            正在生成标书蓝图
          </Text>
        </HStack>
        <Text mt={2} color="workbench.muted" fontSize="sm">
          可以离开当前页面，生成会在后台继续。
        </Text>
        <VStack mt={6} align="stretch" spacing={3}>
          {[70, 92, 78, 86, 64].map((width, index) => (
            <Skeleton
              key={width}
              ml={`${(index % 3) * 18}px`}
              h="42px"
              w={`${width}%`}
              borderRadius="8px"
            />
          ))}
        </VStack>
      </DataSurface>
    );
  }

  if (data.status === "failed") {
    return (
      <DataSurface
        p={{ base: 6, md: 8 }}
        bg="error.50"
        borderColor="error.200"
        boxShadow="none"
      >
        <Flex align="flex-start" gap={3}>
          <Flex
            w="42px"
            h="42px"
            flexShrink={0}
            borderRadius="12px"
            align="center"
            justify="center"
            bg="error.100"
            color="error.600"
          >
            <Icon as={FiAlertTriangle} boxSize={5} />
          </Flex>
          <Box>
            <Text fontWeight="750">蓝图生成失败</Text>
            <Text mt={2} color="error.800" fontSize="sm" lineHeight="1.75">
              {data.generation?.last_error ||
                "生成结果未通过校验，本次失败不影响招标解析结果。"}
            </Text>
          </Box>
        </Flex>
        <Button
          mt={5}
          minH="44px"
          leftIcon={<FiRefreshCw />}
          colorScheme="red"
          variant="outline"
          isLoading={mutating}
          onClick={() => action(`/zb/v3/projects/${projectId}/blueprint/retry`)}
        >
          重新尝试
        </Button>
      </DataSurface>
    );
  }

  const nodes = data.nodes || [];
  const associatedBidId = data.generation?.associated_bid_project_id || 0;
  return (
    <Box>
      <Flex
        mb={4}
        gap={3}
        align={{ base: "stretch", md: "center" }}
        justify="space-between"
        direction={{ base: "column", md: "row" }}
      >
        <Box>
          <Text fontSize="lg" fontWeight="760">
            投标书大纲
          </Text>
          <Text mt={1} color="workbench.muted" fontSize="sm">
            按招标要求生成大纲，AI补充项可自由取舍。
          </Text>
        </Box>
        <HStack alignSelf={{ base: "stretch", md: "auto" }}>
          <Button
            minH="44px"
            leftIcon={<FiPlus />}
            variant="outline"
            isDisabled={associatedBidId > 0}
            onClick={() => {
              setAddParent(0);
              addDialog.onOpen();
            }}
          >
            新增章节
          </Button>
          <Button
            minH="44px"
            leftIcon={<FiRefreshCw />}
            variant="outline"
            isDisabled={associatedBidId > 0 || mutating}
            onClick={regenDialog.onOpen}
          >
            重新生成
          </Button>
          <Button
            minH="44px"
            /* 与默认文案同宽，避免 loading 文案切换时挤动左侧按钮 */
            minW="8.5rem"
            bg="workbench.control"
            color="white"
            leftIcon={<FiFileText />}
            onClick={
              associatedBidId > 0
                ? () => router.push(`/file-gen/${associatedBidId}`)
                : createBid
            }
            /* loading 态：标签保持可读，且不套用禁用态的降透明度 */
            isLoading={creatingBid || mutating}
            loadingText={creatingBid ? "创建中…" : undefined}
            aria-busy={creatingBid || undefined}
            _disabled={{
              opacity: creatingBid ? 1 : 0.4,
              cursor: creatingBid ? "progress" : "not-allowed",
            }}
            _hover={{ bg: "workbench.controlRaised" }}
          >
            {associatedBidId > 0 ? "进入投标书" : "生成投标书"}
          </Button>
        </HStack>
      </Flex>

      <BlueprintTree
        nodes={nodes}
        actionsDisabled={associatedBidId > 0}
        mutating={mutating}
        onSaveTitle={async (node, nextTitle) => saveNode(node, nextTitle)}
        onAddChild={(parentId) => {
          setAddParent(parentId);
          addDialog.onOpen();
        }}
        onMove={async (nodeId, direction) =>
          action(`/zb/v3/blueprint-nodes/${nodeId}/move`, "POST", { direction })
        }
        onAdopt={async (nodeId) =>
          action(`/zb/v3/blueprint-nodes/${nodeId}/adopt`)
        }
        onRemove={async (nodeId) =>
          action(`/zb/v3/blueprint-nodes/${nodeId}/remove`)
        }
        onDelete={async (nodeId) =>
          action(`/zb/v3/blueprint-nodes/${nodeId}`, "DELETE")
        }
        onOpenClause={onOpenClause}
        onOpenEvidence={onOpenEvidence}
      />

      <Modal isOpen={addDialog.isOpen} onClose={addDialog.onClose} isCentered>
        <ModalOverlay />
        <ModalContent mx={3} borderRadius="14px">
          <ModalHeader>{addParent > 0 ? "添加子章节" : "新增章节"}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Input
              minH="48px"
              placeholder="输入章节标题"
              value={addTitle}
              onChange={(event) => setAddTitle(event.target.value)}
              autoFocus
            />
          </ModalBody>
          <ModalFooter gap={2}>
            <Button onClick={addDialog.onClose}>取消</Button>
            <Button
              colorScheme="primary"
              leftIcon={<FiPlus />}
              isLoading={mutating}
              onClick={addNode}
            >
              添加
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal isOpen={regenDialog.isOpen} onClose={regenDialog.onClose} isCentered>
        <ModalOverlay />
        <ModalContent mx={3} borderRadius="14px">
          <ModalHeader>重新生成标书蓝图</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Text fontSize="sm" lineHeight="1.8" color="workbench.text">
              将按最新规则重新整理大纲（一级“第一章/第二章…”，二级“1.1/1.2…”，并补充二级、三级 AI
              建议标题）。当前蓝图中的手动调整（新增章节、采纳/移除建议）会被重置。
            </Text>
            {associatedBidId > 0 && (
              <Text mt={2} fontSize="sm" color="error.700">
                该蓝图已用于创建投标书，不能重新生成。
              </Text>
            )}
          </ModalBody>
          <ModalFooter gap={2}>
            <Button onClick={regenDialog.onClose}>取消</Button>
            <Button
              colorScheme="primary"
              leftIcon={<FiRefreshCw />}
              isLoading={mutating}
              isDisabled={associatedBidId > 0}
              onClick={regenerateBlueprint}
            >
              重新生成
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </Box>
  );
}
