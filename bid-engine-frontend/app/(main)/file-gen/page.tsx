"use client";

/* eslint-disable no-use-before-define */
/* Hallmark · component: project-card · genre: modern-minimal · theme: BidEngine workbench (preserved tokens)
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 * contrast: inherited from locked Chakra workbench tokens
 * pre-emit critique: P5 H4 E4 S5 R5 V4
 */
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { KeyboardEvent } from "react";
import {
  Box,
  Button,
  Flex,
  Grid,
  IconButton,
  Input,
  InputGroup,
  InputLeftElement,
  Progress,
  Skeleton,
  Text,
  Tooltip,
  keyframes,
  useDisclosure,
  useToast,
} from "@chakra-ui/react";
import {
  FiFileText,
  FiPlus,
  FiSearch,
  FiAlertTriangle,
  FiTrash2,
} from "react-icons/fi";
import { format } from "date-fns";
import { useRouter } from "next/navigation";
import NextLink from "next/link";

import { PageViewport } from "@/components/layout/responsive-page";
import {
  ModuleWorkbenchDeck,
  ModuleWorkbenchHeader,
} from "@/components/common/module-workbench.mjs";
import {
  AdaptiveProjectTitle,
  DataSurface,
  WorkspaceShell,
} from "@/components/analysis/bid-analysis-v3/workspace";
import {
  useBidGenList,
  useBidGenDelete,
} from "@/service/bid-gen";
import { useLlmConfigExist } from "@/service/llm-config";
import CreateBidModal from "@/components/file-gen/create-bid-modal";
import type { BidGenProjectItem } from "@/components/file-gen/types";
import StatusFilterBar from "@/components/common/status-filter-bar";
import EmptyState from "@/components/common/empty-state";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { useStatusTotals } from "@/hooks/use-status-totals";

const PAGE_SIZE = 12;
const LIST_URL = "/zb/file-gen/project/list";
const SCROLL_ID = "file-gen-scroll";
// 卡片宽度随视口连续增长（clamp: 小屏 280px 起，大屏最高 420px，列数自适应填充）
const FLUID_GRID =
  "repeat(auto-fill, minmax(min(100%, clamp(280px, 24vw, 420px)), 1fr))";

const STATUS_META: Record<
  string,
  { label: string; color: string; bg: string }
> = {
  parsing: { label: "解析中", color: "primary.600", bg: "primary.50" },
  outline_review: { label: "大纲待确认", color: "gold.600", bg: "gold.50" },
  draft: { label: "草稿", color: "gray.600", bg: "gray.100" },
  generating: { label: "生成中", color: "warning.600", bg: "warning.50" },
  succeeded: { label: "已完成", color: "success.600", bg: "success.50" },
  failed: { label: "失败", color: "error.600", bg: "error.50" },
};

// 创建方式胶囊（与状态胶囊同构：小圆点 + 文案；analysis = 从招标解析终审确认流转）
const CREATE_TYPE_META: Record<
  string,
  { label: string; bg: string; color: string; dotColor: string }
> = {
  tender_file: {
    label: "来自招标书",
    bg: "primary.50",
    color: "primary.600",
    dotColor: "primary.500",
  },
  template: {
    label: "来自模板",
    bg: "info.50",
    color: "info.600",
    dotColor: "info.500",
  },
  blank: {
    label: "来自空白标书",
    bg: "neutral.50",
    color: "neutral.600",
    dotColor: "neutral.400",
  },
  analysis: {
    label: "来自招标解析",
    bg: "gold.50",
    color: "gold.600",
    dotColor: "gold.500",
  },
};

const STATUS_TABS = [
  { key: "", label: "全部", color: "primary.600" },
  { key: "parsing", label: "解析中", color: "info.500" },
  { key: "outline_review", label: "大纲待确认", color: "gold.600" },
  { key: "draft", label: "草稿", color: "neutral.500" },
  { key: "succeeded", label: "已完成", color: "success.500" },
  { key: "failed", label: "失败", color: "error.500" },
];

// 解析阶段元信息（与创建弹窗 stageMeta 一致）
const PARSE_STAGE_META: Record<string, { label: string; desc: string }> = {
  tender_interpretation: {
    label: "招标文件解读",
    desc: "识别章节结构，生成项目摘要",
  },
  info_extraction: { label: "提炼重要信息", desc: "提取核心字段与关键条款" },
  blueprint_generation: { label: "大纲蓝图生成", desc: "生成标书大纲蓝图" },
  template_parse: { label: "模板文档解析", desc: "解析模板文档内容" },
  template_outline: { label: "模板大纲提取", desc: "提取标题大纲结构" },
};

// 当前阶段圆点呼吸动画
const blink = keyframes`
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
`;

function parseStagesOf(createType: string): string[] {
  return createType === "template"
    ? ["template_parse", "template_outline"]
    : ["tender_interpretation", "info_extraction", "blueprint_generation"];
}

function stepDotBg(
  isDone: boolean,
  isFailed: boolean,
  isCurrent: boolean,
): string {
  if (isDone) return "success.500";
  if (isFailed) return "error.500";
  if (isCurrent) return "info.500";
  return "neutral.200";
}

function stepDotColor(
  isDone: boolean,
  isFailed: boolean,
  isCurrent: boolean,
): string {
  if (isDone || isFailed || isCurrent) return "workbench.paper";
  return "neutral.400";
}

function stepLabelColor(isFailed: boolean, isCurrent: boolean): string {
  if (isFailed) return "error.500";
  if (isCurrent) return "info.600";
  return "neutral.600";
}

function stepIcon(st: string): string {
  if (st === "succeeded") return "✓";
  if (st === "failed") return "✗";
  if (st === "running") return "…";
  return "○";
}

export default function FileGenPage() {
  const router = useRouter();
  const toast = useToast();

  const [status, setStatus] = useState("");
  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [deckOpen, setDeckOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const { exists: genLlmExists } = useLlmConfigExist("bid_generation");

  const [deleteTargetId, setDeleteTargetId] = useState<number | null>(null);
  const {
    isOpen: deleteIsOpen,
    onOpen: deleteOnOpen,
    onClose: deleteOnClose,
  } = useDisclosure();
  const [statsRefreshKey, setStatsRefreshKey] = useState(0);

  const { fetchList } = useBidGenList({
    pageNum: 1,
    pageSize: PAGE_SIZE,
    status: status || undefined,
    name: keyword || undefined,
  });
  const { deleteLoading, fetchDelete } = useBidGenDelete();

  useEffect(() => {
    const t = window.setTimeout(() => setKeyword(keywordInput.trim()), 400);
    return () => window.clearTimeout(t);
  }, [keywordInput]);

  const fetcher = useCallback(
    async (pageNum: number) => {
      const res = await fetchList({
        params: {
          pageNum,
          pageSize: PAGE_SIZE,
          status: status || undefined,
          name: keyword || undefined,
        },
      });
      return {
        items: (res?.data?.data?.list as BidGenProjectItem[]) || [],
        total: res?.data?.data?.total || 0,
      };
    },
    [fetchList, status, keyword],
  );

  const {
    items,
    initialLoading,
    loadingMore,
    hasMore,
    error,
    loadFirstPage,
    loadMore,
    mergeRefresh,
  } = useInfiniteList<BidGenProjectItem>({
    fetcher,
    pageSize: PAGE_SIZE,
    resetDeps: [status, keyword],
  });

  const { totals, totalsLoading, refreshTotals } = useStatusTotals({
    statuses: STATUS_TABS.map((s) => s.key),
    keyword,
    url: LIST_URL,
    refreshKey: statsRefreshKey,
  });

  // 自动刷新：存在解析中/生成中项目时每 4s 合并刷新已加载列表 + 统计
  const hasActive = useMemo(
    () =>
      items.some((p) => p.status === "parsing" || p.status === "generating"),
    [items],
  );
  useEffect(() => {
    if (!hasActive) return;
    const timer = window.setInterval(() => {
      mergeRefresh();
      refreshTotals();
    }, 4000);
    return () => window.clearInterval(timer);
  }, [hasActive, mergeRefresh, refreshTotals]);

  const refreshAfterMutation = useCallback(() => {
    setStatsRefreshKey((k) => k + 1);
    loadFirstPage();
  }, [loadFirstPage]);

  const handleDelete = useCallback(
    (id: number) => {
      setDeleteTargetId(id);
      deleteOnOpen();
    },
    [deleteOnOpen],
  );

  const handleDeleteConfirm = useCallback(async () => {
    if (deleteTargetId === null) return;
    try {
      await fetchDelete({ params: { project_id: deleteTargetId } });
      toast({ title: "删除成功", status: "success", duration: 2000 });
      refreshAfterMutation();
    } catch (e) {
      toast({ title: "删除失败", status: "error", duration: 3000 });
    } finally {
      deleteOnClose();
    }
  }, [
    deleteTargetId,
    fetchDelete,
    refreshAfterMutation,
    toast,
    deleteOnClose,
  ]);

  const renderList = () => {
    if (initialLoading) {
      return (
        <Grid templateColumns={FLUID_GRID} gap={4} alignItems="start">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <DataSurface key={i} p={5} minH="248px">
              <Skeleton h="20px" w="42%" />
              <Skeleton mt={3} h="24px" w="68%" borderRadius="full" />
              <Skeleton mt={3} h="64px" borderRadius="11px" />
              <Skeleton mt={4} h="16px" w="48%" />
            </DataSurface>
          ))}
        </Grid>
      );
    }
    if (error && items.length === 0) {
      return (
        <Flex direction="column" align="center" gap={3} py={12}>
          <Text fontSize="sm" color="neutral.500">
            {error || "加载失败"}
          </Text>
          <Button
            size="sm"
            colorScheme="primary"
            variant="outline"
            onClick={loadFirstPage}
          >
            重新加载
          </Button>
        </Flex>
      );
    }
    if (items.length === 0) {
      return (
        <EmptyState
          icon={<FiFileText />}
          title="还没有标书项目"
          description="从招标文件、模板或空白文档开始创建"
          action={
            <Button
              bg="workbench.control"
              color="workbench.paper"
              _hover={{ bg: "workbench.controlRaised" }}
              leftIcon={<FiPlus />}
              onClick={() => setCreateOpen(true)}
            >
              创建第一份标书
            </Button>
          }
        />
      );
    }
    return (
      <InfiniteScrollList
        dataLength={items.length}
        hasMore={hasMore}
        loadMore={loadMore}
        scrollableTarget={SCROLL_ID}
      >
        <Grid templateColumns={FLUID_GRID} gap={4} alignItems="start">
          {items.map((p) => (
            <ProjectCard
              key={p.id}
              project={p}
              onOpen={() => router.push(`/file-gen/${p.id}`)}
              onDelete={handleDelete}
            />
          ))}
        </Grid>
        {error && items.length > 0 && (
          <Flex justify="center" align="center" gap={2} py={4}>
            <Text fontSize="sm" color="neutral.500">
              {error}
            </Text>
            <Button
              size="xs"
              colorScheme="primary"
              variant="outline"
              isLoading={loadingMore}
              onClick={loadMore}
            >
              重试
            </Button>
          </Flex>
        )}
      </InfiniteScrollList>
    );
  };

  const activeCount = items.filter(
    (item) => item.status === "parsing" || item.status === "generating",
  ).length;

  return (
    <PageViewport
      id={SCROLL_ID}
      w="full"
      className="thin-scrollbars"
      bg="workbench.canvas"
    >
      <WorkspaceShell minH="100%">
        <ModuleWorkbenchHeader
          title="标书生成"
          activity={
            activeCount > 0
              ? `当前列表有 ${activeCount} 个项目处理中`
              : undefined
          }
        />

        <ModuleWorkbenchDeck
          title="从大纲开始，组织并完成整份投标书"
          description="选择模板或空白文档起稿，也可从招标解析确认后的蓝图直接进入编写。"
          expanded={deckOpen}
          onToggle={() => setDeckOpen((value) => !value)}
          action={
            <Button
              leftIcon={<FiPlus />}
              minH="44px"
              px={6}
              w={{ base: "full", md: "auto" }}
              bg="workbench.paper"
              color="workbench.control"
              border="1px solid"
              borderColor="whiteAlpha.600"
              whiteSpace="nowrap"
              _hover={{ bg: "gold.50", transform: "translateY(-1px)" }}
              _active={{ transform: "translateY(0)" }}
              _focusVisible={{
                outline: "2px solid",
                outlineColor: "gold.300",
                outlineOffset: "3px",
              }}
              onClick={() => setCreateOpen(true)}
            >
              创建标书
            </Button>
          }
        >
          {["模板起稿", "空白起稿", "解析蓝图起稿"].map((label) => (
            <Flex
              key={label}
              align="center"
              gap={1.5}
              px={2.5}
              py={1}
              borderRadius="full"
              bg="workbench.controlRaised"
            >
              <Box w="6px" h="6px" borderRadius="full" bg="gold.400" />
              <Text>{label}</Text>
            </Flex>
          ))}
        </ModuleWorkbenchDeck>

        {/* LLM 配置缺失横幅 */}
        {genLlmExists === false && (
          <Flex
            align="center"
            gap={3}
            mt={4}
            mb={5}
            px={4}
            py={3}
            borderRadius="12px"
            bg="warning.50"
            border="1px solid"
            borderColor="warning.200"
          >
            <FiAlertTriangle
              color="var(--chakra-colors-warning-500)"
              size="18"
            />
            <Box flex="1">
              <Text fontSize="sm" fontWeight="600" color="warning.700">
                尚未配置标书生成 AI 模型
              </Text>
              <Text fontSize="xs" color="warning.600">
                AI 撰写章节功能暂不可用，请先前往模型配置完成设置
              </Text>
            </Box>
            <Button
              as={NextLink}
              href="/system/llm-config"
              size="sm"
              bg="workbench.control"
              color="workbench.paper"
              _hover={{ bg: "workbench.controlRaised" }}
            >
              去配置
            </Button>
          </Flex>
        )}

        {/* 状态筛选 + 统计（合并胶囊，主题色与卡片一致）+ 搜索 */}
        <Flex
          mt={6}
          align={{ base: "stretch", lg: "center" }}
          justify="space-between"
          direction={{ base: "column", lg: "row" }}
          gap={3}
          mb={5}
        >
          <StatusFilterBar
            options={STATUS_TABS}
            value={status}
            counts={totals}
            loading={totalsLoading}
            onChange={(key) => setStatus(key)}
          />
          <InputGroup w={{ base: "full", lg: "320px" }} flexShrink={0}>
            <InputLeftElement
              h="44px"
              pointerEvents="none"
              color="workbench.muted"
            >
              <FiSearch />
            </InputLeftElement>
            <Input
              placeholder="搜索项目名称"
              h="44px"
              pl="40px"
              borderRadius="10px"
              bg="workbench.paper"
              border="1px solid"
              borderColor="workbench.line"
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              _hover={{ borderColor: "neutral.300" }}
              _focusVisible={{
                borderColor: "workbench.control",
                boxShadow: "none",
                outline: "2px solid",
                outlineColor: "gold.400",
                outlineOffset: "2px",
              }}
            />
          </InputGroup>
        </Flex>

        {/* 长画布卡片流 */}
        {renderList()}
      </WorkspaceShell>

      <CreateBidModal
        isOpen={createOpen}
        onClose={() => {
          setCreateOpen(false);
          refreshAfterMutation();
        }}
        onCreated={(id) => {
          setCreateOpen(false);
          refreshAfterMutation();
          router.push(`/file-gen/${id}`);
        }}
      />

      {/* 删除确认弹窗 */}
      <DeleteConfirmModal
        isOpen={deleteIsOpen}
        onClose={deleteOnClose}
        title="确认删除"
        description="确认删除此标书项目？"
        isLoading={deleteLoading}
        handleConfirm={handleDeleteConfirm}
      />
    </PageViewport>
  );
}

/**
 * Hallmark · component: project-card (bid-gen) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * 两行标题、单轨元信息、常驻章节进度与紧凑页脚；运行态沿用深海蓝进度面板。
 * states: default · hover · focus-visible · active · disabled · loading · error · success
 */
function ProjectCard({
  project,
  onOpen,
  onDelete,
}: {
  project: BidGenProjectItem;
  onOpen: () => void;
  // eslint-disable-next-line no-unused-vars
  onDelete: (_id: number) => void;
}) {
  const meta = STATUS_META[project.status] || STATUS_META.draft;
  const typeMeta =
    CREATE_TYPE_META[project.createType] || CREATE_TYPE_META.blank;
  const isParsing = project.status === "parsing";
  const isGenerating = project.status === "generating";
  const isActive = isParsing || isGenerating;
  const isFailed = project.status === "failed";
  // 招标文件类型解析中不允许进入详情页
  const enterable = !(project.createType === "tender_file" && isParsing);

  // 解析进度气泡（200ms 停留延迟唤起；焦点即时；150ms 延迟关闭桥接）
  const [stageOpen, setStageOpen] = useState(false);
  const stageHideTimer = useRef<number | null>(null);
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
    stageHideTimer.current = window.setTimeout(() => setStageOpen(false), 150);
  };
  useEffect(
    () => () => {
      if (stageHideTimer.current) window.clearTimeout(stageHideTimer.current);
      if (stageShowTimer.current) window.clearTimeout(stageShowTimer.current);
    },
    [],
  );

  const HOVER_BORDER: Record<string, string> = {
    failed: "error.300",
    succeeded: "success.300",
    outline_review: "gold.300",
  };
  const hoverBorder = HOVER_BORDER[project.status] || "primary.200";
  // 卡片边框：失败红 / 默认灰
  const cardBorder = isFailed ? "error.200" : "workbench.line";

  return (
    <Tooltip
      label={enterable ? "" : "解析完成后可进入详情"}
      isDisabled={enterable || stageOpen}
      hasArrow
    >
      <DataSurface
        minH="248px"
        p={5}
        display="flex"
        flexDirection="column"
        cursor={enterable ? "pointer" : "default"}
        position="relative"
        borderColor={cardBorder}
        onClick={() => {
          if (enterable) onOpen();
        }}
        transition="border-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
        _hover={
          enterable
            ? {
                borderColor: hoverBorder,
              }
            : undefined
        }
        _active={enterable ? { transform: "translateY(1px)" } : undefined}
        _focusVisible={{
          outline: "2px solid",
          outlineColor: "gold.400",
          outlineOffset: "2px",
        }}
        role={enterable ? "button" : undefined}
        tabIndex={enterable ? 0 : undefined}
        onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
          if (enterable && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            onOpen();
          }
        }}
      >
        <Box minW="0">
          <AdaptiveProjectTitle name={project.name} />
        </Box>

        {/* 创建来源、项目状态与审核状态合并为同一条元信息轨道。 */}
        <Flex align="center" gap={1.5} mt={2.5} flexWrap="wrap">
          <Flex
            align="center"
            gap={1.5}
            bg={typeMeta.bg}
            color={typeMeta.color}
            px={2}
            py={0.5}
            borderRadius="full"
            fontSize="xs"
            fontWeight="600"
            whiteSpace="nowrap"
          >
            <Box w="1.5" h="1.5" borderRadius="full" bg={typeMeta.dotColor} />
            {typeMeta.label}
          </Flex>
          <Flex
            align="center"
            gap={1.5}
            bg={meta.bg}
            color={meta.color}
            px={2.5}
            py={1}
            borderRadius="full"
            fontSize="xs"
            fontWeight="600"
            whiteSpace="nowrap"
          >
            <Box w="1.5" h="1.5" borderRadius="full" bg={meta.color} />
            {meta.label}
          </Flex>
          {project.reviewProjectId > 0 && (
            <Flex
              align="center"
              gap={1.5}
              bg="success.50"
              color="success.600"
              px={2.5}
              py={1}
              borderRadius="full"
              fontSize="xs"
              fontWeight="600"
              whiteSpace="nowrap"
            >
              <Box w="1.5" h="1.5" borderRadius="full" bg="success.500" />
              已生成审核项目
            </Flex>
          )}
        </Flex>

        {isActive ? (
          <Box
            mt={3}
            position="relative"
            px={3}
            py={2.5}
            minH="64px"
            borderRadius="11px"
            bg="workbench.control"
            color="workbench.paper"
            {...(isParsing
              ? {
                  role: "button",
                  tabIndex: 0,
                  "aria-expanded": stageOpen,
                  onMouseEnter: stageShow,
                  onMouseLeave: stageHide,
                  onFocus: stageShowNow,
                  onBlur: stageHide,
                  onKeyDown: (e: KeyboardEvent<HTMLDivElement>) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setStageOpen((v) => !v);
                    }
                  },
                }
              : {})}
          >
            <Progress
              value={project.progress || 8}
              size="xs"
              colorScheme="gold"
              borderRadius="full"
              hasStripe
              isAnimated
            />
            <Flex
              mt={2}
              justify="space-between"
              color="whiteAlpha.700"
              fontSize="xs"
            >
              <Text>{isParsing ? "正在解析结构" : "正在生成章节"}</Text>
              <Text fontFamily="mono" fontWeight="700">
                {project.progress || 0}%
              </Text>
            </Flex>
            {isParsing && stageOpen && (
              <ParsingStageBubble
                project={project}
                onHoverIn={stageShowNow}
                onHoverOut={stageHide}
              />
            )}
          </Box>
        ) : (
          <Box
            mt={3}
            px={3}
            py={2.5}
            minH="64px"
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="11px"
            bg="neutral.50"
          >
            <Flex align="center" justify="space-between" gap={3}>
              <Text fontSize="xs" fontWeight="600" color="neutral.600">
                章节进度
              </Text>
              <Text
                fontSize="xs"
                fontWeight="700"
                color={project.outlineCount > 0 ? meta.color : "neutral.400"}
                sx={{ fontVariantNumeric: "tabular-nums" }}
              >
                {project.outlineCount > 0
                  ? `${project.succeededCount}/${project.outlineCount}`
                  : "尚未建立大纲"}
              </Text>
            </Flex>
            {project.outlineCount > 0 && (
              <Progress
                mt={2.5}
                value={(project.succeededCount / project.outlineCount) * 100}
                size="xs"
                colorScheme={project.status === "succeeded" ? "green" : "primary"}
                bg="neutral.100"
                borderRadius="full"
              />
            )}
          </Box>
        )}

        {/* 底部：时间 + 删除 */}
        <Flex align="center" justify="space-between" gap={2} mt="auto" pt={3}>
          <Text fontSize="xs" color="neutral.400">
            {project.createdTime
              ? format(new Date(project.createdTime * 1000), "yyyy-MM-dd HH:mm")
              : "—"}
          </Text>
          <Flex align="center" gap={1}>
            <Tooltip label="删除">
              <IconButton
                aria-label="删除项目"
                icon={<FiTrash2 />}
                size="sm"
                minW="44px"
                minH="44px"
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
                  onDelete(project.id);
                }}
              />
            </Tooltip>
          </Flex>
        </Flex>
      </DataSurface>
    </Tooltip>
  );
}

// 解析进度气泡：步骤圆点 + 连接线 + 箭头
function ParsingStageBubble({
  project,
  onHoverIn,
  onHoverOut,
}: {
  project: BidGenProjectItem;
  onHoverIn: () => void;
  onHoverOut: () => void;
}) {
  const stageStatus = project.stageStatus || {};
  const stages = parseStagesOf(project.createType);
  return (
    <Box
      position="absolute"
      bottom="calc(100% + 10px)"
      left={0}
      width={{ base: "calc(100% - 8px)", sm: "340px" }}
      maxWidth="none"
      zIndex={70}
      bg="white"
      border="1px solid"
      borderColor="gray.200"
      borderRadius="xl"
      boxShadow="0 12px 32px rgba(16,24,40,0.16), 0 2px 8px rgba(16,24,40,0.08)"
      px={3.5}
      py={3}
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
        borderColor="gray.200"
        transform="rotate(45deg)"
      />
      <Flex align="center" justify="space-between" mb={2.5}>
        <Text fontSize="xs" fontWeight="semibold" color="neutral.600">
          解析进度
        </Text>
        <Text fontSize="xs" color="gray.400">
          {project.progress || 0}%
        </Text>
      </Flex>
      <Flex align="flex-start">
        {stages.map((key, idx) => {
          const st = stageStatus[key] || "pending";
          const meta = PARSE_STAGE_META[key] || { label: key, desc: "" };
          const isDone = st === "succeeded";
          const isFailed = st === "failed";
          const isCurrent = st === "running";
          const prevDone =
            idx > 0 && stageStatus[stages[idx - 1]] === "succeeded";
          const dotBg = stepDotBg(isDone, isFailed, isCurrent);
          const dotColor = stepDotColor(isDone, isFailed, isCurrent);
          const icon = stepIcon(st);
          return (
            <Fragment key={key}>
              {idx > 0 && (
                <Box
                  flex="none"
                  w="14px"
                  h="2px"
                  bg={prevDone ? "success.500" : "neutral.200"}
                  mt="10px"
                />
              )}
              <Box
                flex={1}
                minW={0}
                display="flex"
                flexDirection="column"
                alignItems="center"
                gap={1}
              >
                <Flex
                  w="22px"
                  h="22px"
                  borderRadius="full"
                  alignItems="center"
                  justifyContent="center"
                  fontSize="11px"
                  fontWeight="bold"
                  color={dotColor}
                  bg={dotBg}
                  flex="none"
                  boxShadow={
                    isCurrent ? "0 0 0 4px rgba(49,130,206,0.15)" : undefined
                  }
                  animation={
                    isCurrent ? `${blink} 1.4s ease-in-out infinite` : undefined
                  }
                >
                  {icon}
                </Flex>
                <Text
                  fontSize="10.5px"
                  noOfLines={1}
                  maxW="100%"
                  fontWeight={isCurrent ? 600 : 500}
                  color={stepLabelColor(isFailed, isCurrent)}
                >
                  {meta.label}
                </Text>
                {isCurrent && (
                  <Text fontSize="10px" color="info.600">
                    {project.progress || 0}%
                  </Text>
                )}
              </Box>
            </Fragment>
          );
        })}
      </Flex>
    </Box>
  );
}
