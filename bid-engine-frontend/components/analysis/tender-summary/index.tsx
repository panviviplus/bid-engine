/* Hallmark · component: summary-qualification-list · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * contrast: pass
 */

"use client";

import React, { useState } from "react";
import { Box, Flex, Text, SimpleGrid, IconButton, Collapse, VStack, Badge } from "@chakra-ui/react";
import { FiChevronDown, FiChevronUp } from "react-icons/fi";

interface KeyDates {
  deadline: string;
  opening: string;
  clarification_deadline: string;
}

export interface TenderSummaryData {
  project_name: string;
  budget_amount: string;
  bid_deadline: string;
  bid_location: string;
  tender_agency: string;
  bidder_qualification_summary: string[] | string;
  key_dates?: KeyDates;
}

interface TenderSummaryProps {
  summary: TenderSummaryData | null;
  summaryStatus: string;
}

const glassCard = {
  bg: "rgba(255, 255, 255, 0.9)",
  backdropFilter: "blur(20px)",
  border: "1px solid rgba(255, 255, 255, 0.5)",
  boxShadow: "0 4px 20px rgba(0, 0, 0, 0.05)",
  borderRadius: "2xl",
  p: 4,
};

export default function TenderSummary({ summary, summaryStatus }: TenderSummaryProps) {
  if (summaryStatus === "pending") {
    return (
      <Box {...glassCard} mb={4}>
        <Flex align="center" gap={2} color="gray.400">
          <Text fontSize="sm">智能摘要即将生成，请稍候...</Text>
        </Flex>
      </Box>
    );
  }

  if (summaryStatus === "running") {
    return (
      <Box {...glassCard} mb={4}>
        <Flex align="center" gap={2} color="gray.400">
          <Text fontSize="sm">智能摘要生成中...</Text>
        </Flex>
      </Box>
    );
  }

  if (summaryStatus === "skipped") {
    return (
      <Box {...glassCard} mb={4} borderLeft="4px solid" borderLeftColor="gray.300">
        <Flex align="center" gap={2}>
          <Text fontSize="sm" color="gray.500">智能摘要已跳过，可在对应数据区域点击"补充解析"重新生成</Text>
        </Flex>
      </Box>
    );
  }

  if (summaryStatus === "failed") {
    return (
      <Box {...glassCard} mb={4} borderLeft="4px solid" borderLeftColor="orange.400">
        <Flex align="center" gap={2}>
          <Text fontSize="sm" color="gray.600">智能摘要生成失败，可重试该阶段或参考详细字段自行判断</Text>
        </Flex>
      </Box>
    );
  }

  if (!summary || summaryStatus === "empty") {
    return (
      <Box {...glassCard} mb={4}>
        <Flex align="center" gap={2} color="gray.400">
          <Text fontSize="sm">智能摘要暂无数据</Text>
        </Flex>
      </Box>
    );
  }

  if (summaryStatus === "error") {
    return (
      <Box {...glassCard} mb={4} borderLeft="4px solid" borderLeftColor="orange.400">
        <Flex align="center" gap={2}>
          <Text fontSize="sm" color="gray.600">智能摘要生成异常，请参考详细字段自行判断</Text>
        </Flex>
      </Box>
    );
  }

  return (
    <Box mb={4}>
      <ProjectSummaryCard summary={summary} defaultExpanded={true} />
      <KeyInfoCard summary={summary} defaultExpanded={true} />
      <KeyDatesDisplay dates={summary.key_dates} defaultExpanded={false} />
    </Box>
  );
}

/* ========== 项目摘要卡片（默认展开） ========== */

function ProjectSummaryCard({ summary, defaultExpanded }: { summary: TenderSummaryData; defaultExpanded: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const qualSummary = summary.bidder_qualification_summary;
  if (!qualSummary || (Array.isArray(qualSummary) && qualSummary.length === 0)) return null;

  const items = Array.isArray(qualSummary) ? qualSummary : (qualSummary ? [qualSummary] : []);

  return (
    <Box {...glassCard} mb={3}>
      <Flex
        justify="space-between"
        align="center"
        mb={expanded ? 3 : 0}
        cursor="pointer"
        onClick={() => setExpanded(!expanded)}
        _hover={{ opacity: 0.85 }}
      >
        <Flex align="center" gap={2}>
          <Text fontSize="sm" fontWeight="bold" color="gray.800">
            项目摘要
          </Text>
          {items.length > 1 && (
            <Badge colorScheme="gray" variant="subtle" fontSize="xs">
              共{items.length}条
            </Badge>
          )}
        </Flex>
        <IconButton
          aria-label={expanded ? "收起" : "展开"}
          icon={expanded ? <FiChevronUp /> : <FiChevronDown />}
          size="xs"
          variant="ghost"
          tabIndex={-1}
        />
      </Flex>
      <Collapse in={expanded}>
        <VStack spacing={2} align="stretch">
          {items.map((item, idx) => (
            <Flex
              key={idx}
              align="start"
              borderLeft="3px solid"
              borderLeftColor="primary.300"
              pl={3}
              py={1}
              gap={2}
            >
              <Box
                w={5}
                h={5}
                flexShrink={0}
                display="flex"
                alignItems="center"
                justifyContent="center"
                bg="primary.50"
                border="1px solid"
                borderColor="primary.200"
                borderRadius="md"
                fontSize="xs"
                fontWeight="semibold"
                color="primary.700"
                lineHeight="1"
                mt="1px"
              >
                {idx + 1}
              </Box>
              <Text fontSize="sm" color="gray.700" lineHeight="tall">
                {item}
              </Text>
            </Flex>
          ))}
        </VStack>
      </Collapse>
    </Box>
  );
}

/* ========== 项目概况卡片（默认展开） ========== */

function KeyInfoCard({ summary, defaultExpanded }: { summary: TenderSummaryData; defaultExpanded: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const items = [
    { label: "项目名称", value: summary.project_name },
    { label: "预算金额", value: summary.budget_amount },
    { label: "投标截止", value: summary.bid_deadline },
    { label: "开标地点", value: summary.bid_location },
    { label: "招标代理", value: summary.tender_agency },
  ];
  const hasData = items.some((it) => it.value);

  if (!hasData) return null;

  return (
    <Box {...glassCard} mb={3}>
      <Flex
        justify="space-between"
        align="center"
        mb={expanded ? 3 : 0}
        cursor="pointer"
        onClick={() => setExpanded(!expanded)}
        _hover={{ opacity: 0.85 }}
      >
        <Text fontSize="sm" fontWeight="bold" color="gray.800">
          项目概况
        </Text>
        <IconButton
          aria-label={expanded ? "收起" : "展开"}
          icon={expanded ? <FiChevronUp /> : <FiChevronDown />}
          size="xs"
          variant="ghost"
          tabIndex={-1}
        />
      </Flex>
      <Collapse in={expanded}>
        <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} spacing={3}>
          {items.map(
            (it, idx) =>
              it.value && (
                <Box key={idx}>
                  <Text fontSize="xs" color="gray.400" mb={0.5}>
                    {it.label}
                  </Text>
                  <Text fontSize="sm" fontWeight="semibold" color="gray.800" noOfLines={2}>
                    {it.value}
                  </Text>
                </Box>
              ),
          )}
        </SimpleGrid>
      </Collapse>
    </Box>
  );
}

/* ========== 关键日期（默认折叠） ========== */

function KeyDatesDisplay({ dates, defaultExpanded }: { dates?: KeyDates; defaultExpanded: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const dateItems = [
    { label: "投标截止", value: dates?.deadline },
    { label: "开标日期", value: dates?.opening },
    { label: "澄清截止", value: dates?.clarification_deadline },
  ];
  const hasData = dateItems.some((it) => it.value);

  if (!hasData) return null;

  return (
    <Box {...glassCard} mb={3}>
      <Flex
        justify="space-between"
        align="center"
        mb={expanded ? 3 : 0}
        cursor="pointer"
        onClick={() => setExpanded(!expanded)}
        _hover={{ opacity: 0.85 }}
      >
        <Text fontSize="sm" fontWeight="bold" color="gray.800">
          关键日期
        </Text>
        <IconButton
          aria-label={expanded ? "收起" : "展开"}
          icon={expanded ? <FiChevronUp /> : <FiChevronDown />}
          size="xs"
          variant="ghost"
          tabIndex={-1}
        />
      </Flex>
      <Collapse in={expanded}>
        <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3}>
          {dateItems.map(
            (it, idx) =>
              it.value && (
                <Box key={idx}>
                  <Text fontSize="xs" color="gray.400" mb={0.5}>
                    {it.label}
                  </Text>
                  <Text fontSize="sm" fontWeight="semibold" color="gray.800" fontFamily="mono">
                    {it.value}
                  </Text>
                </Box>
              ),
          )}
        </SimpleGrid>
      </Collapse>
    </Box>
  );
}
