"use client";

/* Hallmark · component: intel-notice-card · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * states: default · hover · focus-visible · active · disabled(收藏中) · loading · empty · error
 * slop: 无虚构指标；预算缺失时显示“预算未公示”而不是 0
 */
import React from "react";
import {
  Badge,
  Box,
  Button,
  Checkbox,
  Divider,
  Flex,
  HStack,
  IconButton,
  Stack,
  Tag,
  Text,
} from "@chakra-ui/react";
import { FiExternalLink, FiStar } from "react-icons/fi";
import NextLink from "next/link";

import type { IntelNotice } from "@/service/intel";
import { DataSurface } from "@/components/analysis/bid-analysis-v3/workspace";

/** 预算展示：缺失时明确写“预算未公示”，避免出现 0 元这种误导信息。 */
export function formatBudget(notice: IntelNotice): string {
  if (notice.budget_amount && notice.budget_amount > 0) {
    const amount = notice.budget_amount;
    if (amount >= 100000000) {
      return `${(amount / 100000000).toFixed(2)} 亿元`;
    }
    if (amount >= 10000) {
      return `${(amount / 10000).toFixed(2)} 万元`;
    }
    return `${amount.toFixed(0)} 元`;
  }
  return notice.budget_text?.trim() || "预算未公示";
}

export function formatRegion(notice: IntelNotice): string {
  return (
    [notice.region_province, notice.region_city].filter(Boolean).join(" · ") ||
    "地区未标注"
  );
}

const META_LABEL_W = { base: "4.25rem", md: "4.75rem" };

/** 行业标签的柔和主题色：按名称稳定映射，保证同一标签跨卡片颜色一致。 */
const INDUSTRY_TAG_SCHEMES = ["primary", "info", "success", "purple"] as const;

function industryTagScheme(name: string) {
  let hash = 0;
  for (const char of name) {
    hash = (hash * 31 + (char.codePointAt(0) ?? 0)) >>> 0;
  }
  return INDUSTRY_TAG_SCHEMES[hash % INDUSTRY_TAG_SCHEMES.length];
}

/** 标题不截断：超过可读长度后逐档降字号，让长标题换行而不是变窄。 */
function titleFontSize(title: string) {
  const length = Array.from(title).length;
  if (length <= 28) return { base: "1.0625rem", md: "1.25rem" };
  if (length <= 45) return { base: "1rem", md: "1.125rem" };
  return { base: "0.9375rem", md: "1rem" };
}

/** 一条“定宽标签 + 值”事实，窄卡片里保持标签列对齐、值自然换行。 */
function MetaRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Flex align="flex-start" gap={2} minW={0}>
      <Text
        flexShrink={0}
        w={META_LABEL_W}
        pt="1px"
        fontSize="xs"
        color="workbench.muted"
        whiteSpace="nowrap"
      >
        {label}
      </Text>
      <Box minW={0} flex={1} overflowWrap="anywhere">
        {children}
      </Box>
    </Flex>
  );
}

export default function IntelNoticeCard({
  notice,
  onToggleFavorite,
  favoriteLoading,
  showFavorite = true,
  statusBadge,
  meta,
  actions,
  selectable = false,
  selected = false,
  onSelectedChange,
  detailBasePath = "/intel",
  detailFrom,
  variant = "user",
}: {
  notice: IntelNotice;
  // eslint-disable-next-line no-unused-vars
  onToggleFavorite: (notice: IntelNotice, next: boolean) => void;
  favoriteLoading?: boolean;
  /** 情报管理场景不展示收藏按钮（收藏是个人行为，与管理无关） */
  showFavorite?: boolean;
  /** 管理态徽标（在架/已隐藏/已下架） */
  statusBadge?: React.ReactNode;
  /** 管理态补充信息（备注、置顶时间等） */
  meta?: React.ReactNode;
  /** 管理操作区，渲染在卡片底部 */
  actions?: React.ReactNode;
  selectable?: boolean;
  selected?: boolean;
  // eslint-disable-next-line no-unused-vars
  onSelectedChange?: (next: boolean) => void;
  /**
   * 详情路由前缀。情报管理用自己的详情路由（`/system/intel/notices`），
   * 这样左侧导航定位在“系统管理 → 招标情报管理”，而不是情报大厅。
   */
  detailBasePath?: string;
  /**
   * 来源页面标识。详情页据此决定“返回”回到哪里，
   * 例如情报管理列表点进详情后应回到情报管理，而不是情报大厅。
   */
  detailFrom?: string;
  /** 展示口径：用户侧不渲染管理状态，管理侧追加状态与操作区。 */
  variant?: "user" | "admin";
}) {
  const detailHref = `${detailBasePath}/${notice.id}${
    detailFrom ? `?from=${detailFrom}` : ""
  }`;
  const isAdmin = variant === "admin";
  const hoverProps = {
    transform: "translateY(-2px)",
    boxShadow: "0 12px 28px rgba(11, 27, 43, 0.10)",
    borderColor: "primary.200",
  };
  /** 卡片内可交互控件的焦点环：与既有模块一致的信号金描边。 */
  const focusRing = {
    outline: "2px solid",
    outlineColor: "gold.400",
    outlineOffset: "2px",
  };

  // 未识别/未分类的公告类型不该抢视觉重点：弱化成中性描边，避免满屏“其他”
  const typeUnclear =
    !notice.notice_type_name || notice.notice_type_name === "其他";
  const industryNames = notice.industry_names || [];
  const title = notice.title || "（无标题公告）";

  return (
    <DataSurface
      role="group"
      display="flex"
      flexDirection="column"
      h="full"
      minW={0}
      p={{ base: 4, md: 5 }}
      borderColor={selected ? "primary.600" : "workbench.line"}
      boxShadow={
        selected
          ? "0 0 0 3px var(--chakra-colors-primary-100)"
          : "0 10px 30px rgba(11, 27, 43, 0.06)"
      }
      transition="transform .18s cubic-bezier(0.16,1,0.3,1), box-shadow .18s cubic-bezier(0.16,1,0.3,1), border-color .18s ease-out"
      _hover={
        selected
          ? { transform: hoverProps.transform, boxShadow: hoverProps.boxShadow }
          : hoverProps
      }
      _focusWithin={{
        borderColor: "primary.600",
        boxShadow: "0 0 0 2px var(--chakra-colors-primary-100)",
      }}
    >
      {/* ① 标题：始终是卡片视觉主角，长标题换行并逐档降字号 */}
      <Flex justify="space-between" align="flex-start" gap={3} minW={0}>
        <Text
          as={NextLink}
          href={detailHref}
          flex={1}
          minW={0}
          fontSize={titleFontSize(title)}
          fontWeight="700"
          color="workbench.text"
          lineHeight="1.35"
          letterSpacing="-0.01em"
          overflowWrap="anywhere"
          _hover={{ color: "primary.600", textDecoration: "underline" }}
          _focusVisible={focusRing}
        >
          {title}
        </Text>
        {/* 收藏与选择固定右上角：不占用标题横向空间，长标题也不会被挤压 */}
        <HStack spacing={1} flexShrink={0} align="center">
          {showFavorite ? (
            <IconButton
              aria-label={notice.favorited ? "取消收藏" : "收藏"}
              icon={<FiStar aria-hidden />}
              size="sm"
              minW="44px"
              h="44px"
              variant="ghost"
              isLoading={favoriteLoading}
              color={notice.favorited ? "gold.500" : "neutral.300"}
              _hover={{ color: "gold.500", bg: "gold.50" }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-gold-400)",
                outline: "none",
              }}
              onClick={() => onToggleFavorite(notice, !notice.favorited)}
            />
          ) : null}
          {selectable ? (
            <Box
              minW="44px"
              minH="44px"
              display="flex"
              alignItems="center"
              justifyContent="center"
            >
              <Checkbox
                isChecked={selected}
                onChange={(event) => onSelectedChange?.(event.target.checked)}
                aria-label={`选择公告 ${notice.title}`}
                size="lg"
                colorScheme="primary"
              />
            </Box>
          ) : null}
        </HStack>
      </Flex>

      {/* ② 状态与公告类型：管理侧同一行，用户侧只保留情报类型 */}
      <Flex mt={3} gap={2} wrap="wrap" align="center">
        {isAdmin && notice.pinned ? (
          <Badge colorScheme="gold" variant="subtle" borderRadius="md">
            置顶
          </Badge>
        ) : null}
        {isAdmin ? statusBadge : null}
        <Badge
          colorScheme={typeUnclear ? "neutral" : "primary"}
          variant={typeUnclear ? "outline" : "subtle"}
          borderRadius="md"
          px={2}
          py={0.5}
          flexShrink={0}
        >
          {notice.notice_type_name || "类型未识别"}
        </Badge>
      </Flex>

      {/* ③ 行业标签单独占一行，全部渲染，不做截断 */}
      <Flex mt={2} align="flex-start" gap={2} minW={0}>
        <Text
          flexShrink={0}
          pt="2px"
          fontSize="xs"
          color="workbench.muted"
          whiteSpace="nowrap"
        >
          行业
        </Text>
        {industryNames.length > 0 ? (
          <Flex flex={1} minW={0} wrap="wrap" gap={2}>
            {industryNames.map((name) => (
              <Tag
                key={name}
                size="sm"
                colorScheme={industryTagScheme(name)}
                variant="subtle"
                borderRadius="md"
                px={2}
                py={0.5}
                fontWeight="500"
                maxW="full"
              >
                <Text as="span" overflowWrap="anywhere">
                  {name}
                </Text>
              </Tag>
            ))}
          </Flex>
        ) : (
          <Text fontSize="xs" color="workbench.muted">
            未标注
          </Text>
        )}
      </Flex>

      {/* ④ 关键事实：预算第一重点，标签定宽让窄卡片仍能纵向扫读 */}
      <Flex direction="column" gap={2} mt={4} minW={0}>
        <MetaRow label="预算">
          <Text
            fontSize="sm"
            fontWeight="700"
            color="primary.700"
            sx={{ fontVariantNumeric: "tabular-nums" }}
          >
            {formatBudget(notice)}
          </Text>
        </MetaRow>
        <MetaRow label="地区">
          <Text fontSize="sm" fontWeight="600" color="workbench.text">
            {formatRegion(notice)}
          </Text>
        </MetaRow>
        <MetaRow label="发布">
          <Text
            fontSize="sm"
            fontWeight="600"
            color="workbench.text"
            sx={{ fontVariantNumeric: "tabular-nums" }}
          >
            {notice.publish_date || "未标注"}
          </Text>
        </MetaRow>
        {notice.publisher ? (
          <MetaRow label="采购人">
            <Text fontSize="sm" color="workbench.text">
              {notice.publisher}
            </Text>
          </MetaRow>
        ) : null}
      </Flex>

      {/* ⑤ 底部：来源与采集时间弱化，原文/详情入口固定右侧 */}
      <Box mt="auto" pt={3}>
        <Divider mb={3} borderColor="workbench.line" />
        <Flex direction="column" gap={1.5} minW={0}>
          <Text
            fontSize="xs"
            color="workbench.muted"
            minW={0}
            overflowWrap="anywhere"
          >
            {notice.source_name || notice.source_key}
            {notice.first_seen_at ? ` · 采集于 ${notice.first_seen_at}` : ""}
          </Text>
          <HStack spacing={3} justify="flex-end" wrap="wrap">
            {notice.url ? (
              <Text
                as="a"
                href={notice.url}
                target="_blank"
                rel="noreferrer noopener"
                fontSize="sm"
                color="primary.600"
                display="inline-flex"
                alignItems="center"
                gap={1}
                minH="44px"
                _hover={{ color: "primary.700", textDecoration: "underline" }}
                _focusVisible={focusRing}
              >
                原文 <FiExternalLink aria-hidden size={13} />
              </Text>
            ) : (
              <Text
                fontSize="sm"
                color="workbench.muted"
                minH="44px"
                display="flex"
                alignItems="center"
              >
                系统录入，无原站链接
              </Text>
            )}
            <Button
              as={NextLink}
              href={detailHref}
              size="sm"
              variant="solid"
              colorScheme="primary"
              h="44px"
              px={4}
              borderRadius="lg"
              flexShrink={0}
              whiteSpace="nowrap"
              transition="none"
              _hover={{
                bg: "primary.700",
                transform: "none",
                textDecoration: "none",
              }}
              _active={{ bg: "primary.800", transform: "none" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "primary.500",
                outlineOffset: "2px",
                boxShadow: "none",
              }}
            >
              查看详情
            </Button>
          </HStack>
        </Flex>

        {meta ? <Box mt={2}>{meta}</Box> : null}

        {actions ? (
          <Stack
            direction={{ base: "column", sm: "row" }}
            spacing={2}
            mt={3}
            pt={3}
            borderTop="1px solid"
            borderColor="workbench.line"
            wrap="wrap"
            align={{ base: "stretch", sm: "center" }}
          >
            {actions}
          </Stack>
        ) : null}
      </Box>
    </DataSurface>
  );
}
