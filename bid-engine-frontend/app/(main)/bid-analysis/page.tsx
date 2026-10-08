"use client";

/* eslint-disable no-use-before-define, no-nested-ternary */

/*
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 * Hallmark · macrostructure: compact module heading + collapsible creation forehead + status grid
 * Running motion is isolated from readable card content and stops when the card/page is hidden.
 */

import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Box,
  Button,
  Collapse,
  Flex,
  Grid,
  HStack,
  Icon,
  IconButton,
  Input,
  InputGroup,
  InputLeftElement,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Radio,
  RadioGroup,
  Skeleton,
  SkeletonText,
  Text,
  Tooltip,
  useDisclosure,
  useToast,
  VStack,
} from "@chakra-ui/react";
import { motion, useInView, useReducedMotion } from "framer-motion";
import { useRouter } from "next/navigation";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import {
  FiAlertTriangle,
  FiArrowUpRight,
  FiChevronDown,
  FiFileText,
  FiPause,
  FiPlay,
  FiRefreshCw,
  FiSearch,
  FiSkipForward,
  FiTrash2,
  FiUpload,
} from "react-icons/fi";

import {
  useAnalysisMutation,
  useAnalysisProjects,
  useCreateAnalysisProject,
  type V3ProjectListItem,
  type V3ControlMode,
  type V3StatusCounts,
} from "@/service/bid-analysis";
import { useLlmConfigExist } from "@/service/llm-config";
import {
  AdaptiveProjectTitle,
  DataSurface,
  SignalBadge,
  WorkspaceShell,
  stageLabel,
  stageProgressFloor,
} from "@/components/analysis/bid-analysis-v3/workspace";
import InfiniteScrollList from "@/components/common/infinite-scroll";
import { useInfiniteList } from "@/hooks/use-infinite-list";

const MotionBox = motion(Box);
const ANALYSIS_PAGE_SIZE = 12;
const ANALYSIS_SCROLL_ID = "analysis-project-list-scroll";
const filters = [
  { key: "", label: "全部", countKey: "all", color: "neutral.500" },
  { key: "running", label: "解析中", countKey: "running", color: "info.500" },
  { key: "paused", label: "已暂停", countKey: "paused", color: "neutral.500" },
  {
    key: "completed",
    label: "已完成",
    countKey: "completed",
    color: "success.500",
  },
  {
    key: "succeeded_with_warnings",
    label: "有告警",
    countKey: "succeeded_with_warnings",
    color: "warning.500",
  },
  { key: "failed", label: "失败", countKey: "failed", color: "error.500" },
] as const;

const emptyCounts: V3StatusCounts = {
  all: 0,
  running: 0,
  paused: 0,
  succeeded: 0,
  succeeded_with_warnings: 0,
  completed: 0,
  failed: 0,
};

function formatDate(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.replace("T", " ").slice(0, 19);
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

const clampProgress = (value: unknown) => {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return 0;
  return Math.max(0, Math.min(100, numeric));
};

function resolveDisplayedProgress(
  item: V3ProjectListItem,
  previousProgress = 0,
) {
  if (
    item.status === "succeeded" ||
    item.status === "succeeded_with_warnings"
  ) {
    return 100;
  }
  if (item.status === "paused") return clampProgress(item.progress);
  return Math.max(
    clampProgress(previousProgress),
    clampProgress(item.progress),
    stageProgressFloor(item.stage),
  );
}

export default function AnalysisWorkbenchPage() {
  const router = useRouter();
  const toast = useToast();
  const reducedMotion = useReducedMotion();
  const fileRef = useRef<HTMLInputElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const skipCancelRef = useRef<HTMLButtonElement>(null);
  const pauseFirstRef = useRef<HTMLInputElement>(null);
  const pauseDiscardRef = useRef<HTMLInputElement>(null);
  const displayedProgressRef = useRef(new Map<number, number>());
  const [counts, setCounts] = useState<V3StatusCounts>(emptyCounts);
  const [status, setStatus] = useState("");
  const [keyword, setKeyword] = useState("");
  const [creationOpen, setCreationOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<V3ProjectListItem | null>(
    null,
  );
  const [retryTarget, setRetryTarget] = useState<V3ProjectListItem | null>(
    null,
  );
  const [retryMode, setRetryMode] = useState("current_stage");
  const [pauseTarget, setPauseTarget] = useState<V3ProjectListItem | null>(
    null,
  );
  const [pauseMode, setPauseMode] = useState<V3ControlMode | "">("");
  const [skipTarget, setSkipTarget] = useState<V3ProjectListItem | null>(null);
  const [pageVisible, setPageVisible] = useState(true);
  // 招标情报站联动：从 /intel/[id] 跳转过来时携带的公告上下文。
  // 直接读 location.search，避免 useSearchParams 在 App Router 下需要 Suspense 边界。
  const [prefill, setPrefill] = useState<{
    name: string;
    url: string;
    publisher: string;
  } | null>(null);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const params = new URLSearchParams(window.location.search);
    const name = params.get("prefill_name") || "";
    const url = params.get("prefill_url") || "";
    const publisher = params.get("prefill_publisher") || "";
    if (!name && !url) return;
    setPrefill({ name, url, publisher });
  }, []);
  const [prefillDismissed, setPrefillDismissed] = useState(false);
  const deleteDialog = useDisclosure();
  const retryDialog = useDisclosure();
  const pauseDialog = useDisclosure();
  const skipDialog = useDisclosure();
  const { execute } = useAnalysisProjects();
  const { loading: uploading, execute: createProject } =
    useCreateAnalysisProject();
  const { loading: mutating, execute: mutate } = useAnalysisMutation();
  const { exists: modelReady, checking: checkingModel } =
    useLlmConfigExist("bid_analysis");

  const fetcher = useCallback(
    async (pageNum: number) => {
      const response = await execute({
        params: {
          page: pageNum,
          page_size: ANALYSIS_PAGE_SIZE,
          status: status || undefined,
          keyword: keyword.trim() || undefined,
        },
      });
      const data = response?.data?.data;
      const nextItems = ((data?.items || []) as V3ProjectListItem[]).map(
        (item) => {
          const progress = resolveDisplayedProgress(
            item,
            displayedProgressRef.current.get(item.id),
          );
          displayedProgressRef.current.set(item.id, progress);
          return { ...item, progress };
        },
      );
      setCounts({ ...emptyCounts, ...(data?.status_counts || {}) });
      return {
        items: nextItems,
        total: Number(data?.total || 0),
      };
    },
    [execute, keyword, status],
  );

  const {
    items,
    setItems,
    initialLoading: loading,
    loadingMore,
    hasMore,
    error: listError,
    loadFirstPage,
    loadMore,
    mergeRefresh,
  } = useInfiniteList<V3ProjectListItem>({
    fetcher,
    pageSize: ANALYSIS_PAGE_SIZE,
    resetDeps: [status, keyword],
  });

  const hasRunning = useMemo(() => counts.running > 0, [counts.running]);
  useEffect(() => {
    const onVisibility = () =>
      setPageVisible(document.visibilityState === "visible");
    onVisibility();
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);
  useEffect(() => {
    if (!hasRunning || !pageVisible) return undefined;
    const timer = window.setInterval(
      () => mergeRefresh().catch(() => undefined),
      4000,
    );
    return () => window.clearInterval(timer);
  }, [hasRunning, mergeRefresh, pageVisible]);

  const upload = async (file?: File) => {
    if (!file) return;
    if (
      !/\.(pdf|doc|docx)$/i.test(file.name) ||
      file.size > 100 * 1024 * 1024
    ) {
      toast({
        status: "error",
        title: "请选择 100MB 以内的 PDF、DOC 或 DOCX 文件",
      });
      if (fileRef.current) fileRef.current.value = "";
      return;
    }
    const body = new FormData();
    body.append("file", file);
    // 从情报站跳转过来时，用公告标题作为项目名称，避免出现“未命名”项目
    if (prefill?.name && !prefillDismissed) {
      body.append("name", prefill.name);
    }
    try {
      await createProject({
        data: body,
        headers: { "Content-Type": "multipart/form-data" },
      });
      toast({ status: "success", title: "解析项目已创建" });
      await loadFirstPage();
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "创建解析项目失败",
      });
    } finally {
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const remove = async () => {
    if (!deleteTarget) return;
    const target = deleteTarget;
    const removeFromView = () => {
      displayedProgressRef.current.delete(target.id);
      setItems((current) => current.filter((item) => item.id !== target.id));
      setCounts((current) => {
        const next = {
          ...current,
          all: Math.max(0, current.all - 1),
        };
        if (target.status === "succeeded") {
          next.succeeded = Math.max(0, current.succeeded - 1);
          next.completed = Math.max(0, current.completed - 1);
        } else if (target.status === "succeeded_with_warnings") {
          next.succeeded_with_warnings = Math.max(
            0,
            current.succeeded_with_warnings - 1,
          );
          next.completed = Math.max(0, current.completed - 1);
        } else {
          next[target.status] = Math.max(0, current[target.status] - 1);
        }
        return next;
      });
      deleteDialog.onClose();
      setDeleteTarget(null);
    };
    try {
      await mutate({
        url: `/zb/v3/projects/${target.id}`,
        method: "DELETE",
        data: { reason: "用户删除招标解析项目" },
      });
      // The vanished card is the success feedback; do not wait for a second
      // list request before reflecting the completed deletion.
      removeFromView();
    } catch (error: any) {
      if (error?.response?.status === 404) {
        // A stale list item should not survive after the server confirms that
        // the underlying project is already absent.
        removeFromView();
      } else {
        toast({
          status: "error",
          title: error?.response?.data?.msg || "删除失败",
        });
        return;
      }
    }
    try {
      await loadFirstPage();
    } catch {
      toast({
        status: "warning",
        title: "项目已移除，但列表刷新失败",
        description: "当前结果已保留，可稍后手动刷新页面。",
      });
    }
  };

  const openRetry = (item: V3ProjectListItem) => {
    setRetryTarget(item);
    setRetryMode("current_stage");
    retryDialog.onOpen();
  };

  const recover = async () => {
    if (!retryTarget) return;
    try {
      if (retryMode === "skip") {
        await mutate({
          url: `/zb/v3/projects/${retryTarget.id}/skip-stage`,
          method: "POST",
          data: {
            expected_run_id: retryTarget.run_id,
            expected_stage: retryTarget.stage,
          },
        });
      } else {
        await mutate({
          url: `/zb/v3/projects/${retryTarget.id}/retry`,
          method: "POST",
          data: { mode: retryMode },
        });
      }
      if (retryMode === "global") {
        displayedProgressRef.current.delete(retryTarget.id);
        setItems((current) =>
          current.map((item) =>
            item.id === retryTarget.id
              ? {
                  ...item,
                  status: "running",
                  stage: "document_preprocessing",
                  progress: 0,
                }
              : item,
          ),
        );
      }
      retryDialog.onClose();
      toast({
        status: "success",
        title:
          retryMode === "global"
            ? "已开始全局重新解析"
            : retryMode === "skip"
              ? "已跳过当前阶段"
              : "已从当前阶段重试",
      });
      await mergeRefresh();
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "恢复解析失败",
      });
    }
  };

  const openPause = (item: V3ProjectListItem) => {
    setPauseTarget(item);
    setPauseMode("");
    pauseDialog.onOpen();
  };

  const requestPause = async () => {
    if (!pauseTarget || !pauseMode) return;
    const target = pauseTarget;
    try {
      const response = await mutate({
        url: `/zb/v3/projects/${target.id}/pause`,
        method: "POST",
        data: {
          mode: pauseMode,
          expected_run_id: target.run_id,
          expected_stage: target.stage,
        },
      });
      const control = response?.data?.data?.control;
      setItems((current) =>
        current.map((item) =>
          item.id === target.id
            ? {
                ...item,
                control,
                can_pause: false,
                can_skip_current: false,
              }
            : item,
        ),
      );
      pauseDialog.onClose();
      setPauseTarget(null);
      setPauseMode("");
      mergeRefresh().catch(() => undefined);
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "暂停请求未提交",
        description: "项目仍按原状态运行，请刷新后重试。",
      });
    }
  };

  const requestSkip = async () => {
    if (!skipTarget) return;
    const target = skipTarget;
    try {
      const response = await mutate({
        url: `/zb/v3/projects/${target.id}/skip-stage`,
        method: "POST",
        data: {
          expected_run_id: target.run_id,
          expected_stage: target.stage,
        },
      });
      const control = response?.data?.data?.control;
      setItems((current) =>
        current.map((item) =>
          item.id === target.id
            ? {
                ...item,
                control,
                can_pause: false,
                can_skip_current: false,
              }
            : item,
        ),
      );
      skipDialog.onClose();
      setSkipTarget(null);
      mergeRefresh().catch(() => undefined);
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "跳过请求未提交",
        description: "当前阶段仍在运行，请刷新后重试。",
      });
    }
  };

  const resume = async (item: V3ProjectListItem) => {
    try {
      await mutate({
        url: `/zb/v3/projects/${item.id}/resume`,
        method: "POST",
        data: {
          expected_run_id: item.run_id,
          expected_stage: item.stage,
        },
      });
      setItems((current) =>
        current.map((candidate) =>
          candidate.id === item.id
            ? {
                ...candidate,
                status: "running",
                control: null,
                can_pause: true,
                can_resume: false,
                can_skip_current:
                  candidate.stage === "chapter_fact_extracting" ||
                  candidate.stage === "content_consolidating",
              }
            : candidate,
        ),
      );
      mergeRefresh().catch(() => undefined);
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "继续解析失败",
        description: "项目仍保持暂停，请刷新后重试。",
      });
    }
  };

  return (
    <WorkspaceShell
      id={ANALYSIS_SCROLL_ID}
      h="full"
      overflowY="auto"
      className="thin-scrollbars"
    >
      <Flex
        align={{ base: "flex-start", sm: "flex-end" }}
        justify="space-between"
        gap={4}
        mb={6}
        direction={{ base: "column", sm: "row" }}
      >
        <HStack align="baseline" spacing={2}>
          <Text
            as="h1"
            minW={0}
            fontSize="28px"
            lineHeight="1.15"
            fontWeight="800"
            color="workbench.control"
            letterSpacing="-0.035em"
            overflowWrap="anywhere"
          >
            招标解析
          </Text>
          <Text
            fontSize="md"
            fontWeight="600"
            color="workbench.muted"
            letterSpacing="-0.01em"
          >
            工作台
          </Text>
        </HStack>
        {counts.running > 0 && (
          <HStack
            minH="44px"
            px={4}
            bg="workbench.paper"
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="full"
            fontSize="sm"
            color="workbench.muted"
          >
            <MotionBox
              aria-hidden
              w="8px"
              h="8px"
              borderRadius="full"
              bg="gold.400"
              animate={
                !reducedMotion && pageVisible
                  ? { opacity: [0.45, 1, 0.45], scale: [0.82, 1, 0.82] }
                  : { opacity: 1, scale: 1 }
              }
              transition={{
                duration: 2.4,
                repeat: !reducedMotion && pageVisible ? Infinity : 0,
                ease: [0.65, 0, 0.35, 1],
              }}
            />
            <Text>当前有 {counts.running} 个项目解析中</Text>
          </HStack>
        )}
      </Flex>

      {prefill && !prefillDismissed && (
        <Flex
          mb={3}
          px={4}
          py={3}
          gap={3}
          align={{ base: "flex-start", md: "center" }}
          justify="space-between"
          direction={{ base: "column", md: "row" }}
          bg="primary.50"
          borderLeftWidth="4px"
          borderLeftColor="primary.400"
          borderRadius="lg"
          role="status"
        >
          <Box minW={0}>
            <Text fontSize="xs" fontWeight="700" color="primary.800">
              来自招标情报站的公告
            </Text>
            <Text
              mt={1}
              fontSize="sm"
              color="workbench.text"
              overflowWrap="anywhere"
            >
              {prefill.name}
              {prefill.publisher ? ` · 采购人：${prefill.publisher}` : ""}
            </Text>
            <Text mt={1} fontSize="xs" color="workbench.muted">
              上传该项目的招标文件后，项目名称会自动使用上面的公告标题。
            </Text>
          </Box>
          <HStack spacing={2} flexShrink={0}>
            {prefill.url && (
              <Button
                as="a"
                href={prefill.url}
                target="_blank"
                rel="noreferrer noopener"
                size="sm"
                minH="44px"
                variant="outline"
                borderColor="primary.200"
                color="primary.700"
                _hover={{ bg: "white" }}
              >
                查看公告原文
              </Button>
            )}
            <Button
              size="sm"
              minH="44px"
              variant="ghost"
              color="neutral.500"
              onClick={() => setPrefillDismissed(true)}
              _hover={{ bg: "white", color: "neutral.700" }}
            >
              忽略
            </Button>
          </HStack>
        </Flex>
      )}

      <DataSurface
        position="relative"
        overflow="hidden"
        bg="workbench.control"
        color="white"
        borderColor="workbench.controlRaised"
        boxShadow="0 18px 50px rgba(11,27,43,.15)"
        _before={{
          content: '""',
          position: "absolute",
          inset: 0,
          pointerEvents: "none",
          opacity: 0.18,
          backgroundImage:
            "linear-gradient(var(--chakra-colors-workbench-controlRaised) 1px, transparent 1px), linear-gradient(90deg, var(--chakra-colors-workbench-controlRaised) 1px, transparent 1px)",
          backgroundSize: "28px 28px",
        }}
      >
        <input
          ref={fileRef}
          type="file"
          accept=".pdf,.doc,.docx"
          hidden
          onChange={(event) => upload(event.target.files?.[0])}
        />
        <Flex
          position="relative"
          zIndex={1}
          align={{ base: "stretch", sm: "center" }}
          justify="space-between"
          gap={3}
          px={{ base: 4, md: 6 }}
          py={creationOpen ? { base: 4, md: 6 } : 3}
          direction={{ base: "column", sm: "row" }}
        >
          <Flex
            as="button"
            type="button"
            flex={1}
            minW={0}
            minH={creationOpen ? "52px" : "36px"}
            p={0}
            align="center"
            justify="space-between"
            gap={4}
            color="white"
            textAlign="left"
            aria-expanded={creationOpen}
            aria-controls="analysis-ingest-details"
            onClick={() => setCreationOpen((value) => !value)}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.300",
              outlineOffset: "3px",
              borderRadius: "md",
            }}
          >
            <Box minW={0}>
              <Text
                display="block"
                fontSize={{ base: "xl", md: "27px" }}
                lineHeight="1.2"
                fontWeight="800"
                letterSpacing="-0.025em"
                overflowWrap="anywhere"
              >
                把整本招标文件，变成可核验的事实网络
              </Text>
            </Box>
            <Box
              aria-hidden
              flexShrink={0}
              transform={creationOpen ? "rotate(180deg)" : "rotate(0deg)"}
              transition={
                reducedMotion
                  ? "none"
                  : "transform 0.22s cubic-bezier(0.65, 0, 0.35, 1)"
              }
            >
              <FiChevronDown size={20} />
            </Box>
          </Flex>
          <Button
            w={{ base: "full", sm: "auto" }}
            minW={{ sm: creationOpen ? "224px" : undefined }}
            minH="44px"
            px={{ base: 4, md: 6 }}
            flexShrink={0}
            whiteSpace="nowrap"
            leftIcon={<FiUpload />}
            bg="white"
            color="workbench.control"
            border="1px solid"
            borderColor="whiteAlpha.600"
            isLoading={uploading}
            loadingText="正在创建"
            onClick={() => fileRef.current?.click()}
            transition="background-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), transform 0.1s cubic-bezier(0.7, 0, 0.84, 0)"
            _hover={{ bg: "gold.50" }}
            _active={{ transform: "translateY(1px)" }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.300",
              outlineOffset: "3px",
            }}
          >
            创建解析项目
          </Button>
        </Flex>
        <Collapse in={creationOpen} animateOpacity={!reducedMotion}>
          <Grid
            id="analysis-ingest-details"
            position="relative"
            zIndex={1}
            templateColumns={{
              base: "minmax(0,1fr)",
              lg: "minmax(0,.8fr) minmax(0,1.2fr)",
            }}
            gap={{ base: 5, md: 6 }}
            alignItems={{ lg: "center" }}
            px={{ base: 4, md: 6 }}
            pb={{ base: 5, md: 6 }}
          >
            <Box minW={0}>
              <Text as="h2" fontWeight="740" fontSize="lg">
                上传招标文件
              </Text>
              <Text mt={2} maxW="58ch" color="whiteAlpha.700" fontSize="sm">
                提交即解析，进度全程于项目卡片中实时呈现。
              </Text>
            </Box>
            <Grid
              templateColumns={{
                base: "repeat(2,minmax(0,1fr))",
                md: "repeat(4,minmax(0,1fr))",
              }}
              gap={2}
            >
              {[
                ["文件格式", "PDF / DOC / DOCX"],
                ["大小限制", "100MB"],
                ["页数限制", "1000 页"],
                [
                  "模型配置",
                  checkingModel ? "检查中" : modelReady ? "已就绪" : "待配置",
                ],
              ].map(([label, value]) => (
                <Box
                  key={label}
                  minW={0}
                  px={3}
                  py={3}
                  bg="workbench.controlRaised"
                  color="white"
                  borderRadius="8px"
                >
                  <Text fontSize="xs" color="whiteAlpha.700">
                    {label}
                  </Text>
                  <Text
                    mt={1}
                    fontFamily="mono"
                    fontSize="sm"
                    fontWeight="650"
                    noOfLines={1}
                  >
                    {value}
                  </Text>
                </Box>
              ))}
            </Grid>
          </Grid>
        </Collapse>
      </DataSurface>

      <Flex
        mt={6}
        gap={3}
        align={{ base: "stretch", lg: "center" }}
        justify="space-between"
        direction={{ base: "column", lg: "row" }}
      >
        <HStack spacing={1} overflowX="auto" pb={1}>
          {filters.map((filter) => {
            const active = status === filter.key;
            return (
              <Button
                key={filter.key}
                size="sm"
                minH="42px"
                flexShrink={0}
                variant="ghost"
                border="1px solid"
                borderColor={active ? "workbench.control" : "transparent"}
                bg={active ? "white" : "transparent"}
                color={active ? "workbench.text" : "workbench.muted"}
                onClick={() => setStatus(filter.key)}
                _hover={{ bg: "white" }}
              >
                <Box
                  w="7px"
                  h="7px"
                  mr={2}
                  borderRadius="full"
                  bg={filter.color}
                />
                {filter.label}
                <Text
                  as="span"
                  ml={2}
                  minW="20px"
                  color={
                    filter.key === "failed" && counts.failed > 0
                      ? "error.600"
                      : active
                        ? "workbench.text"
                        : "workbench.muted"
                  }
                  fontFamily="mono"
                >
                  {counts[filter.countKey]}
                </Text>
              </Button>
            );
          })}
        </HStack>
        <InputGroup w={{ base: "full", lg: "320px" }} flexShrink={0}>
          <InputLeftElement h="44px" pointerEvents="none">
            <Icon as={FiSearch} color="workbench.muted" />
          </InputLeftElement>
          <Input
            aria-label="搜索项目名或文件名"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            h="44px"
            pl="40px"
            bg="white"
            border="1px solid"
            borderColor="workbench.line"
            borderRadius="10px"
            placeholder="搜索项目或文件名"
            _hover={{ borderColor: "neutral.300" }}
            _focus={{
              borderColor: "workbench.control",
              boxShadow: "none",
              outline: "none",
            }}
            _focusVisible={{
              borderColor: "workbench.control",
              boxShadow: "none",
              outline: "2px solid",
              outlineColor: "primary.500",
              outlineOffset: "2px",
            }}
          />
        </InputGroup>
      </Flex>

      {loading && !items.length ? (
        <Grid
          mt={4}
          templateColumns="repeat(auto-fill,minmax(min(100%,340px),1fr))"
          gap={4}
          alignItems="start"
        >
          {[1, 2, 3, 4, 5, 6].map((key) => (
            <DataSurface key={key} p={5} minH="330px">
              <Skeleton h="20px" w="40%" />
              <SkeletonText mt={6} noOfLines={6} spacing={4} />
            </DataSurface>
          ))}
        </Grid>
      ) : listError && !items.length ? (
        <DataSurface
          mt={4}
          minH="290px"
          display="flex"
          alignItems="center"
          justifyContent="center"
        >
          <VStack spacing={3} px={5} textAlign="center">
            <Icon as={FiAlertTriangle} boxSize={8} color="error.400" />
            <Text fontWeight="700">解析项目加载失败</Text>
            <Text color="workbench.muted" fontSize="sm">
              {listError}
            </Text>
            <Button variant="outline" onClick={loadFirstPage}>
              重新加载
            </Button>
          </VStack>
        </DataSurface>
      ) : items.length ? (
        <InfiniteScrollList
          dataLength={items.length}
          hasMore={hasMore}
          loadMore={loadMore}
          scrollableTarget={ANALYSIS_SCROLL_ID}
        >
          <Grid
            mt={4}
            templateColumns="repeat(auto-fill,minmax(min(100%,340px),1fr))"
            gap={4}
            alignItems="start"
          >
            {items.map((item) => (
              <ProjectCard
                key={item.id}
                item={item}
                animate={!reducedMotion && pageVisible}
                isMutating={mutating}
                onOpen={() => router.push(`/bid-analysis/${item.id}`)}
                onRetry={() => openRetry(item)}
                onPause={() => openPause(item)}
                onSkip={() => {
                  setSkipTarget(item);
                  skipDialog.onOpen();
                }}
                onResume={() => resume(item)}
                onDelete={() => {
                  setDeleteTarget(item);
                  deleteDialog.onOpen();
                }}
              />
            ))}
          </Grid>
          {listError && (
            <Flex justify="center" align="center" gap={2} py={4}>
              <Text fontSize="sm" color="workbench.muted">
                {listError}
              </Text>
              <Button
                size="xs"
                variant="outline"
                isLoading={loadingMore}
                onClick={loadMore}
              >
                重试
              </Button>
            </Flex>
          )}
        </InfiniteScrollList>
      ) : (
        <DataSurface
          mt={4}
          minH="290px"
          display="flex"
          alignItems="center"
          justifyContent="center"
        >
          <VStack spacing={3} px={5} textAlign="center">
            <Icon as={FiFileText} boxSize={8} color="neutral.300" />
            <Text fontWeight="700">暂无符合条件的解析项目</Text>
            <Text color="workbench.muted" fontSize="sm">
              创建解析项目后，可在这里查看进度和核验结果。
            </Text>
          </VStack>
        </DataSurface>
      )}

      <AlertDialog
        isOpen={deleteDialog.isOpen}
        onClose={deleteDialog.onClose}
        leastDestructiveRef={cancelRef}
      >
        <AlertDialogOverlay>
          <AlertDialogContent mx={3} borderRadius="14px">
            <AlertDialogHeader>删除解析项目</AlertDialogHeader>
            <AlertDialogBody>
              <HStack align="flex-start">
                <Icon as={FiAlertTriangle} mt={1} color="error.500" />
                <Text color="workbench.muted">
                  删除“{deleteTarget?.name}
                  ”后，解析结果、关联数据和源文件将被清理，且无法恢复。
                </Text>
              </HStack>
            </AlertDialogBody>
            <AlertDialogFooter gap={2}>
              <Button ref={cancelRef} onClick={deleteDialog.onClose}>
                取消
              </Button>
              <Button colorScheme="red" isLoading={mutating} onClick={remove}>
                确认删除
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>

      <Modal
        isOpen={retryDialog.isOpen}
        onClose={retryDialog.onClose}
        size="lg"
        isCentered
      >
        <ModalOverlay />
        <ModalContent mx={3} borderRadius="14px">
          <ModalHeader>选择重试方式</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Text color="workbench.muted" fontSize="sm">
              当前失败阶段：
              <Text as="span" color="workbench.text" fontWeight="700">
                {stageLabel(retryTarget?.stage)}
              </Text>
              。请选择如何恢复解析。
            </Text>
            <RadioGroup mt={4} value={retryMode} onChange={setRetryMode}>
              <VStack align="stretch" spacing={2}>
                <RetryOption
                  value="current_stage"
                  title="从当前阶段重试"
                  description={`保留已经完成的数据，从“${stageLabel(retryTarget?.stage)}”重新执行后续阶段。`}
                />
                <RetryOption
                  value="skip"
                  title="跳过本阶段"
                  description={
                    retryTarget?.can_skip_current
                      ? "记录一条告警并继续后续提炼，最终结果将标记为“完成 · 有告警”。"
                      : "当前阶段是后续解析的基础，不允许跳过。"
                  }
                  disabled={!retryTarget?.can_skip_current}
                />
                <RetryOption
                  value="global"
                  title="全局重新解析"
                  description="从源文件重新开始。现有蓝图会作废，已创建的投标书仍可依靠来源快照继续编辑。"
                />
              </VStack>
            </RadioGroup>
          </ModalBody>
          <ModalFooter gap={2}>
            <Button onClick={retryDialog.onClose}>取消</Button>
            <Button
              colorScheme="primary"
              leftIcon={<FiRefreshCw />}
              isLoading={mutating}
              onClick={recover}
            >
              {retryMode === "global"
                ? "确认重新解析"
                : retryMode === "skip"
                  ? "确认跳过"
                  : "确认重试"}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal
        isOpen={pauseDialog.isOpen}
        onClose={() => {
          pauseDialog.onClose();
          setPauseMode("");
          setPauseTarget(null);
        }}
        initialFocusRef={
          pauseTarget?.stage === "content_consolidating"
            ? pauseDiscardRef
            : pauseFirstRef
        }
        size="lg"
        isCentered
      >
        <ModalOverlay />
        <ModalContent mx={3} borderRadius="14px">
          <ModalHeader>暂停解析</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Text color="workbench.muted" fontSize="sm">
              当前阶段：
              <Text as="span" color="workbench.text" fontWeight="700">
                {stageLabel(pauseTarget?.stage)}
              </Text>
              。请选择暂停位置。
            </Text>
            <RadioGroup
              mt={4}
              value={pauseMode}
              onChange={(value) => setPauseMode(value as V3ControlMode)}
            >
              <VStack align="stretch" spacing={2}>
                <ControlOption
                  inputRef={pauseFirstRef}
                  value="after_stage"
                  title="完成本阶段后暂停"
                  description="保留本阶段完整结果，停在下一阶段开始前。"
                  disabled={pauseTarget?.stage === "content_consolidating"}
                  disabledReason="这是最后阶段，完成后解析将直接完成。"
                />
                <ControlOption
                  inputRef={pauseDiscardRef}
                  value="discard_current"
                  title="立即暂停并回退"
                  description="立即停止当前阶段并清除本阶段结果；再次继续时从本阶段重新开始。"
                  danger
                />
              </VStack>
            </RadioGroup>
          </ModalBody>
          <ModalFooter gap={2}>
            <Button
              minH="44px"
              onClick={() => {
                pauseDialog.onClose();
                setPauseMode("");
                setPauseTarget(null);
              }}
            >
              取消
            </Button>
            <Button
              minH="44px"
              colorScheme={pauseMode === "discard_current" ? "red" : "primary"}
              leftIcon={<FiPause />}
              isDisabled={!pauseMode}
              isLoading={mutating}
              loadingText="正在提交"
              onClick={requestPause}
            >
              确认暂停
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <AlertDialog
        isOpen={skipDialog.isOpen}
        onClose={() => {
          skipDialog.onClose();
          setSkipTarget(null);
        }}
        leastDestructiveRef={skipCancelRef}
        isCentered
      >
        <AlertDialogOverlay>
          <AlertDialogContent mx={3} borderRadius="14px">
            <AlertDialogHeader>
              跳过“{stageLabel(skipTarget?.stage)}”？
            </AlertDialogHeader>
            <AlertDialogBody>
              <HStack align="flex-start" spacing={3}>
                <Icon as={FiAlertTriangle} mt={0.5} color="error.600" />
                <Text color="error.800" lineHeight="1.75">
                  本阶段会立即停止，已生成的数据将清除。后续解析仍会继续，但最终结果可能不完整，并标记告警。
                </Text>
              </HStack>
            </AlertDialogBody>
            <AlertDialogFooter gap={2}>
              <Button
                ref={skipCancelRef}
                minH="44px"
                onClick={() => {
                  skipDialog.onClose();
                  setSkipTarget(null);
                }}
              >
                取消
              </Button>
              <Button
                minH="44px"
                colorScheme="red"
                leftIcon={<FiSkipForward />}
                isLoading={mutating}
                loadingText="正在提交"
                onClick={requestSkip}
              >
                确认跳过
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </WorkspaceShell>
  );
}

function ProjectCard({
  item,
  animate,
  isMutating,
  onOpen,
  onRetry,
  onPause,
  onSkip,
  onResume,
  onDelete,
}: {
  item: V3ProjectListItem;
  animate: boolean;
  isMutating: boolean;
  onOpen: () => void;
  onRetry: () => void;
  onPause: () => void;
  onSkip: () => void;
  onResume: () => void;
  onDelete: () => void;
}) {
  const cardRef = useRef<HTMLDivElement>(null);
  const inView = useInView(cardRef, { amount: 0.15 });
  const running = item.status === "running";
  const failed = item.status === "failed";
  const paused = item.status === "paused";
  const succeeded = item.status === "succeeded";
  const activeMotion = animate && inView && running;
  const controlPending =
    item.control?.status === "requested" || item.control?.status === "applying";
  const progress = clampProgress(item.progress);
  const tailWidth = Math.min(22, progress);
  const remaining =
    item.page_count > 0 ? Math.max(0, item.page_count - item.parsed_pages) : 0;
  const borderColor = succeeded
    ? "success.200"
    : item.status === "succeeded_with_warnings"
      ? "warning.200"
      : failed
        ? "error.200"
        : paused
          ? "neutral.200"
          : running
            ? "info.200"
            : "workbench.line";
  return (
    <DataSurface
      ref={cardRef}
      minH={{ base: "372px", sm: "344px" }}
      p={5}
      display="flex"
      flexDirection="column"
      position="relative"
      overflow="hidden"
      borderColor={borderColor}
      boxShadow={succeeded ? "0 12px 34px rgba(22,101,52,.09)" : undefined}
    >
      {running && (
        <MotionBox
          aria-hidden
          position="absolute"
          inset="-45%"
          bg="conic-gradient(from 0deg,transparent 0 68%,rgba(56,189,248,.8) 76%,rgba(229,185,79,.9) 80%,transparent 88%)"
          opacity={activeMotion ? 0.52 : 0.18}
          animate={activeMotion ? { rotate: 360 } : { rotate: 0 }}
          transition={{
            duration: 8,
            repeat: activeMotion ? Infinity : 0,
            ease: "linear",
          }}
        />
      )}
      <Box position="absolute" inset="1px" bg="white" borderRadius="13px" />
      <Flex position="relative" zIndex={1} direction="column" h="full" flex={1}>
        <Box>
          <SignalBadge status={item.status} />
          <Box mt={3}>
            <AdaptiveProjectTitle name={item.name} />
          </Box>
        </Box>

        {running && (
          <Box
            mt={5}
            p={3}
            borderRadius="11px"
            bg="workbench.control"
            color="white"
          >
            <Flex justify="space-between" gap={3} fontSize="sm">
              <HStack>
                <MotionBox
                  w="7px"
                  h="7px"
                  borderRadius="full"
                  bg="gold.400"
                  animate={
                    activeMotion
                      ? { opacity: [0.45, 1, 0.45], scale: [0.82, 1, 0.82] }
                      : { opacity: 1, scale: 1 }
                  }
                  transition={{
                    duration: 2.4,
                    repeat: activeMotion ? Infinity : 0,
                    ease: [0.65, 0, 0.35, 1],
                  }}
                />
                <Text>{item.fact_subtask_label || stageLabel(item.stage)}</Text>
              </HStack>
              <Text fontFamily="mono" fontWeight="700">
                {progress}%
              </Text>
            </Flex>
            <Box
              role="progressbar"
              aria-label={`${stageLabel(item.stage)}进度`}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress}
              mt={3}
              h="6px"
              borderRadius="full"
              bg="whiteAlpha.200"
              position="relative"
              overflow="visible"
            >
              <MotionBox
                aria-hidden
                position="absolute"
                inset={0}
                borderRadius="full"
                bg="info.300"
                style={{ transformOrigin: "left center" }}
                initial={false}
                animate={{ scaleX: progress / 100 }}
                transition={{
                  duration: animate && inView ? 0.42 : 0,
                  ease: [0.16, 1, 0.3, 1],
                }}
              />
              <MotionBox
                aria-hidden
                position="absolute"
                top="-3px"
                bottom="-3px"
                left={`${Math.max(0, progress - tailWidth)}%`}
                w={`${tailWidth}%`}
                borderRadius="full"
                bgGradient="linear(to-r, transparent, gold.300)"
                transformOrigin="right center"
                animate={
                  activeMotion
                    ? { opacity: [0.25, 0.9, 0.25], x: ["-25%", "0%", "-25%"] }
                    : { opacity: 0.72, x: "0%" }
                }
                transition={{
                  duration: 1.8,
                  repeat: activeMotion ? Infinity : 0,
                  ease: [0.65, 0, 0.35, 1],
                }}
              />
              <MotionBox
                aria-hidden
                position="absolute"
                top="-3px"
                left={`calc(${progress}% - 6px)`}
                w="12px"
                h="12px"
                borderRadius="full"
                bg="gold.300"
                border="2px solid"
                borderColor="workbench.control"
                boxShadow="0 0 0 4px rgba(229,185,79,.15)"
                animate={
                  activeMotion
                    ? { opacity: [0.68, 1, 0.68], scale: [0.82, 1, 0.82] }
                    : { opacity: 1, scale: 1 }
                }
                transition={{
                  duration: 1.8,
                  repeat: activeMotion ? Infinity : 0,
                  ease: [0.65, 0, 0.35, 1],
                }}
              />
            </Box>
            <Flex
              mt={2}
              justify="space-between"
              color="whiteAlpha.700"
              fontSize="xs"
            >
              <Text>
                已覆盖 {item.parsed_pages} / {item.page_count || "—"} 页
              </Text>
              <Text>
                {item.page_count > 0 ? `剩余 ${remaining} 页` : "正在读取页数"}
              </Text>
            </Flex>
            {item.fact_subtask_progress && (
              <Text mt={2} fontSize="xs" color="whiteAlpha.700">
                当前子步骤 {item.fact_subtask_progress.completed}/
                {item.fact_subtask_progress.total}
              </Text>
            )}
          </Box>
        )}

        {failed && (
          <Box
            mt={5}
            p={3}
            borderRadius="10px"
            bg="error.50"
            border="1px solid"
            borderColor="error.100"
          >
            <Flex justify="space-between" gap={3}>
              <Text fontSize="xs" color="error.700">
                失败阶段
              </Text>
              <Text fontSize="sm" fontWeight="700" color="error.800">
                {stageLabel(item.stage)}
              </Text>
            </Flex>
            <Text
              mt={2}
              fontSize="xs"
              lineHeight="1.7"
              color="error.700"
              noOfLines={2}
            >
              {item.last_error ||
                "当前阶段未能完成，请选择合适的方式恢复解析。"}
            </Text>
          </Box>
        )}
        {paused && (
          <Box
            mt={5}
            p={3}
            borderRadius="10px"
            bg="neutral.50"
            border="1px solid"
            borderColor="neutral.200"
          >
            <Text fontSize="xs" color="neutral.700">
              继续阶段
            </Text>
            <Text mt={1} fontWeight="700" color="neutral.800">
              {stageLabel(item.stage)}
            </Text>
          </Box>
        )}
        {!failed && !running && !paused && (
          <Text
            mt={5}
            px={1}
            fontSize="sm"
            color={succeeded ? "success.700" : "warning.700"}
          >
            全文覆盖 {item.parsed_pages} /{" "}
            {item.page_count || item.parsed_pages || "—"} 页
          </Text>
        )}
        {!failed && (
          <Grid
            mt={4}
            templateColumns={{
              base: "repeat(2,minmax(0,1fr))",
              sm: "repeat(4,minmax(0,1fr))",
            }}
            gap={2}
          >
            {[
              ["字段", item.field_count],
              ["证据", item.evidence_count],
              ["表格", item.table_count],
              ["告警", item.warning_count],
            ].map(([label, value]) => (
              <Box
                key={label}
                minW={0}
                bg="neutral.50"
                borderRadius="8px"
                px={2}
                py={2}
              >
                <Text fontSize="10px" color="workbench.muted">
                  {label}
                </Text>
                <Text mt={0.5} fontSize="sm" fontWeight="700" fontFamily="mono">
                  {value}
                </Text>
              </Box>
            ))}
          </Grid>
        )}

        <Box mt="auto" pt={4}>
          {running && controlPending && (
            <Box
              minH="44px"
              display="flex"
              alignItems="center"
              justifyContent="center"
              px={3}
              borderRadius="9px"
              bg={item.control?.action === "skip" ? "error.50" : "info.50"}
              border="1px solid"
              borderColor={
                item.control?.action === "skip" ? "error.200" : "info.200"
              }
              color={item.control?.action === "skip" ? "error.800" : "info.800"}
              fontSize="sm"
              fontWeight="700"
              aria-live="polite"
            >
              {item.control?.action === "skip"
                ? "正在跳过"
                : item.control?.mode === "after_stage"
                  ? "完成本阶段后暂停"
                  : "正在暂停"}
            </Box>
          )}
          {running &&
            !controlPending &&
            (item.control?.status === "failed" ||
              item.control?.status === "cancelled") && (
              <Box
                mb={3}
                px={3}
                py={2.5}
                borderRadius="9px"
                bg="error.50"
                border="1px solid"
                borderColor="error.200"
                aria-live="polite"
              >
                <Text fontSize="xs" fontWeight="700" color="error.800">
                  {item.control?.action === "skip"
                    ? "跳过未生效"
                    : "暂停未生效"}
                </Text>
                <Text mt={0.5} fontSize="xs" lineHeight="1.6" color="error.700">
                  {item.control?.last_error ||
                    "解析任务未在运行，请刷新后重试。"}
                </Text>
              </Box>
            )}
          {running && !controlPending && (
            <Box>
              <Grid templateColumns="repeat(2,minmax(0,1fr))" gap={2}>
                <Button
                  minH="44px"
                  px={2}
                  leftIcon={<FiPause />}
                  variant="outline"
                  borderColor="info.300"
                  color="info.800"
                  isDisabled={!item.can_pause}
                  onClick={onPause}
                >
                  暂停解析
                </Button>
                <Button
                  minH="44px"
                  px={2}
                  leftIcon={<FiSkipForward />}
                  variant="outline"
                  borderColor="error.300"
                  color="error.700"
                  isDisabled={!item.can_skip_current}
                  onClick={onSkip}
                >
                  跳过本阶段
                </Button>
              </Grid>
              {!item.can_skip_current && item.skip_blocked_reason && (
                <Text
                  mt={2}
                  color="error.700"
                  fontSize="xs"
                  fontWeight="650"
                  textAlign="right"
                >
                  {item.skip_blocked_reason}
                </Text>
              )}
            </Box>
          )}
          {paused && (
            <Button
              w="full"
              minH="44px"
              leftIcon={<FiPlay />}
              colorScheme="neutral"
              isLoading={isMutating}
              loadingText="正在继续"
              isDisabled={!item.can_resume}
              onClick={onResume}
            >
              继续解析
            </Button>
          )}
          {failed && (
            <Button
              w="full"
              minH="44px"
              leftIcon={<FiRefreshCw />}
              variant="outline"
              borderColor="error.200"
              color="error.700"
              onClick={onRetry}
            >
              重试
            </Button>
          )}
          {!running && !failed && !paused && (
            <Button
              w="full"
              minH="44px"
              rightIcon={<FiArrowUpRight />}
              colorScheme={succeeded ? "green" : "orange"}
              variant={succeeded ? "solid" : "outline"}
              onClick={onOpen}
            >
              核验解析结果
            </Button>
          )}
          <Flex
            mt={4}
            pt={3}
            align="center"
            justify="space-between"
            gap={3}
            borderTop="1px solid"
            borderColor="workbench.line"
          >
            <Text
              color="workbench.muted"
              fontSize="xs"
              fontFamily="mono"
              sx={{ fontVariantNumeric: "tabular-nums" }}
            >
              创建于 {formatDate(item.created_at)}
            </Text>
            <Tooltip label="删除项目">
              <IconButton
                aria-label={`删除 ${item.name}`}
                icon={<FiTrash2 />}
                variant="ghost"
                color="neutral.400"
                minW="44px"
                minH="44px"
                onClick={onDelete}
                _hover={{ bg: "error.50", color: "error.600" }}
              />
            </Tooltip>
          </Flex>
        </Box>
      </Flex>
    </DataSurface>
  );
}

function ControlOption({
  value,
  title,
  description,
  disabled = false,
  disabledReason,
  danger = false,
  inputRef,
}: {
  value: V3ControlMode;
  title: string;
  description: string;
  disabled?: boolean;
  disabledReason?: string;
  danger?: boolean;
  inputRef?: RefObject<HTMLInputElement>;
}) {
  return (
    <Box
      as="label"
      p={3}
      border="1px solid"
      borderColor={danger ? "error.200" : "workbench.line"}
      borderRadius="10px"
      bg={danger ? "error.50" : "white"}
      opacity={disabled ? 0.58 : 1}
      cursor={disabled ? "not-allowed" : "pointer"}
      _hover={
        disabled
          ? undefined
          : {
              borderColor: danger ? "error.400" : "primary.300",
              bg: danger ? "error.50" : "primary.50",
            }
      }
    >
      <Radio
        ref={inputRef}
        value={value}
        isDisabled={disabled}
        colorScheme={danger ? "red" : "primary"}
        alignItems="flex-start"
      >
        <Box ml={1}>
          <Text
            fontWeight="700"
            color={danger ? "error.900" : "workbench.text"}
          >
            {title}
          </Text>
          <Text
            mt={1}
            fontSize="xs"
            lineHeight="1.65"
            color={danger ? "error.800" : "workbench.muted"}
          >
            {disabled && disabledReason ? disabledReason : description}
          </Text>
        </Box>
      </Radio>
    </Box>
  );
}

function RetryOption({
  value,
  title,
  description,
  disabled = false,
}: {
  value: string;
  title: string;
  description: string;
  disabled?: boolean;
}) {
  return (
    <Box
      as="label"
      p={3}
      border="1px solid"
      borderColor="workbench.line"
      borderRadius="10px"
      opacity={disabled ? 0.55 : 1}
      cursor={disabled ? "not-allowed" : "pointer"}
      _hover={
        disabled ? undefined : { borderColor: "primary.300", bg: "primary.50" }
      }
    >
      <Radio value={value} isDisabled={disabled} alignItems="flex-start">
        <Box ml={1}>
          <Text fontWeight="700" color="workbench.text">
            {title}
          </Text>
          <Text mt={1} fontSize="xs" lineHeight="1.65" color="workbench.muted">
            {description}
          </Text>
        </Box>
      </Radio>
    </Box>
  );
}
