/* Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4 */

"use client";

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Box,
  Flex,
  Icon,
  Text,
  Badge,
  Button,
  IconButton,
  Spinner,
  useToast,
  Alert,
  AlertIcon,
  Switch,
  Tooltip,
  useDisclosure,
} from "@chakra-ui/react";
import {
  FiChevronDown,
  FiChevronUp,
  FiDownload,
  FiList,
  FiRefreshCw,
  FiTool,
} from "react-icons/fi";
import { useReducedMotion } from "framer-motion";
import { useRouter } from "next/navigation";
import { BackButton } from "@/components/common/header-controls";

import {
  useReviewDetail,
  useReviewItemUpdate,
  useReviewItemRecheck,
  useReviewItemAdd,
  useReviewItemDelete,
  useReviewExportReport,
  useReviewRetryStage,
  useReviewAnonymousToggle,
  useRemediationUpdate,
  useRuleFromItem,
} from "@/service/audit";
import CheckListPanel from "./check-list-panel";
import SourcePdfPanel, { SourcePdfHandle } from "./source-pdf-panel";
import { buildReviewSourcePdfUrl } from "./source-pdf-url.mjs";
import EdgeEar from "./edge-ear";
import RemediationPanel from "./remediation-panel";
import StagePanel from "./stage-panel";
import {
  ChecklistItem,
  DimensionStat,
  DIMENSION_META,
  DIMENSION_ORDER,
  Remediation,
  ReviewFile,
  ReviewProject,
  STAGE_LABELS,
  StageInfo,
  TraceRef,
  getRetryableStage,
} from "./types";

function statusBadge(project: ReviewProject) {
  if (project.status === "running")
    return (
      <Badge colorScheme="info" borderRadius="full" px={2}>
        审核中
      </Badge>
    );
  if (project.status === "failed")
    return (
      <Badge colorScheme="error" borderRadius="full" px={2}>
        审核失败
      </Badge>
    );
  if (project.error_items > 0)
    return (
      <Badge colorScheme="error" borderRadius="full" px={2}>
        存在告警
      </Badge>
    );
  if (project.warning_items > 0)
    return (
      <Badge colorScheme="warning" borderRadius="full" px={2}>
        需关注
      </Badge>
    );
  return (
    <Badge colorScheme="success" borderRadius="full" px={2}>
      已通过
    </Badge>
  );
}

/* Hallmark · component: stat-pill (bid-audit detail) · genre: modern-minimal · theme: chakra-smart-bid (preserved tokens)
 * states: default (static summary, no interaction)
 */
function StatPill({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: string;
}) {
  const colors: Record<string, string> = {
    primary: "var(--chakra-colors-primary-600)",
    error: "var(--chakra-colors-error-500)",
    warning: "var(--chakra-colors-warning-500)",
    success: "var(--chakra-colors-success-500)",
  };
  return (
    <Flex
      align="center"
      gap={1}
      bg="neutral.50"
      borderRadius="full"
      px={2.5}
      py={0.5}
      border="1px solid"
      borderColor="neutral.100"
      flexShrink={0}
    >
      <Box w={1.5} h={1.5} borderRadius="full" bg={colors[tone]} />
      <Text fontSize="11px" fontWeight="700" color="neutral.700">
        {value}
      </Text>
      <Text fontSize="10px" color="neutral.400">
        {label}
      </Text>
    </Flex>
  );
}

export default function AuditDetail({ projectId }: { projectId: string }) {
  const router = useRouter();
  const toast = useToast();

  const { detailData, detailError, detailLoading, fetchDetail } =
    useReviewDetail(projectId);
  const { fetchUpdate } = useReviewItemUpdate();
  const { fetchRecheck } = useReviewItemRecheck();
  const { fetchAdd } = useReviewItemAdd();
  const { fetchDelete: fetchDeleteItem } = useReviewItemDelete();
  const { exportLoading, fetchExport } = useReviewExportReport();
  const { retryLoading, fetchRetryStage } = useReviewRetryStage();
  const { anonymousLoading, fetchToggle } = useReviewAnonymousToggle();
  const { fetchUpdate: fetchRemediationUpdate } = useRemediationUpdate();
  const { fetchSave: fetchRuleFromItem } = useRuleFromItem();

  const [leftPercent, setLeftPercent] = useState(42);
  const [isResizing, setIsResizing] = useState(false);
  const [tenderCollapsed, setTenderCollapsed] = useState(false);
  const [bidCollapsed, setBidCollapsed] = useState(false);
  const [highlightItemId, setHighlightItemId] = useState<number | null>(null);
  // 头部默认折叠：只保留返回 + 项目名与右侧三个操作入口，释放垂直空间
  const [headerCollapsed, setHeaderCollapsed] = useState(true);
  const {
    isOpen: remediationIsOpen,
    onOpen: remediationOnOpen,
    onClose: remediationOnClose,
  } = useDisclosure();

  const bothCollapsed = tenderCollapsed && bidCollapsed;

  // 两侧都折叠时，预览区整体向右缘耳朵渐收（与面板内折叠动画同节奏）；拖拽调宽时禁用过渡
  const reduceMotion = useReducedMotion();
  const foldDur = reduceMotion ? "0.15s" : "0.5s";
  const foldEase = reduceMotion ? "ease" : "cubic-bezier(0.4, 0, 0.2, 1)";
  const layoutTransition = isResizing
    ? "none"
    : `flex-basis ${foldDur} ${foldEase}, min-width ${foldDur} ${foldEase}`;
  const dividerTransition = isResizing
    ? "background 0.2s"
    : `width ${foldDur} ${foldEase}, background 0.2s`;

  const sourceRef = useRef<SourcePdfHandle>(null);

  const project: ReviewProject | null = detailData?.project || null;
  const files: ReviewFile[] = useMemo(
    () => detailData?.files || [],
    [detailData?.files],
  );
  const items: ChecklistItem[] = useMemo(
    () => detailData?.items || [],
    [detailData?.items],
  );
  const dimensions: DimensionStat[] = useMemo(
    () => detailData?.dimensions || [],
    [detailData?.dimensions],
  );
  const remediations: Remediation[] = useMemo(
    () => detailData?.remediations || [],
    [detailData?.remediations],
  );
  const stages: StageInfo[] = useMemo(
    () => detailData?.stages || [],
    [detailData?.stages],
  );
  const [loadingStage, setLoadingStage] = useState<string | null>(null);
  const [recheckingDim, setRecheckingDim] = useState<string | null>(null);

  // 审核进度成功完成后才展示检查项；running/failed 时左栏置空（仅提示）
  const reviewCompleted = project?.status === "succeed";
  const visibleItems = useMemo(
    () => (reviewCompleted ? items : []),
    [items, reviewCompleted],
  );

  const tenderFiles = useMemo(
    () => files.filter((f) => f.file_type === "tender"),
    [files],
  );
  const bidFiles = useMemo(
    () => files.filter((f) => f.file_type === "bid"),
    [files],
  );

  // 审核中每 5s 刷新
  const running = project?.status === "running";
  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => fetchDetail(), 5000);
    return () => window.clearInterval(timer);
  }, [running, fetchDetail]);

  // 统计：以后端维度汇总为准（判定结论统一在服务端聚合）
  const stats = useMemo(() => {
    const empty = { total: 0, passed: 0, warning: 0, error: 0, notFound: 0 };
    for (const st of dimensions) {
      empty.total += st.total;
      empty.passed += st.passed;
      empty.warning += st.warning;
      empty.error += st.error;
      empty.notFound += st.not_found;
    }
    return empty;
  }, [dimensions]);

  const failedStage = getRetryableStage(project);

  const handleTrace = useCallback(
    (refs: TraceRef[], side: "tender" | "bid") => {
      sourceRef.current?.jumpToTrace(refs, side);
    },
    [],
  );

  const handleUpdateItem = useCallback(
    async (itemId: number, reviewStatus: string, note: string) => {
      const res = await fetchUpdate({
        data: { item_id: itemId, review_status: reviewStatus, note },
      });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "更新失败");
      }
      fetchDetail();
    },
    [fetchUpdate, fetchDetail],
  );

  const handleRecheckItem = useCallback(
    async (itemId: number) => {
      const res = await fetchRecheck({
        // projectId 来自路由参数是字符串，接口按 int64 解析，必须显式转数字
        data: { project_id: Number(projectId), item_id: itemId },
      });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "复检失败");
      }
      fetchDetail();
    },
    [fetchRecheck, projectId, fetchDetail],
  );

  const handleAddItem = useCallback(
    async (data: {
      dimension: string;
      category: string;
      title: string;
      requirement: string;
      expected_evidence: string;
      severity: string;
    }) => {
      const res = await fetchAdd({
        data: { project_id: Number(projectId), ...data },
      });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "添加失败");
      }
      fetchDetail();
    },
    [fetchAdd, projectId, fetchDetail],
  );

  const handleDeleteItem = useCallback(
    async (itemId: number) => {
      const res = await fetchDeleteItem({ data: { item_id: itemId } });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "删除失败");
      }
      fetchDetail();
    },
    [fetchDeleteItem, fetchDetail],
  );

  const handleExport = useCallback(async () => {
    try {
      const res = await fetchExport({
        params: { project_id: projectId },
      });
      if (res?.data?.code === 200 && res?.data?.data?.file_url) {
        window.open(res.data.data.file_url, "_blank");
        toast({ title: "报告已生成", status: "success" });
      } else {
        toast({ title: res?.data?.msg || "导出失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "导出失败", status: "error" });
    }
  }, [fetchExport, projectId, toast]);

  // 整批复检：某个维度全部重新判定（投标文件更新后使用）
  const handleRecheckDimension = useCallback(
    async (dimension: string) => {
      setRecheckingDim(dimension);
      try {
        const res = await fetchRecheck({
          data: { project_id: Number(projectId), dimension },
        });
        if (res?.data?.code === 200) {
          toast({
            title: `已重新判定${DIMENSION_META[dimension]?.label || dimension}`,
            status: "success",
          });
          fetchDetail();
        } else {
          toast({ title: res?.data?.msg || "整批复检失败", status: "error" });
        }
      } catch (e: any) {
        toast({ title: e?.response?.data?.msg || "整批复检失败", status: "error" });
      } finally {
        setRecheckingDim(null);
      }
    },
    [fetchRecheck, projectId, toast, fetchDetail],
  );

  const handleSaveRule = useCallback(
    async (itemId: number) => {
      const res = await fetchRuleFromItem({ data: { item_id: itemId } });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "沉淀失败");
      }
    },
    [fetchRuleFromItem],
  );

  const handleRemediationUpdate = useCallback(
    async (id: number, payload: { status?: string; note?: string }) => {
      const res = await fetchRemediationUpdate({ data: { id, ...payload } });
      if (res?.data?.code !== 200) {
        throw new Error(res?.data?.msg || "更新失败");
      }
      fetchDetail();
    },
    [fetchRemediationUpdate, fetchDetail],
  );

  const handleAnonymousToggle = useCallback(
    async (next: boolean) => {
      try {
        const res = await fetchToggle({
          url: `/zb/review/project/anonymous/${projectId}`,
          data: { is_anonymous: next ? "true" : "false" },
        });
        if (res?.data?.code === 200) {
          toast({ title: next ? "已开启暗标检查" : "已关闭暗标检查", status: "success" });
          fetchDetail();
        } else {
          toast({ title: res?.data?.msg || "更新失败", status: "error" });
        }
      } catch (e: any) {
        toast({ title: e?.response?.data?.msg || "更新失败", status: "error" });
      }
    },
    [fetchToggle, projectId, toast, fetchDetail],
  );

  const jumpToItem = useCallback((itemId: number) => {
    setHighlightItemId(itemId);
    remediationOnClose();
    window.setTimeout(() => setHighlightItemId(null), 2600);
  }, [remediationOnClose]);

  const handleRetryStage = useCallback(async () => {
    if (!failedStage) return;
    try {
      const res = await fetchRetryStage({
        url: `/zb/review/project/retry-stage/${projectId}`,
        data: { stage: failedStage },
      });
      if (res?.data?.code === 200) {
        toast({ title: "已从该阶段重跑", status: "success" });
        fetchDetail();
      } else {
        toast({ title: res?.data?.msg || "重试失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "重试失败", status: "error" });
    }
  }, [failedStage, fetchRetryStage, projectId, toast, fetchDetail]);

  // 分阶段重跑：从指定阶段起重置并重新执行（该阶段及后续阶段）
  const handleRetryStageAt = useCallback(
    async (stage: string) => {
      setLoadingStage(stage);
      try {
        const res = await fetchRetryStage({
          url: `/zb/review/project/retry-stage/${projectId}`,
          data: { stage },
        });
        if (res?.data?.code === 200) {
          toast({
            title: `已从${STAGE_LABELS[stage] || stage}重跑`,
            status: "success",
          });
          fetchDetail();
        } else {
          toast({ title: res?.data?.msg || "重跑失败", status: "error" });
        }
      } catch (e: any) {
        toast({ title: e?.response?.data?.msg || "重跑失败", status: "error" });
      } finally {
        setLoadingStage(null);
      }
    },
    [fetchRetryStage, projectId, toast, fetchDetail],
  );

  const handleCancelTask = useCallback(async () => {
    try {
      const res = await fetchRetryStage({
        url: `/zb/review/project/cancel/${projectId}`,
        data: {},
      });
      if (res?.data?.code === 200) {
        toast({ title: "已请求取消，将在当前阶段结束后停止", status: "success" });
        fetchDetail();
      } else {
        toast({ title: res?.data?.msg || "取消失败", status: "error" });
      }
    } catch (e: any) {
      toast({ title: e?.response?.data?.msg || "取消失败", status: "error" });
    }
  }, [fetchRetryStage, projectId, toast, fetchDetail]);

  // 拖拽调宽
  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      setIsResizing(true);
      const startX = e.clientX;
      const startPercent = leftPercent;
      const onMove = (ev: MouseEvent) => {
        const dx = ev.clientX - startX;
        const next = Math.min(
          70,
          Math.max(25, startPercent + (dx / window.innerWidth) * 100),
        );
        setLeftPercent(next);
      };
      const onUp = () => {
        setIsResizing(false);
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [leftPercent],
  );

  if (detailLoading && !detailData) {
    return (
      <Flex h="full" align="center" justify="center">
        <Spinner size="lg" color="primary.500" />
      </Flex>
    );
  }
  if (!project) {
    return (
      <Flex h="full" align="center" justify="center" direction="column" gap={3}>
        <Text color="neutral.500">
          {detailError
            ? (detailError as any)?.response?.data?.msg ||
              "详情加载失败，请稍后重试"
            : "项目不存在或已被删除"}
        </Text>
        <Button size="sm" onClick={() => router.push("/bid-audit")}>
          返回列表
        </Button>
      </Flex>
    );
  }

  return (
    <Flex
      minH="full"
      direction="column"
      gap={3}
      p={{ base: 2, md: 4 }}
      bg="workbench.canvas"
    >
      {/* 头部：[返回] 名称 + 折叠开关 + 暗标开关 / 整改清单 / 导出；展开时补充徽章与统计胶囊 */}
      <Box
        bg="workbench.control"
        color="workbench.paper"
        border="1px solid"
        borderColor="workbench.controlRaised"
        borderRadius="16px"
        boxShadow="lg"
        px={{ base: 3, md: 5 }}
        py={3}
        flexShrink={0}
      >
        <Flex align="center" gap={2.5} flexWrap="wrap" rowGap={2}>
          <BackButton href="/bid-audit" inverted />
          <Text
            fontSize="lg"
            fontWeight="700"
            color="workbench.paper"
            noOfLines={1}
            title={project.name}
            minW={0}
            flex={headerCollapsed ? "1 1 auto" : "0 1 auto"}
          >
            {project.name}
          </Text>
          <IconButton
            aria-label={headerCollapsed ? "展开项目信息" : "折叠项目信息"}
            icon={headerCollapsed ? <FiChevronDown /> : <FiChevronUp />}
            size="xs"
            variant="ghost"
            color="whiteAlpha.800"
            _hover={{ bg: "whiteAlpha.200" }}
            onClick={() => setHeaderCollapsed((v) => !v)}
            flexShrink={0}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.300",
              outlineOffset: "2px",
            }}
          />
          {!headerCollapsed && (
            <>
              <Badge
                variant="subtle"
                bg="whiteAlpha.200"
                color="whiteAlpha.900"
                borderRadius="full"
                flexShrink={0}
              >
                {project.create_type === "gen" ? "从标书生成" : "上传创建"}
              </Badge>
              {statusBadge(project)}
              <Flex gap={2} align="center" flexWrap="wrap" rowGap={2} ml={1}>
                <StatPill label="检查项" value={stats.total} tone="primary" />
                <StatPill label="高风险" value={stats.error} tone="error" />
                <StatPill label="需整改" value={stats.warning} tone="warning" />
                <StatPill label="未找到依据" value={stats.notFound} tone="warning" />
                <StatPill label="通过" value={stats.passed} tone="success" />
              </Flex>
            </>
          )}
          <Box flex={headerCollapsed ? "0 0 auto" : 1} />
          <Flex
            align="center"
            gap={2}
            px={2.5}
            py={1}
            borderRadius="full"
            bg="whiteAlpha.200"
            flexShrink={0}
          >
            <Text
              fontSize="11px"
              color="whiteAlpha.800"
              // 折叠态在窄屏隐藏文字，只保留开关，保证三个入口按钮不被挤走
              display={
                headerCollapsed ? { base: "none", md: "block" } : "block"
              }
            >
              暗标评审
            </Text>
            <Switch
              size="sm"
              colorScheme="purple"
              isChecked={project.is_anonymous}
              isDisabled={anonymousLoading || !reviewCompleted}
              onChange={(e) => handleAnonymousToggle(e.target.checked)}
            />
          </Flex>
          <Button
            size="sm"
            leftIcon={<FiTool />}
            variant="outline"
            // 整改项属于风险处置入口：统一用危险色主题，待整改数量用红点徽标提示
            color="error.200"
            borderColor="error.400"
            _hover={{ bg: "whiteAlpha.200", borderColor: "error.300" }}
            _active={{ bg: "whiteAlpha.300" }}
            isDisabled={!reviewCompleted}
            onClick={remediationOnOpen}
            flexShrink={0}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "error.300",
              outlineOffset: "2px",
            }}
          >
            整改清单
            {project.todo_items > 0 && (
              <Badge
                ml={2}
                bg="error.500"
                color="workbench.paper"
                borderRadius="full"
                fontSize="10px"
                px={1.5}
                style={{ fontVariantNumeric: "tabular-nums" }}
              >
                {project.todo_items}
              </Badge>
            )}
          </Button>
          {/* Hallmark · component: export-report-button · genre: modern-minimal · theme: chakra-smart-bid
           * states: default · hover · focus-visible · active · disabled · loading · error · success
           * contrast: pass — warm gold highlight stays distinct from the deep-sea header
           */}
          <Button
            size="sm"
            leftIcon={<FiDownload />}
            colorScheme="primary"
            bg="workbench.paper"
            color="workbench.control"
            borderColor="whiteAlpha.600"
            transition="background 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease"
            _hover={{
              bg: "gold.50",
              color: "workbench.control",
              borderColor: "gold.400",
              boxShadow: "0 0 0 1px var(--chakra-colors-gold-400)",
              transform: "translateY(-1px)",
            }}
            _active={{
              bg: "gold.100",
              borderColor: "gold.500",
              boxShadow: "0 0 0 1px var(--chakra-colors-gold-500)",
              transform: "translateY(0)",
            }}
            _focusVisible={{
              outline: "2px solid",
              outlineColor: "gold.300",
              outlineOffset: "2px",
            }}
            _disabled={{
              bg: "whiteAlpha.700",
              color: "whiteAlpha.600",
              borderColor: "whiteAlpha.300",
              boxShadow: "none",
              transform: "none",
              cursor: "not-allowed",
            }}
            isDisabled={!reviewCompleted}
            isLoading={exportLoading}
            onClick={handleExport}
            flexShrink={0}
          >
            导出报告
          </Button>
        </Flex>
      </Box>

      {/* 失败提示 */}
      {project.status === "failed" && (
        <Alert status="error" size="sm" py={2.5} px={5} borderRadius="10px">
          <AlertIcon />
          <Text fontSize="12px" flex={1}>
            {project.last_error || "审核失败，请重试失败阶段"}
          </Text>
          {failedStage && (
            <Button
              size="xs"
              colorScheme="error"
              leftIcon={<FiRefreshCw />}
              onClick={handleRetryStage}
              isLoading={retryLoading}
            >
              重试
            </Button>
          )}
        </Alert>
      )}

      {/* 任务阶段：分阶段执行 + 断点重跑 + 取消 */}
      {stages.length > 0 && (
        <Box flexShrink={0}>
          <StagePanel
            stages={stages}
            projectStatus={project.status}
            runCount={project.run_count}
            startedAt={project.started_at}
            finishedAt={project.finished_at}
            loadingStage={loadingStage}
            onRetryStage={handleRetryStageAt}
            onCancel={handleCancelTask}
          />
        </Box>
      )}

      {/* 整批复检：投标文件更新后只重跑某个维度的判定（清单筛选在上方清单的维度标签里，这里不是筛选） */}
      {reviewCompleted && (
        <Flex
          gap={2}
          flexWrap="wrap"
          align="center"
          rowGap={2}
          flexShrink={0}
          px={3}
          py={2}
          borderRadius="10px"
          bg="neutral.50"
          border="1px solid"
          borderColor="neutral.100"
        >
          <Icon as={FiRefreshCw} color="workbench.muted" boxSize={3.5} flexShrink={0} />
          <Text fontSize="11px" color="neutral.600">
            投标文件更新后，可对某个维度整批重新判定；筛选清单请用上方清单里的维度标签
          </Text>
          {DIMENSION_ORDER.map((dim) => (
            <Tooltip
              key={dim}
              label={`重新判定${DIMENSION_META[dim].label}下的全部检查项`}
              hasArrow
              openDelay={200}
            >
              <Button
                size="xs"
                variant="outline"
                leftIcon={<FiList size={11} />}
                isLoading={recheckingDim === dim}
                onClick={() => handleRecheckDimension(dim)}
                _focusVisible={{
                  outline: "2px solid",
                  outlineColor: "gold.400",
                  outlineOffset: "2px",
                }}
              >
                {DIMENSION_META[dim].label}
              </Button>
            </Tooltip>
          ))}
        </Flex>
      )}

      {/* 主体：左检查清单 + 右双栏原文；两侧都折叠时预览区整体渐收、左栏平滑占满 */}
      <Flex
        flex="none"
        h={{ base: "72dvh", lg: "min(74dvh, 720px)" }}
        minH={0}
        position="relative"
        direction={{ base: "column", lg: "row" }}
        overflow={{ base: "auto", lg: "hidden" }}
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="14px"
        bg="workbench.paper"
      >
        <Box
          flex={{
            base: bothCollapsed ? "1 1 auto" : "0 0 min(52dvh, 34rem)",
            lg: `0 0 ${bothCollapsed ? "100%" : `${leftPercent}%`}`,
          }}
          minW={0}
          minH={{ base: bothCollapsed ? 0 : "20rem", lg: 0 }}
          bg="white"
          overflow="hidden"
          display="flex"
          flexDirection="column"
          transition={layoutTransition}
        >
          <CheckListPanel
            items={visibleItems}
            loading={detailLoading}
            notCompleted={!reviewCompleted}
            highlightItemId={highlightItemId}
            onTrace={handleTrace}
            onUpdate={handleUpdateItem}
            onRecheck={handleRecheckItem}
            onDelete={handleDeleteItem}
            onSaveRule={handleSaveRule}
            onAddItem={handleAddItem}
          />
        </Box>

        <Box
          display={{ base: "none", lg: "block" }}
          w={bothCollapsed ? "0px" : "4px"}
          cursor={bothCollapsed ? "default" : "col-resize"}
          bg={isResizing ? "primary.400" : "transparent"}
          _hover={bothCollapsed ? undefined : { bg: "primary.200" }}
          transition={dividerTransition}
          onMouseDown={bothCollapsed ? undefined : handleMouseDown}
          flexShrink={0}
          overflow="hidden"
        />

        <Box
          flex={{
            base: bothCollapsed ? "0 0 0" : "0 0 min(52dvh, 34rem)",
            lg: `0 0 ${
              bothCollapsed ? "0px" : `calc(100% - ${leftPercent}% - 4px)`
            }`,
          }}
          minW={0}
          minH={{ base: bothCollapsed ? 0 : "20rem", lg: 0 }}
          bg="neutral.100"
          overflow="hidden"
          transition={layoutTransition}
        >
          <SourcePdfPanel
            ref={sourceRef}
            tenderFiles={tenderFiles}
            bidFiles={bidFiles}
            buildUrl={(fileId) => buildReviewSourcePdfUrl(projectId, fileId)}
            tenderCollapsed={tenderCollapsed}
            bidCollapsed={bidCollapsed}
            onTenderCollapsedChange={setTenderCollapsed}
            onBidCollapsedChange={setBidCollapsed}
          />
        </Box>

        {/* 右缘召唤耳：折叠侧显示 招/投 */}
        {tenderCollapsed && (
          <EdgeEar
            char="招"
            label="展开招标文件"
            top={bidCollapsed ? "calc(50% - 24px)" : "50%"}
            onClick={() => setTenderCollapsed(false)}
          />
        )}
        {bidCollapsed && (
          <EdgeEar
            char="投"
            label="展开投标文件"
            top={tenderCollapsed ? "calc(50% + 24px)" : "50%"}
            onClick={() => setBidCollapsed(false)}
          />
        )}
      </Flex>

      <RemediationPanel
        isOpen={remediationIsOpen}
        onClose={remediationOnClose}
        remediations={remediations}
        onUpdate={handleRemediationUpdate}
        onJumpToItem={jumpToItem}
      />
    </Flex>
  );
}
