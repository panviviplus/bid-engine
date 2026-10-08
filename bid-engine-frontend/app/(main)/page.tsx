"use client";

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * modules: six-card responsive grid · motion: hover lift + button press · no fabricated metrics
 * contrast: pass (40–41) · mobile: code-reviewed (34, 49, 50–57)
 * slop: 57/58 · gate 13 intentionally follows the approved multi-channel hover
 */
/* Hallmark · pre-emit critique: P5 H5 E5 S5 R4 V4 */
import { type ReactNode, useMemo, useState } from "react";
import NextLink from "next/link";
import {
  Badge,
  Box,
  Button,
  ButtonGroup,
  Flex,
  Grid,
  Heading,
  HStack,
  Icon,
  Link,
  Skeleton,
  Stack,
  Text,
  VStack,
} from "@chakra-ui/react";
import { PageContent, PageViewport } from "@/components/layout/responsive-page";
import {
  FiAlertTriangle,
  FiArrowUpRight,
  FiClock,
  FiFileText,
  FiPlayCircle,
  FiRefreshCw,
} from "react-icons/fi";
import { useReducedMotion } from "framer-motion";

import AIBadge from "@/components/common/ai-badge";
import { useAppContext } from "@/contexts/app-context";
import {
  HomeModuleKey,
  HomeModuleStat,
  HomeIntelStat,
  HomePeriod,
  HomeWorkItem,
  useHomeAttention,
  useHomeLLMConfigStatus,
  useHomeRecentWork,
  useHomeStats,
} from "@/service/home";
import {
  HOME_MODULES,
  HOME_MODULE_LABELS,
  MATERIAL_LINKS,
} from "./home-modules.mjs";

type ModuleDefinition = {
  id: string;
  label: string;
  description: string;
  statKey?: HomeModuleKey;
  href?: string;
  planned: boolean;
};

const PERIOD_OPTIONS: Array<{ key: HomePeriod; label: string }> = [
  { key: "today", label: "今日" },
  { key: "week", label: "近 7 天" },
  { key: "month", label: "本月" },
  { key: "year", label: "今年" },
];

const MODULE_DEFINITIONS = HOME_MODULES as ModuleDefinition[];
const MODULE_LABELS = HOME_MODULE_LABELS as Record<string, string>;

const PERIOD_LABELS: Record<HomePeriod, string> = {
  today: "今日",
  week: "近 7 天",
  month: "本月",
  year: "今年",
};

const ATTENTION_META = {
  failed: { label: "失败", bg: "error.50", color: "error.700" },
  risk: { label: "风险", bg: "error.50", color: "error.700" },
  action_required: { label: "待操作", bg: "warning.50", color: "warning.700" },
  running: { label: "运行中", bg: "primary.50", color: "primary.700" },
} as const;

function getGreeting() {
  const hour = new Date().getHours();
  if (hour < 8) return "早上好";
  if (hour < 12) return "上午好";
  if (hour < 14) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
}

function getWorkHref(item: HomeWorkItem) {
  if (item.module === "bid_analysis") {
    if (
      item.status === "succeeded" ||
      item.status === "succeeded_with_warnings"
    ) {
      return `/bid-analysis/${item.id}`;
    }
    return "/bid-analysis";
  }
  if (item.module === "bid_generation") return `/file-gen/${item.id}`;
  if (item.module === "bid_review") return `/bid-audit/${item.id}`;
  return "/";
}

function formatUpdatedAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "更新时间未知";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function statusLabel(item: HomeWorkItem) {
  if (item.attention_type) return ATTENTION_META[item.attention_type].label;
  const labels: Record<string, string> = {
    running: "运行中",
    paused: "已暂停",
    failed: "失败",
    succeeded: "已完成",
    succeeded_with_warnings: "有警告",
    parsing: "解析中",
    outline_review: "待确认大纲",
    draft: "草稿",
    generating: "生成中",
    succeed: "已完成",
  };
  return labels[item.status] || item.status || "状态未知";
}

function RetryBlock({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <Flex
      minH="132px"
      align="center"
      justify="center"
      direction="column"
      gap={3}
      color="neutral.500"
    >
      <Text fontSize="sm">{message}</Text>
      <Button
        size="sm"
        minH="44px"
        variant="outline"
        leftIcon={<FiRefreshCw />}
        onClick={onRetry}
      >
        重新加载
      </Button>
    </Flex>
  );
}

/** 招标情报站卡片指标：今日新增 / 未读提醒 / 我的订阅（数值来自真实接口，失败时显示占位符）。 */
function IntelMetrics({
  failed,
  intelStat,
}: {
  failed: boolean;
  intelStat?: HomeIntelStat;
}) {
  const items = [
    { label: "今日新增", value: intelStat?.today_new ?? 0 },
    { label: "未读提醒", value: intelStat?.unread_alerts ?? 0 },
    { label: "我的订阅", value: intelStat?.subscription_count ?? 0 },
  ];
  return (
    <Flex gap={4} sx={{ fontVariantNumeric: "tabular-nums" }}>
      {items.map((item) => (
        <Box key={item.label}>
          <Text
            fontFamily="mono"
            fontSize="xl"
            fontWeight="700"
            lineHeight="1"
            color="neutral.900"
          >
            {failed ? "—" : item.value}
          </Text>
          <Text mt={1.5} color="neutral.600" fontSize="xs">
            {item.label}
          </Text>
        </Box>
      ))}
    </Flex>
  );
}

function ModuleCard({
  definition,
  stat,
  intelStat,
  period,
  loading,
  failed,
}: {
  definition: ModuleDefinition;
  stat?: HomeModuleStat;
  intelStat?: HomeIntelStat;
  period: HomePeriod;
  loading: boolean;
  failed: boolean;
}) {
  const reducedMotion = useReducedMotion();
  const metric = failed ? "—" : (stat?.count ?? 0);
  const attentionCount = stat?.attention_count || 0;
  const isMaterial = definition.statKey === "material";
  const isIntel = definition.id === "tender_intelligence";
  // 指标区内容（拆成变量，避免深层三元嵌套）
  let metricContent: ReactNode;
  if (isIntel) {
    metricContent = loading ? (
      <Skeleton h="30px" w="160px" borderRadius="md" />
    ) : (
      <IntelMetrics failed={failed} intelStat={intelStat} />
    );
  } else if (loading) {
    metricContent = <Skeleton h="30px" w="64px" borderRadius="md" />;
  } else {
    metricContent = (
      <Text
        fontFamily="mono"
        fontSize="xl"
        fontWeight="700"
        lineHeight="1"
        sx={{ fontVariantNumeric: "tabular-nums" }}
        color="neutral.900"
      >
        {metric}
      </Text>
    );
  }
  let headerAction: ReactNode = null;
  let moduleAction: ReactNode = null;

  if (definition.planned) {
    headerAction = (
      <Badge
        minH="28px"
        display="inline-flex"
        alignItems="center"
        px={2.5}
        flexShrink={0}
        borderRadius="full"
        bg="neutral.100"
        color="neutral.600"
      >
        规划中
      </Badge>
    );
  } else if (definition.href) {
    headerAction = (
      <Flex
        className="module-arrow"
        w="36px"
        h="36px"
        flexShrink={0}
        align="center"
        justify="center"
        border="1px solid"
        borderColor="neutral.400"
        borderRadius="full"
        color="neutral.700"
        aria-hidden
      >
        <FiArrowUpRight />
      </Flex>
    );
  }

  if (isMaterial) {
    moduleAction = (
      <HStack spacing={1.5} flexWrap="wrap">
        {MATERIAL_LINKS.map(({ id, label, href }) => (
          <Link
            key={id}
            as={NextLink}
            href={href}
            minH="44px"
            display="inline-flex"
            alignItems="center"
            px={2}
            border="1px solid"
            borderColor="neutral.300"
            borderRadius="full"
            fontSize="sm"
            whiteSpace="nowrap"
            _hover={{
              borderColor: "primary.600",
              color: "primary.700",
              bg: "white",
            }}
            _focusVisible={{
              boxShadow: "0 0 0 3px var(--chakra-colors-gold-700)",
            }}
          >
            {label} {failed ? "—" : (stat?.breakdown?.[id] ?? 0)}
          </Link>
        ))}
      </HStack>
    );
  } else if (attentionCount > 0) {
    moduleAction = (
      <Badge
        minH="30px"
        display="inline-flex"
        alignItems="center"
        px={2.5}
        borderRadius="full"
        bg="warning.50"
        color="warning.700"
      >
        待关注 {attentionCount}
      </Badge>
    );
  }

  const body = (
    <>
      <Flex align="flex-start" justify="space-between" gap={3}>
        <Heading
          className="module-title"
          minW={0}
          fontSize={{ base: "lg", md: "xl" }}
          letterSpacing="-0.03em"
          color="neutral.900"
        >
          {definition.label}
        </Heading>
        {headerAction}
      </Flex>

      <Text
        mt={3}
        maxW="52ch"
        minH={{ base: "auto", md: "2.75rem" }}
        color="neutral.600"
        fontSize="sm"
        noOfLines={2}
      >
        {definition.description}
      </Text>

      {!definition.planned && (
        <Flex
          mt="auto"
          pt={4}
          align={{ base: "flex-start", sm: "flex-end" }}
          justify="space-between"
          direction={{ base: "column", sm: "row" }}
          gap={3}
        >
          <Box>
            {metricContent}
            {!isIntel && (
              <Text mt={1.5} color="neutral.600" fontSize="xs">
                {isMaterial
                  ? "个人累计素材"
                  : `${PERIOD_LABELS[period]}个人项目`}
              </Text>
            )}
          </Box>

          {moduleAction}
        </Flex>
      )}
    </>
  );

  const surfaceProps = {
    minH: { base: "auto", md: "216px" },
    display: "flex",
    flexDirection: "column" as const,
    p: { base: 4, md: 5 },
    border: "1px solid",
    borderColor: "workbench.line",
    borderRadius: "16px",
    bg: "workbench.paper",
    color: "neutral.900",
    transition: reducedMotion ? "none" : "transform 0.12s var(--ease-out)",
    _focusVisible: {
      outline: "none",
      boxShadow: "0 0 0 3px var(--chakra-colors-gold-700)",
    },
  };
  const interactiveHover = {
    bg: "primary.50",
    borderColor: "primary.500",
    boxShadow: "md",
    transform: reducedMotion ? undefined : "translateY(-2px)",
    "& .module-title": { color: "primary.700" },
    "& .module-arrow": {
      bg: "primary.600",
      borderColor: "primary.600",
      color: "white",
    },
  };

  if (definition.href) {
    return (
      <Link
        as={NextLink}
        href={definition.href}
        textDecoration="none"
        _hover={{ textDecoration: "none", ...interactiveHover }}
        _active={{ transform: reducedMotion ? undefined : "translateY(0)" }}
        {...surfaceProps}
      >
        {body}
      </Link>
    );
  }
  return (
    <Box
      _hover={definition.planned ? undefined : interactiveHover}
      {...surfaceProps}
    >
      {body}
    </Box>
  );
}

function WorkRow({ item }: { item: HomeWorkItem }) {
  return (
    <Link
      as={NextLink}
      href={getWorkHref(item)}
      minH="72px"
      display="grid"
      gridTemplateColumns={{
        base: "44px minmax(0, 1fr)",
        sm: "44px minmax(0, 1fr) auto",
      }}
      alignItems="center"
      gap={3}
      p={3}
      borderRadius="12px"
      textDecoration="none"
      _hover={{ textDecoration: "none", bg: "neutral.100" }}
      _active={{ transform: "translateY(1px)" }}
      _focusVisible={{ boxShadow: "0 0 0 3px var(--chakra-colors-gold-500)" }}
    >
      <Flex
        w="40px"
        h="40px"
        align="center"
        justify="center"
        borderRadius="10px"
        bg="primary.50"
        color="primary.700"
        aria-hidden
      >
        <Icon as={FiFileText} boxSize={6} />
      </Flex>
      <Box minW={0}>
        <Text fontWeight="700" noOfLines={1}>
          {item.name}
        </Text>
        <Text mt={1} fontSize="xs" color="neutral.500" noOfLines={1}>
          {MODULE_LABELS[item.module] || item.module} ·{" "}
          {formatUpdatedAt(item.updated_at)}
        </Text>
      </Box>
      <Badge
        gridColumn={{ base: 2, sm: 3 }}
        justifySelf="start"
        minH="30px"
        display="inline-flex"
        alignItems="center"
        px={3}
        borderRadius="full"
        colorScheme={item.status === "failed" ? "red" : "blue"}
        whiteSpace="nowrap"
      >
        {statusLabel(item)}
      </Badge>
    </Link>
  );
}

function AttentionRow({ item }: { item: HomeWorkItem }) {
  const meta =
    ATTENTION_META[item.attention_type || "running"] || ATTENTION_META.running;
  return (
    <Link
      as={NextLink}
      href={getWorkHref(item)}
      minH="72px"
      display="grid"
      gridTemplateColumns="auto minmax(0, 1fr)"
      alignItems="center"
      gap={3}
      p={3}
      borderRadius="12px"
      textDecoration="none"
      _hover={{ textDecoration: "none", bg: "neutral.100" }}
      _active={{ transform: "translateY(1px)" }}
      _focusVisible={{ boxShadow: "0 0 0 3px var(--chakra-colors-gold-500)" }}
    >
      <Badge
        minH="30px"
        display="inline-flex"
        alignItems="center"
        px={3}
        borderRadius="full"
        bg={meta.bg}
        color={meta.color}
        whiteSpace="nowrap"
      >
        {meta.label}
      </Badge>
      <Box minW={0}>
        <Text fontWeight="700" noOfLines={1}>
          {item.name}
        </Text>
        <Text mt={1} fontSize="xs" color="neutral.500" noOfLines={1}>
          {MODULE_LABELS[item.module] || item.module}
          {item.error_count > 0 ? ` · 高风险 ${item.error_count}` : ""}
          {item.warning_count > 0 ? ` · 告警 ${item.warning_count}` : ""}
        </Text>
      </Box>
    </Link>
  );
}

export default function DashboardPage() {
  const { userProfile } = useAppContext() as any;
  const [period, setPeriod] = useState<HomePeriod>("month");
  const { statsData, statsLoading, statsError, refreshStats } =
    useHomeStats(period);
  const { recentItems, recentLoading, recentError, refreshRecent } =
    useHomeRecentWork();
  const { attentionItems, attentionLoading, attentionError, refreshAttention } =
    useHomeAttention();
  const { llmStatus, llmStatusError, refreshLLMStatus } =
    useHomeLLMConfigStatus();

  const missingModuleNames = useMemo(
    () =>
      (llmStatus?.missing_modules || []).map(
        (key) => MODULE_LABELS[key] || key,
      ),
    [llmStatus?.missing_modules],
  );

  let recentContent;
  if (recentLoading) {
    recentContent = (
      <Stack spacing={2}>
        {[0, 1, 2].map((item) => (
          <Skeleton key={item} h="72px" borderRadius="12px" />
        ))}
      </Stack>
    );
  } else if (recentError) {
    recentContent = (
      <RetryBlock message="最近工作加载失败" onRetry={() => refreshRecent()} />
    );
  } else if (recentItems.length === 0) {
    recentContent = (
      <Flex minH="132px" align="center" justify="center" direction="column">
        <Icon as={FiClock} color="neutral.400" boxSize={6} />
        <Text mt={3} fontWeight="700">
          还没有最近项目
        </Text>
        <Text mt={1} fontSize="sm" color="neutral.500">
          从上方任一模块开始第一项工作。
        </Text>
      </Flex>
    );
  } else {
    recentContent = (
      <VStack spacing={1} align="stretch">
        {recentItems.map((item) => (
          <WorkRow key={[item.module, item.id].join("-")} item={item} />
        ))}
      </VStack>
    );
  }

  let attentionContent;
  if (attentionLoading) {
    attentionContent = (
      <Stack spacing={2}>
        {[0, 1, 2].map((item) => (
          <Skeleton key={item} h="72px" borderRadius="12px" />
        ))}
      </Stack>
    );
  } else if (attentionError) {
    attentionContent = (
      <RetryBlock
        message="待关注任务加载失败"
        onRetry={() => refreshAttention()}
      />
    );
  } else if (attentionItems.length === 0) {
    attentionContent = (
      <Flex minH="132px" align="center" justify="center" direction="column">
        <Icon as={FiPlayCircle} color="success.500" boxSize={6} />
        <Text mt={3} fontWeight="700">
          当前没有待关注任务
        </Text>
        <Text mt={1} fontSize="sm" color="neutral.500">
          失败、风险和运行中任务会显示在这里。
        </Text>
      </Flex>
    );
  } else {
    attentionContent = (
      <VStack spacing={1} align="stretch">
        {attentionItems.map((item) => (
          <AttentionRow
            key={[item.module, item.id, item.attention_type].join("-")}
            item={item}
          />
        ))}
      </VStack>
    );
  }

  return (
    <PageViewport bg="workbench.canvas">
      <PageContent py={{ base: 5, md: 8 }}>
        <Grid
          templateColumns={{
            base: "minmax(0, 1fr)",
            lg: "minmax(0, 1fr) auto",
          }}
          alignItems="end"
          gap={6}
          pb={{ base: 6, md: 8 }}
        >
          <Box minW={0}>
            <HStack spacing={2} align="center">
              <Heading
                fontSize={{ base: "2xl", md: "2.5rem" }}
                letterSpacing="-0.04em"
                overflowWrap="anywhere"
              >
                {getGreeting()}，{userProfile?.name || "用户"}
              </Heading>
              <AIBadge />
            </HStack>
          </Box>
          <ButtonGroup
            isAttached
            variant="outline"
            flexWrap="wrap"
            aria-label="统计周期"
          >
            {PERIOD_OPTIONS.map((option) => (
              <Button
                key={option.key}
                minH="44px"
                px={{ base: 3, md: 4 }}
                borderRadius="full"
                bg={
                  period === option.key
                    ? "workbench.control"
                    : "workbench.paper"
                }
                color={period === option.key ? "white" : "neutral.700"}
                borderColor="workbench.line"
                whiteSpace="nowrap"
                aria-pressed={period === option.key}
                onClick={() => setPeriod(option.key)}
                _hover={{
                  bg:
                    period === option.key
                      ? "workbench.controlRaised"
                      : "neutral.50",
                }}
                _active={{ transform: "translateY(1px)" }}
              >
                {option.label}
              </Button>
            ))}
          </ButtonGroup>
        </Grid>

        {missingModuleNames.length > 0 && (
          <Flex
            mb={7}
            minH="72px"
            align={{ base: "flex-start", md: "center" }}
            justify="space-between"
            direction={{ base: "column", md: "row" }}
            gap={3}
            px={{ base: 4, md: 5 }}
            py={3}
            border="1px solid"
            borderColor="warning.400"
            borderRadius="14px"
            bg="warning.50"
          >
            <HStack align="flex-start" spacing={3}>
              <Icon as={FiAlertTriangle} mt={1} color="warning.700" />
              <Box>
                <Text fontWeight="700">
                  {missingModuleNames.length} 个模块尚未配置模型
                </Text>
                <Text mt={1} fontSize="sm" color="neutral.600">
                  {missingModuleNames.join("、")} ·
                  这里只表示配置就绪度，不代表实时在线状态。
                </Text>
              </Box>
            </HStack>
            <Link
              as={NextLink}
              href="/system/llm-config"
              minH="44px"
              display="inline-flex"
              alignItems="center"
              fontWeight="700"
              whiteSpace="nowrap"
            >
              检查配置 →
            </Link>
          </Flex>
        )}

        {llmStatusError && (
          <Flex mb={7} align="center" gap={3} color="error.700">
            <Text fontSize="sm">模型配置状态加载失败。</Text>
            <Button
              size="sm"
              minH="44px"
              variant="ghost"
              onClick={() => refreshLLMStatus()}
            >
              重试
            </Button>
          </Flex>
        )}

        <Flex
          mb={4}
          align={{ base: "flex-start", md: "flex-end" }}
          justify="space-between"
          direction={{ base: "column", md: "row" }}
          gap={2}
        >
          <Box>
            <Heading fontSize="xl" letterSpacing="-0.02em">
              模块总览
            </Heading>
          </Box>
          <HStack>
            <Text color="workbench.muted" fontSize="sm">
              统计周期 · {PERIOD_LABELS[period]}
            </Text>
            {statsError && (
              <Button
                size="sm"
                minH="44px"
                variant="ghost"
                leftIcon={<FiRefreshCw />}
                onClick={() => refreshStats()}
              >
                重试
              </Button>
            )}
          </HStack>
        </Flex>

        <Grid
          templateColumns="repeat(auto-fit, minmax(min(100%, 21rem), 1fr))"
          gap={4}
        >
          {MODULE_DEFINITIONS.map((definition) => (
            <ModuleCard
              key={definition.id}
              definition={definition}
              stat={
                definition.statKey
                  ? statsData?.modules?.[definition.statKey]
                  : undefined
              }
              intelStat={statsData?.tender_intel}
              period={period}
              loading={statsLoading}
              failed={Boolean(statsError)}
            />
          ))}
        </Grid>

        <Grid
          mt={6}
          templateColumns={{
            base: "minmax(0, 1fr)",
            xl: "minmax(0, 1.45fr) minmax(18rem, .8fr)",
          }}
          gap={6}
        >
          <Box
            minW={0}
            p={{ base: 4, md: 5 }}
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="16px"
            bg="workbench.paper"
          >
            <Flex align="center" justify="space-between" gap={3} mb={3}>
              <HStack>
                <Icon as={FiClock} color="primary.600" />
                <Heading fontSize="lg">继续工作</Heading>
              </HStack>
              <Text color="workbench.muted" fontSize="xs">
                最近更新
              </Text>
            </Flex>
            {recentContent}
          </Box>

          <Box
            minW={0}
            p={{ base: 4, md: 5 }}
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="16px"
            bg="workbench.paper"
          >
            <Flex align="center" justify="space-between" gap={3} mb={3}>
              <HStack>
                <Icon as={FiPlayCircle} color="gold.600" />
                <Heading fontSize="lg">待关注</Heading>
              </HStack>
              <Text color="workbench.muted" fontSize="xs">
                按优先级
              </Text>
            </Flex>
            {attentionContent}
          </Box>
        </Grid>
      </PageContent>
    </PageViewport>
  );
}
