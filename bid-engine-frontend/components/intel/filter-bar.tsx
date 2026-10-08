"use client";

/* Hallmark · component: intel-filter-bar · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * 交互：所有条件都自动生效（父级做 350ms 防抖），不再要求用户点“筛选”。
 * 布局：每个条件都有明确的字段名，日期与预算成对出现并各自带单位，避免“两个日期不知道管哪个”。
 * states: default · hover · focus-visible · active · disabled · loading · empty
 * responsive: 320px 单列 → 宽屏四列；控件高度统一 44px，无页面级横向滚动
 * Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import React, { useState } from "react";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Input,
  InputGroup,
  InputLeftElement,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  SimpleGrid,
  Skeleton,
  Spinner,
  Stack,
  Tag,
  Text,
  Wrap,
  WrapItem,
} from "@chakra-ui/react";
import { FiChevronUp, FiSearch, FiSliders, FiX } from "react-icons/fi";

import IntelSelect, {
  FIELD_HEIGHT,
  type IntelSelectOption,
} from "@/components/intel/intel-select";
import type { IntelFilters, IntelNoticeQuery } from "@/service/intel";

/** 采集时间预设：对应后端的 collect_within_days。 */
export const COLLECT_WITHIN_OPTIONS: IntelSelectOption[] = [
  { value: "", label: "不限采集时间" },
  { value: "1", label: "今天采集" },
  { value: "3", label: "近 3 天采集" },
  { value: "7", label: "近 7 天采集" },
  { value: "30", label: "近 30 天采集" },
];

export type IntelFilterValue = {
  keyword: string;
  industries: string[];
  region: string;
  noticeType: string;
  sourceKey: string;
  dateFrom: string;
  dateTo: string;
  /** 预算下限，单位：万元（提交前换算为元） */
  budgetMinWan: string;
  /** 预算上限，单位：万元（提交前换算为元） */
  budgetMaxWan: string;
  /** 采集时间预设：空 / 1 / 3 / 7 / 30 */
  collectWithin: string;
  favoriteOnly: boolean;
};

export const EMPTY_FILTER_VALUE: IntelFilterValue = {
  keyword: "",
  industries: [],
  region: "",
  noticeType: "",
  sourceKey: "",
  dateFrom: "",
  dateTo: "",
  budgetMinWan: "",
  budgetMaxWan: "",
  collectWithin: "",
  favoriteOnly: false,
};

/** 万元 → 元：筛选表单用万元，接口按元过滤。 */
function wanToYuan(value: string): number | undefined {
  const amount = Number(value);
  if (!value.trim() || !Number.isFinite(amount) || amount < 0) return undefined;
  return Math.round(amount * 10000);
}

/** 把界面状态转成接口查询参数。 */
export function toNoticeQuery(
  value: IntelFilterValue,
  pageNum = 1,
  pageSize = 20,
): IntelNoticeQuery {
  const query: IntelNoticeQuery = { pageNum, pageSize };
  if (value.keyword.trim()) query.keyword = value.keyword.trim();
  if (value.industries.length) query.industries = value.industries;
  if (value.region) query.regions = [value.region];
  if (value.noticeType) query.notice_types = [value.noticeType];
  if (value.sourceKey) query.source_keys = [value.sourceKey];
  if (value.dateFrom) query.date_from = value.dateFrom;
  if (value.dateTo) query.date_to = value.dateTo;
  const budgetMin = wanToYuan(value.budgetMinWan);
  if (budgetMin !== undefined) query.budget_min = budgetMin;
  const budgetMax = wanToYuan(value.budgetMaxWan);
  if (budgetMax !== undefined) query.budget_max = budgetMax;
  const withinDays = Number(value.collectWithin);
  if (value.collectWithin && Number.isFinite(withinDays) && withinDays > 0) {
    query.collect_within_days = withinDays;
  }
  if (value.favoriteOnly) query.favorite_only = true;
  return query;
}

/** 当前生效的筛选条件数量（用于“重置”与空态提示）。 */
export function countActiveFilters(value: IntelFilterValue): number {
  let count = 0;
  if (value.keyword.trim()) count += 1;
  count += value.industries.length;
  if (value.region) count += 1;
  if (value.noticeType) count += 1;
  if (value.sourceKey) count += 1;
  if (value.dateFrom) count += 1;
  if (value.dateTo) count += 1;
  if (value.budgetMinWan.trim()) count += 1;
  if (value.budgetMaxWan.trim()) count += 1;
  if (value.collectWithin) count += 1;
  if (value.favoriteOnly) count += 1;
  return count;
}

/**
 * 筛选状态文案：只在“正在筛选”或“已有生效条件”时展示。
 *
 * 无生效条件时不渲染任何占位文案（此前会显示“条件变更后自动筛选”，属于无信息量的噪音）。
 */
function filterStatusText(loading: boolean, activeCount: number): string {
  if (loading) return "正在筛选";
  return `已自动应用 ${activeCount} 个条件`;
}

/** 每个条件自带字段名，用户一眼看清“这个控件管什么”。 */
function FilterField({
  label,
  helper,
  children,
}: {
  label: string;
  helper?: string;
  children: React.ReactNode;
}) {
  return (
    <FormControl minW={0}>
      <FormLabel
        mb={1.5}
        fontSize="xs"
        fontWeight="650"
        color="workbench.muted"
        letterSpacing="0.02em"
      >
        {label}
        {helper ? (
          <Text as="span" ml={1} fontWeight="400" color="neutral.400">
            {helper}
          </Text>
        ) : null}
      </FormLabel>
      {children}
    </FormControl>
  );
}

/** 原生日期输入默认只有日历图标能展开；点击输入框任意位置时主动唤起选择器。 */
function openNativeDatePicker(event: React.MouseEvent<HTMLInputElement>) {
  event.currentTarget.showPicker?.();
}

/** 正整数输入：支持步进加减，也会挡住非法字符。 */
function BudgetInput({
  value,
  onChange,
  placeholder,
  ariaLabel,
}: {
  value: string;
  // eslint-disable-next-line no-unused-vars
  onChange: (next: string) => void;
  placeholder?: string;
  ariaLabel: string;
}) {
  return (
    <NumberInput
      value={value}
      min={0}
      step={100}
      precision={0}
      clampValueOnBlur={false}
      onChange={(next) => {
        // NumberInput 在非法输入时会给出空串，这里原样回传，交由防抖后的校验处理
        const sanitized = String(next ?? "").replace(/[^\d]/g, "");
        onChange(sanitized);
      }}
      size="md"
    >
      <NumberInputField
        h={FIELD_HEIGHT}
        aria-label={ariaLabel}
        placeholder={placeholder}
        borderColor="neutral.200"
        sx={{ fontVariantNumeric: "tabular-nums" }}
        _hover={{ borderColor: "primary.300" }}
        _focusVisible={{
          borderColor: "primary.500",
          boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
          outline: "none",
        }}
      />
      <NumberInputStepper>
        <NumberIncrementStepper borderColor="neutral.200" />
        <NumberDecrementStepper borderColor="neutral.200" />
      </NumberInputStepper>
    </NumberInput>
  );
}

export default function IntelFilterBar({
  filters,
  filtersLoading,
  value,
  onChange,
  onReset,
  loading,
  collapsible = false,
  primaryExtras,
  showFavoriteToggle = true,
}: {
  filters: IntelFilters | null;
  filtersLoading: boolean;
  value: IntelFilterValue;
  // eslint-disable-next-line no-unused-vars
  onChange: (next: IntelFilterValue) => void;
  onReset: () => void;
  loading: boolean;
  /** 是否把高级条件收进可折叠区（默认折叠，只留关键词） */
  collapsible?: boolean;
  /**
   * 折叠时也常驻在关键词右侧的条件（管理端的状态 / 来源类型下拉）。
   * 情报大厅不传，折叠时就只有搜索框。
   */
  primaryExtras?: React.ReactNode;
  /** 管理端不需要“只看我收藏”（收藏是个人行为） */
  showFavoriteToggle?: boolean;
}) {
  // 条件项偏多，默认折叠；展开状态只属于本次会话，不做持久化
  const [expanded, setExpanded] = useState(!collapsible);
  const patch = (part: Partial<IntelFilterValue>) =>
    onChange({ ...value, ...part });

  const toggleIndustry = (code: string) => {
    const next = value.industries.includes(code)
      ? value.industries.filter((item) => item !== code)
      : [...value.industries, code];
    patch({ industries: next });
  };

  const activeCount = countActiveFilters(value);
  const regionOptions: IntelSelectOption[] = (filters?.regions || []).map(
    (region) => ({ value: region, label: region }),
  );
  const noticeTypeOptions: IntelSelectOption[] = (
    filters?.notice_types || []
  ).map((item) => ({ value: item.code, label: item.name }));
  const sourceOptions: IntelSelectOption[] = (filters?.sources || []).map(
    (item) => ({
      value: item.source_key,
      label: item.name || item.source_key,
      hint: item.category || undefined,
    }),
  );
  const panelId = "intel-filter-panel";
  // 无生效条件且不在请求中时，不渲染任何状态文案
  const showStatusChip = loading || activeCount > 0;

  /** 生效条件数与加载态：折叠时收进展开区头部，避免折叠态被次要信息占位。 */
  const statusChip = (
    <HStack
      spacing={2}
      px={3}
      h={FIELD_HEIGHT}
      borderRadius="lg"
      bg="workbench.canvas"
      border="1px solid"
      borderColor="neutral.100"
    >
      {loading ? (
        <Spinner size="xs" color="primary.500" />
      ) : (
        <Box
          aria-hidden
          w="6px"
          h="6px"
          borderRadius="full"
          bg={activeCount > 0 ? "primary.500" : "neutral.300"}
        />
      )}
      <Text
        fontSize="xs"
        color="workbench.muted"
        whiteSpace="nowrap"
        sx={{ fontVariantNumeric: "tabular-nums" }}
      >
        {filterStatusText(loading, activeCount)}
      </Text>
    </HStack>
  );

  const resetButton = (
    <Button
      h={FIELD_HEIGHT}
      variant="outline"
      borderColor="neutral.200"
      color="neutral.600"
      leftIcon={<FiX aria-hidden />}
      isDisabled={activeCount === 0}
      onClick={onReset}
      _hover={{ borderColor: "primary.300", color: "primary.600" }}
      _active={{ transform: "scale(0.98)" }}
      _focusVisible={{
        boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
        outline: "none",
      }}
    >
      重置
    </Button>
  );

  return (
    <Box
      bg="white"
      borderRadius="xl"
      borderWidth="1px"
      borderColor="neutral.100"
      p={{ base: 4, md: 5 }}
      /* 筛选栏与下方第一个条目卡片之间留出呼吸位（此前紧贴，视觉上像同一块） */
      mb={5}
    >
      <Stack spacing={4}>
        {/* 常驻行：关键词始终可见；管理端再把状态 / 来源类型下拉挂在这一行 */}
        <Flex
          gap={3}
          direction={{ base: "column", lg: "row" }}
          align={{ lg: "center" }}
          wrap="wrap"
        >
          <InputGroup
            w={{ base: "full", md: "auto" }}
            /** 保持与改动前一致的宽度：420px 封顶，空间不足时先收缩再换行，不放大 */
            flex={{ md: "0 1 420px" }}
            minW={{ md: "220px" }}
            maxW={{ base: "full", md: "420px" }}
          >
            <InputLeftElement h={FIELD_HEIGHT} color="neutral.400">
              <FiSearch aria-hidden />
            </InputLeftElement>
            <Input
              h={FIELD_HEIGHT}
              value={value.keyword}
              onChange={(event) => patch({ keyword: event.target.value })}
              placeholder="搜索公告标题或正文关键词"
              aria-label="关键词搜索"
              bg="workbench.canvas"
              borderColor="neutral.200"
              _hover={{ borderColor: "primary.300" }}
              _focusVisible={{
                borderColor: "primary.500",
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
              }}
            />
          </InputGroup>

          {primaryExtras ? (
            <HStack
              spacing={3}
              flexShrink={0}
              wrap="wrap"
              w={{ base: "full", lg: "auto" }}
            >
              {primaryExtras}
            </HStack>
          ) : null}

          {collapsible ? (
            /* “重置”与“展开/收起筛选”同一行、整体贴右：重置只在面板展开时出现（收起时没有
             * 可重置的可见条件），收起/展开按钮始终占据最右，切换时按钮位置不跳动。 */
            <Flex
              gap={3}
              align="center"
              wrap="wrap"
              flexShrink={0}
              w={{ base: "full", lg: "auto" }}
              ml={{ lg: "auto" }}
            >
              {expanded && showStatusChip ? statusChip : null}
              {expanded ? resetButton : null}
              <Button
                h={FIELD_HEIGHT}
                flexShrink={0}
                variant="outline"
                borderColor={expanded ? "primary.300" : "neutral.200"}
                color={expanded ? "primary.600" : "neutral.600"}
                bg={expanded ? "primary.50" : "white"}
                leftIcon={
                  expanded ? (
                    <FiChevronUp aria-hidden />
                  ) : (
                    <FiSliders aria-hidden />
                  )
                }
                rightIcon={
                  !expanded && activeCount > 0 ? (
                    <Badge
                      borderRadius="full"
                      px={2}
                      fontSize="xs"
                      bg="primary.500"
                      color="white"
                      sx={{ fontVariantNumeric: "tabular-nums" }}
                    >
                      {activeCount}
                    </Badge>
                  ) : undefined
                }
                aria-expanded={expanded}
                aria-controls={panelId}
                onClick={() => setExpanded((prev) => !prev)}
                _hover={{ borderColor: "primary.300", color: "primary.600" }}
                _active={{ transform: "scale(0.98)" }}
                _focusVisible={{
                  boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                  outline: "none",
                }}
              >
                {expanded ? "收起筛选" : "展开筛选"}
              </Button>
            </Flex>
          ) : (
            <Flex gap={3} align="center" flexShrink={0} ml="auto">
              {showStatusChip ? statusChip : null}
              {resetButton}
            </Flex>
          )}
        </Flex>

        <Collapse in={expanded} animateOpacity>
          <Stack spacing={4} id={panelId}>
            <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} spacing={4}>
              <FilterField label="地区">
                <IntelSelect
                  value={value.region}
                  options={regionOptions}
                  onChange={(next) => patch({ region: next })}
                  placeholder="全部地区"
                  ariaLabel="地区筛选"
                  isClearable
                  emptyText={filtersLoading ? "加载中" : "暂无可选地区"}
                />
              </FilterField>

              <FilterField label="公告类型">
                <IntelSelect
                  value={value.noticeType}
                  options={noticeTypeOptions}
                  onChange={(next) => patch({ noticeType: next })}
                  placeholder="全部公告类型"
                  ariaLabel="公告类型筛选"
                  isClearable
                  emptyText={filtersLoading ? "加载中" : "暂无可选类型"}
                />
              </FilterField>

              <FilterField label="来源站">
                <IntelSelect
                  value={value.sourceKey}
                  options={sourceOptions}
                  onChange={(next) => patch({ sourceKey: next })}
                  placeholder="全部来源"
                  ariaLabel="来源筛选"
                  isClearable
                  emptyText={filtersLoading ? "加载中" : "暂无可选来源"}
                />
              </FilterField>

              <FilterField label="采集时间" helper="(入库时间)">
                <IntelSelect
                  value={value.collectWithin}
                  options={COLLECT_WITHIN_OPTIONS}
                  onChange={(next) => patch({ collectWithin: next })}
                  placeholder="不限采集时间"
                  ariaLabel="采集时间筛选"
                  isClearable
                />
              </FilterField>
            </SimpleGrid>

            <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} spacing={4}>
              <FilterField label="发布时间" helper="从">
                <Input
                  h={FIELD_HEIGHT}
                  type="date"
                  aria-label="发布时间从"
                  value={value.dateFrom}
                  cursor="pointer"
                  onClick={openNativeDatePicker}
                  onChange={(event) => patch({ dateFrom: event.target.value })}
                  borderColor="neutral.200"
                  _hover={{ borderColor: "primary.300" }}
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FilterField>
              <FilterField label="发布时间" helper="到">
                <Input
                  h={FIELD_HEIGHT}
                  type="date"
                  aria-label="发布时间到"
                  value={value.dateTo}
                  cursor="pointer"
                  onClick={openNativeDatePicker}
                  onChange={(event) => patch({ dateTo: event.target.value })}
                  borderColor="neutral.200"
                  _hover={{ borderColor: "primary.300" }}
                  _focusVisible={{
                    borderColor: "primary.500",
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-200)",
                    outline: "none",
                  }}
                />
              </FilterField>
              <FilterField label="预算下限" helper="(万元)">
                <BudgetInput
                  value={value.budgetMinWan}
                  onChange={(next) => patch({ budgetMinWan: next })}
                  placeholder="不限"
                  ariaLabel="预算下限（万元）"
                />
              </FilterField>
              <FilterField label="预算上限" helper="(万元)">
                <BudgetInput
                  value={value.budgetMaxWan}
                  onChange={(next) => patch({ budgetMaxWan: next })}
                  placeholder="不限"
                  ariaLabel="预算上限（万元）"
                />
              </FilterField>
            </SimpleGrid>

            <Box>
              <Text
                fontSize="xs"
                fontWeight="650"
                color="workbench.muted"
                letterSpacing="0.02em"
                mb={2}
              >
                行业（可多选）
              </Text>
              {filtersLoading ? (
                <Wrap spacing={2}>
                  {[0, 1, 2, 3, 4, 5].map((key) => (
                    <WrapItem key={key}>
                      <Skeleton h="32px" w="72px" borderRadius="full" />
                    </WrapItem>
                  ))}
                </Wrap>
              ) : (
                <Wrap spacing={2}>
                  {(filters?.industries || []).map((item) => {
                    const active = value.industries.includes(item.code);
                    return (
                      <WrapItem key={item.code}>
                        <Tag
                          as="button"
                          type="button"
                          size="md"
                          minH="32px"
                          px={3}
                          borderRadius="full"
                          cursor="pointer"
                          aria-pressed={active}
                          bg={active ? "primary.500" : "workbench.canvas"}
                          color={active ? "white" : "neutral.600"}
                          border="1px solid"
                          borderColor={active ? "primary.500" : "neutral.200"}
                          transition="background-color .15s ease-out, color .15s ease-out, border-color .15s ease-out"
                          _hover={{
                            bg: active ? "primary.600" : "primary.50",
                            color: active ? "white" : "primary.700",
                          }}
                          _focusVisible={{
                            boxShadow:
                              "0 0 0 2px var(--chakra-colors-primary-300)",
                            outline: "none",
                          }}
                          onClick={() => toggleIndustry(item.code)}
                        >
                          {item.name}
                        </Tag>
                      </WrapItem>
                    );
                  })}
                </Wrap>
              )}
            </Box>

            {showFavoriteToggle ? (
              <Flex gap={3} wrap="wrap" align="center">
                <Button
                  h={FIELD_HEIGHT}
                  variant={value.favoriteOnly ? "solid" : "outline"}
                  colorScheme={value.favoriteOnly ? "primary" : undefined}
                  borderColor="neutral.200"
                  color={value.favoriteOnly ? "white" : "neutral.600"}
                  onClick={() => patch({ favoriteOnly: !value.favoriteOnly })}
                  _active={{ transform: "scale(0.98)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                    outline: "none",
                  }}
                >
                  只看我收藏
                </Button>
              </Flex>
            ) : null}
          </Stack>
        </Collapse>
      </Stack>
    </Box>
  );
}
