"use client";

/* eslint-disable no-use-before-define, no-nested-ternary, no-unused-vars */

/*
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V5
 * Hallmark · genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app
 * No parser jobs or duplicate progress appear in the completed-result workspace.
 */

import {
  Badge,
  Box,
  Button,
  Flex,
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
  Select,
  Skeleton,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Text,
  Tooltip,
  useBreakpointValue,
  useToast,
} from "@chakra-ui/react";
import { motion, useReducedMotion } from "framer-motion";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  FiAlertTriangle,
  FiArrowLeft,
  FiChevronLeft,
  FiChevronRight,
  FiEye,
  FiEyeOff,
  FiLink2,
  FiSave,
} from "react-icons/fi";

import PDFViewer from "@/components/common/viewer/pdf-viewer";
import { useSidebarCollapse } from "@/components/layout";
import {
  DataSurface,
  SignalBadge,
  WorkspaceShell,
} from "@/components/analysis/bid-analysis-v3/workspace";
import {
  BlueprintPanel,
  ClausesPanel,
  FieldsPanel,
  OverviewPanel,
} from "@/components/analysis/bid-analysis-v3/detail/panels";
import {
  buildDetailTabs,
  CLAUSE_NOISE_TITLE_RE,
} from "@/components/analysis/bid-analysis-v3/detail/model.mjs";
import { AiDisclaimerBanner } from "@/components/analysis/bid-analysis-v3/detail/ai-disclaimer-banner";
import type {
  BlueprintViewStatus,
  Category,
  Chapter,
  Clause,
  DetailTabKey,
  Evidence,
  FieldValue,
  FollowItem,
  SourceTable,
  Summary,
  WarningGroup,
} from "@/components/analysis/bid-analysis-v3/detail/types";
import {
  useAnalysisMutation,
  useAnalysisProject,
} from "@/service/bid-analysis";
import type { V3BlueprintData } from "@/service/bid-analysis";

const MotionBox = motion(Box);
const DETAIL_TAB_KEYS: DetailTabKey[] = [
  "overview",
  "fields",
  "clauses",
  "blueprint",
];

function normalizeDetail(input: any) {
  const categories: Category[] = Array.isArray(input?.categories)
    ? input.categories.map((category: any) => ({
        ...category,
        fields: Array.isArray(category?.fields)
          ? category.fields.map((field: any) => ({
              ...field,
              values: Array.isArray(field?.values)
                ? field.values.map((value: any) => ({
                    ...value,
                    evidences: Array.isArray(value?.evidences)
                      ? value.evidences
                      : [],
                  }))
                : [],
            }))
          : [],
      }))
    : [];
  const summary = input?.summary?.summary;
  if (summary && Array.isArray(summary.risks)) {
    summary.risks = summary.risks.map((item: any, index: number) =>
      typeof item === "string"
        ? { index, text: item, kind: "unknown", label: "", resolved: false }
        : {
            index,
            text: item?.text ?? "",
            kind: item?.kind || "unknown",
            label: item?.label || "",
            resolved: Boolean(item?.resolved),
          },
    );
  }
  return {
    ...input,
    categories,
    chapters: Array.isArray(input?.chapters) ? input.chapters : [],
    warning_groups: Array.isArray(input?.warning_groups)
      ? input.warning_groups
      : [],
    follows: Array.isArray(input?.follows) ? input.follows : [],
  };
}

export default function AnalysisVerificationPage() {
  const id = String(useParams()?.id || "");
  const router = useRouter();
  const toast = useToast();
  const reducedMotion = useReducedMotion();
  const isMobile = useBreakpointValue({ base: true, lg: false }) ?? false;
  const { collapseForDetail, restoreSidebar } = useSidebarCollapse();
  const [detail, setDetail] = useState<any>(null);
  const [activeTab, setActiveTab] = useState<DetailTabKey>("overview");
  const [categoryKey, setCategoryKey] = useState("");
  const [clauseChapterKey, setClauseChapterKey] = useState(0);
  const [pdfOpen, setPdfOpen] = useState(false);
  const [leftPercent, setLeftPercent] = useState(42);
  const [resizing, setResizing] = useState(false);
  const [selectedValue, setSelectedValue] = useState<FieldValue | null>(null);
  const [selectedLabel, setSelectedLabel] = useState("");
  const [evidenceIndex, setEvidenceIndex] = useState(0);
  const [selectedSourceTableId, setSelectedSourceTableId] = useState(0);
  const [selectedClauseId, setSelectedClauseId] = useState(0);
  const [highlightedClauseId, setHighlightedClauseId] = useState(0);
  const [editing, setEditing] = useState<FieldValue | null>(null);
  const [draft, setDraft] = useState("");
  const [blueprintData, setBlueprintData] = useState<V3BlueprintData | null>(
    null,
  );
  const [blueprintLoading, setBlueprintLoading] = useState(true);
  const [blueprintLoadError, setBlueprintLoadError] = useState("");
  const { loading, execute: loadDetail } = useAnalysisProject(id);
  const { loading: mutating, execute: mutate } = useAnalysisMutation();
  const { loading: blueprintMutating, execute: blueprintMutate } =
    useAnalysisMutation();

  useEffect(() => {
    collapseForDetail();
    return restoreSidebar;
  }, [collapseForDetail, restoreSidebar]);

  const refresh = useCallback(async () => {
    try {
      const response = await loadDetail();
      const next = normalizeDetail(response?.data?.data);
      const status = next?.project?.status;
      if (
        status &&
        status !== "succeeded" &&
        status !== "succeeded_with_warnings"
      ) {
        toast({
          status: "info",
          title:
            status === "running"
              ? "项目仍在解析中"
              : status === "paused"
                ? "项目已暂停，请先在列表页继续解析"
                : "请先在列表页处理解析失败",
        });
        router.replace("/bid-analysis");
        return;
      }
      setDetail(next);
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "加载解析结果失败",
      });
    }
  }, [loadDetail, router, toast]);

  const loadBlueprint = useCallback(
    async (showLoading = false) => {
      if (showLoading) setBlueprintLoading(true);
      setBlueprintLoadError("");
      try {
        const response = await blueprintMutate({
          url: `/zb/v3/projects/${id}/blueprint`,
          method: "GET",
        });
        setBlueprintData(response?.data?.data || null);
      } catch (error: any) {
        setBlueprintLoadError(
          error?.response?.data?.msg || "暂时无法读取蓝图状态，请重新加载。",
        );
      } finally {
        setBlueprintLoading(false);
      }
    },
    [blueprintMutate, id],
  );

  useEffect(() => {
    refresh().catch(() => undefined);
  }, [refresh]);
  useEffect(() => {
    loadBlueprint(true).catch(() => undefined);
  }, [loadBlueprint]);
  useEffect(() => {
    if (
      blueprintData?.status !== "pending" &&
      blueprintData?.status !== "running"
    )
      return undefined;
    const timer = window.setInterval(
      () => loadBlueprint(false).catch(() => undefined),
      2500,
    );
    return () => window.clearInterval(timer);
  }, [blueprintData?.status, loadBlueprint]);
  const categories: Category[] = useMemo(
    () => detail?.categories || [],
    [detail?.categories],
  );
  useEffect(() => {
    if (!categoryKey && categories[0]?.key) setCategoryKey(categories[0].key);
  }, [categories, categoryKey]);
  const activeCategory =
    categories.find((category) => category.key === categoryKey) ||
    categories[0];
  const notFoundFields = categories
    .flatMap((category) => category.fields)
    .filter(
      (field) =>
        field.extract_status === "not_found" && !field.ai_interpretation,
    );
  const fieldCount = categories
    .flatMap((category) => category.fields)
    .filter(
      (field) =>
        field.extract_status !== "not_found" || Boolean(field.ai_interpretation),
    ).length;
  const chapters: Chapter[] = detail?.chapters || [];
  useEffect(() => {
    if (!clauseChapterKey) {
      const first = chapters.find((chapter) =>
        (chapter.clauses || []).some(
          (clause) => !CLAUSE_NOISE_TITLE_RE.test(clause.title),
        ),
      );
      if (first) setClauseChapterKey(first.id);
    }
  }, [chapters, clauseChapterKey]);
  const clauses = chapters
    .flatMap((chapter) => chapter.clauses || [])
    .filter((clause) => !CLAUSE_NOISE_TITLE_RE.test(clause.title));
  const summary: Summary = detail?.summary?.summary || {};
  const sourceTables: SourceTable[] = detail?.source_tables || [];
  const warningGroups: WarningGroup[] = detail?.warning_groups || [];
  const follows: FollowItem[] = detail?.follows || [];
  const followedFieldIds = new Set(
    follows
      .filter((item) => item.target_type === "field")
      .map((item) => item.target_id),
  );
  const followedClauseIds = new Set(
    follows
      .filter((item) => item.target_type === "clause")
      .map((item) => item.target_id),
  );
  const warningSeverity = warningGroups.some(
    (group) => group.severity === "critical",
  )
    ? "critical"
    : warningGroups.some((group) => group.severity === "warning")
      ? "warning"
      : null;
  const selectedSourceTable = sourceTables.find(
    (table) => table.id === selectedSourceTableId,
  );
  const project = detail?.project || {};
  const [retryingStage, setRetryingStage] = useState("");
  const [resolvingWarning, setResolvingWarning] = useState<string | null>(
    null,
  );
  const [resolvingRiskIndex, setResolvingRiskIndex] = useState<number | null>(
    null,
  );
  const [followingKey, setFollowingKey] = useState<string | null>(null);
  const handleRetryStage = useCallback(
    async (stage: string) => {
      const runId = detail?.project?.current_run_id;
      if (!runId) return;
      setRetryingStage(stage);
      try {
        await mutate({
          url: `/zb/v3/projects/${id}/retry-stage`,
          method: "POST",
          data: { run_id: runId, stage },
        });
        toast({
          status: "success",
          title: "已开始阶段重试",
          description: "解析重新运行中，请返回列表页查看进度。",
        });
        router.replace("/bid-analysis");
      } catch (error: any) {
        toast({
          status: "error",
          title: error?.response?.data?.msg || "阶段重试失败，请稍后重试",
        });
      } finally {
        setRetryingStage("");
      }
    },
    [detail?.project?.current_run_id, id, mutate, router, toast],
  );
  const handleResolveWarning = useCallback(
    async (group: WarningGroup) => {
      const key = group.group_key || `id:${group.id}`;
      setResolvingWarning(key);
      try {
        await mutate({
          url: `/zb/v3/projects/${id}/warnings/resolve`,
          method: "PATCH",
          data: group.group_key
            ? { group_keys: [group.group_key] }
            : { ids: [group.id] },
        });
        await refresh();
        toast({
          status: "success",
          title: "告警已标记为解决",
        });
      } catch (error: any) {
        toast({
          status: "error",
          title: error?.response?.data?.msg || "标记失败，请稍后重试",
        });
      } finally {
        setResolvingWarning(null);
      }
    },
    [id, mutate, refresh, toast],
  );
  const handleResolveAllInfo = useCallback(async () => {
    setResolvingWarning("all-info");
    try {
      await mutate({
        url: `/zb/v3/projects/${id}/warnings/resolve`,
        method: "PATCH",
        data: { severity: "info" },
      });
      await refresh();
      toast({
        status: "success",
        title: "系统提示已全部忽略",
      });
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "操作失败，请稍后重试",
      });
    } finally {
      setResolvingWarning(null);
    }
  }, [id, mutate, refresh, toast]);
  const handleResolveRisk = useCallback(
    async (index: number, resolved: boolean, kind = "unknown") => {
      setResolvingRiskIndex(index);
      try {
        await mutate({
          url: `/zb/v3/projects/${id}/summary-risks/${index}`,
          method: "PATCH",
          data: { resolved },
        });
        await refresh();
        toast({
          status: "success",
          title: resolved
            ? kind === "risk"
              ? "已标记为已知悉"
              : "已标记为解决"
            : "已撤销标记",
        });
      } catch (error: any) {
        toast({
          status: "error",
          title:
            error?.response?.data?.msg ||
            (resolved ? "标记失败，请稍后重试" : "撤销失败，请稍后重试"),
        });
      } finally {
        setResolvingRiskIndex(null);
      }
    },
    [id, mutate, refresh, toast],
  );
  const handleToggleFollow = useCallback(
    async (targetType: "field" | "clause", targetId: number) => {
      const key = `${targetType}:${targetId}`;
      const existing = follows.find(
        (item) => item.target_type === targetType && item.target_id === targetId,
      );
      setFollowingKey(key);
      try {
        if (existing) {
          await mutate({
            url: `/zb/v3/projects/${id}/follows/${existing.id}`,
            method: "DELETE",
          });
          toast({ status: "info", title: "已取消关注" });
        } else {
          await mutate({
            url: `/zb/v3/projects/${id}/follows`,
            method: "POST",
            data: { target_type: targetType, target_id: targetId },
          });
          toast({ status: "success", title: "已添加到我的关注" });
        }
        await refresh();
      } catch (error: any) {
        toast({
          status: error?.response?.status === 409 ? "warning" : "error",
          title: error?.response?.data?.msg || "操作失败，请稍后重试",
        });
      } finally {
        setFollowingKey(null);
      }
    },
    [follows, id, mutate, refresh, toast],
  );
  const handleRemoveFollow = useCallback(
    async (followId: number) => {
      setFollowingKey(`follow:${followId}`);
      try {
        await mutate({
          url: `/zb/v3/projects/${id}/follows/${followId}`,
          method: "DELETE",
        });
        await refresh();
        toast({ status: "info", title: "已取消关注" });
      } catch (error: any) {
        toast({
          status: "error",
          title: error?.response?.data?.msg || "取消关注失败，请稍后重试",
        });
      } finally {
        setFollowingKey(null);
      }
    },
    [id, mutate, refresh, toast],
  );
  const handleJumpToClause = useCallback(
    (clauseId: number) => {
      setActiveTab("clauses");
      const hostChapter = chapters.find((chapter) =>
        (chapter.clauses || []).some((clause) => clause.id === clauseId),
      );
      if (hostChapter) setClauseChapterKey(hostChapter.id);
      window.setTimeout(
        () =>
          document.getElementById(`clause-${clauseId}`)?.scrollIntoView({
            behavior: reducedMotion ? "auto" : "smooth",
            block: "center",
          }),
        160,
      );
    },
    [chapters, reducedMotion],
  );
  const handleJumpToChapter = useCallback(
    (chapterId: number) => {
      setActiveTab("clauses");
      setClauseChapterKey(chapterId);
    },
    [],
  );
  const handleJumpToFields = useCallback(() => setActiveTab("fields"), []);
  const tabIndex = Math.max(0, DETAIL_TAB_KEYS.indexOf(activeTab));
  const blueprintViewStatus: BlueprintViewStatus | null = blueprintLoadError
    ? "load_failed"
    : blueprintData?.status || null;
  const detailTabs = useMemo(
    () =>
      buildDetailTabs({
        warningCount: project.warning_count || 0,
        warningSeverity,
        fieldCount,
        clauseCount: clauses.length,
        blueprintStatus: blueprintViewStatus,
      }),
    [
      blueprintViewStatus,
      clauses.length,
      fieldCount,
      project.warning_count,
      warningSeverity,
    ],
  );
  const evidences = useMemo(
    () => selectedValue?.evidences || [],
    [selectedValue?.evidences],
  );
  const currentEvidence = evidences[evidenceIndex];

  const highlightAreas = useMemo(
    () => ({
      highlight_areas: [
        ...evidences.map((evidence, index) => ({
          top: evidence.bbox_top,
          left: evidence.bbox_left,
          width: evidence.bbox_width || 1,
          height: evidence.bbox_height || 0.025,
          page_index: Math.max(0, evidence.page_no - 1),
          active: !selectedSourceTable && index === evidenceIndex,
        })),
        ...(selectedSourceTable
          ? (selectedSourceTable.regions?.length
              ? selectedSourceTable.regions
              : [
                  {
                    page_no: selectedSourceTable.page_start,
                    left: selectedSourceTable.bbox_left,
                    top: selectedSourceTable.bbox_top,
                    width: selectedSourceTable.bbox_width,
                    height: selectedSourceTable.bbox_height,
                  },
                ]
            ).map((region) => ({
              top: region.top,
              left: region.left,
              width: region.width || 1,
              height: region.height || 0.025,
              page_index: Math.max(0, region.page_no - 1),
              active: true,
            }))
          : []),
      ],
    }),
    [evidenceIndex, evidences, selectedSourceTable],
  );

  useEffect(() => {
    if (!resizing) return undefined;
    const move = (event: MouseEvent) =>
      setLeftPercent(
        Math.min(68, Math.max(30, (event.clientX / window.innerWidth) * 100)),
      );
    const up = () => setResizing(false);
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
    return () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
  }, [resizing]);

  const showEvidence = (
    label: string,
    evidenceItems: Evidence[],
    sourceValue?: FieldValue,
  ) => {
    setSelectedLabel(label);
    setSelectedValue(
      sourceValue || {
        id: -1,
        display_value: label,
        origin: "ai",
        status: "active",
        confidence: "",
        is_user_edited: false,
        needs_evidence: false,
        evidence_count: evidenceItems.length,
        evidences: evidenceItems,
      },
    );
    setEvidenceIndex(0);
    setSelectedSourceTableId(0);
    setPdfOpen(true);
  };

  const openSourceDocument = () => {
    setSelectedLabel("");
    setSelectedValue(null);
    setEvidenceIndex(0);
    setSelectedSourceTableId(0);
    setSelectedClauseId(0);
    setPdfOpen(true);
  };

  const saveEdit = async () => {
    if (!editing || !draft.trim()) return;
    try {
      const response = await mutate({
        url: `/zb/v3/field-values/${editing.id}`,
        method: "PATCH",
        data: { display_value: draft.trim(), normalized_value: null },
      });
      const created = response?.data?.data;
      setEditing(null);
      await refresh();
      if (created)
        showEvidence(created.display_value, [], {
          ...created,
          evidences: [],
          evidence_count: 0,
        });
      toast({
        status: "success",
        title: "人工值已保存",
        description: "请在原文中选择一处或多处证据。",
      });
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "保存失败",
      });
    }
  };

  const bindSelection = async (selection: any) => {
    if (!selectedValue || selectedValue.id <= 0) return;
    try {
      await mutate({
        url: `/zb/v3/field-values/${selectedValue.id}/evidences`,
        method: "POST",
        data: { evidences: [{ source_kind: "manual_text", ...selection }] },
      });
      const response = await mutate({
        url: `/zb/v3/field-values/${selectedValue.id}/evidences`,
        method: "GET",
      });
      setSelectedValue((current) =>
        current
          ? {
              ...current,
              needs_evidence: false,
              evidences: response?.data?.data || [],
              evidence_count: response?.data?.data?.length || 0,
            }
          : current,
      );
      toast({ status: "success", title: "证据已绑定" });
      await refresh();
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "绑定证据失败",
      });
    }
  };

  const previewSourceTable = (tableId: number) => {
    setSelectedSourceTableId(tableId);
  };

  const bindSourceTable = async () => {
    if (!selectedValue || selectedValue.id <= 0 || !selectedSourceTableId)
      return;
    try {
      await mutate({
        url: `/zb/v3/field-values/${selectedValue.id}/evidences`,
        method: "POST",
        data: {
          evidences: [
            { source_kind: "manual_table", table_id: selectedSourceTableId },
          ],
        },
      });
      const response = await mutate({
        url: `/zb/v3/field-values/${selectedValue.id}/evidences`,
        method: "GET",
      });
      setSelectedValue((current) =>
        current
          ? {
              ...current,
              needs_evidence: false,
              evidences: response?.data?.data || [],
              evidence_count: response?.data?.data?.length || 0,
            }
          : current,
      );
      setSelectedSourceTableId(0);
      toast({ status: "success", title: "原文表格已绑定" });
      await refresh();
    } catch (error: any) {
      toast({
        status: "error",
        title: error?.response?.data?.msg || "绑定表格证据失败",
      });
    }
  };

  if (loading && !detail)
    return (
      <WorkspaceShell>
        <Skeleton h="76px" borderRadius="14px" />
        <Skeleton mt={4} h="68vh" borderRadius="14px" />
      </WorkspaceShell>
    );
  if (!detail) return null;

  const verificationContent = (
    <Box h="full" minW={0} display="flex" flexDirection="column">
      <AiDisclaimerBanner />
      <Tabs
        index={tabIndex}
        onChange={(index) => setActiveTab(DETAIL_TAB_KEYS[index] || "overview")}
        display="flex"
        flexDirection="column"
        flex={1}
        minH={0}
        isLazy
        lazyBehavior="unmount"
      >
        <DataSurface p={1} flexShrink={0} boxShadow="sm" overflow="hidden">
          <TabList
            border={0}
            gap={1}
            flexWrap="wrap"
            aria-label="招标解析详情"
          >
            {detailTabs.map((tab: any, index: number) => {
              const selected = index === tabIndex;
              const badgeStyle = selected
                ? { bg: "whiteAlpha.200", color: "white" }
                : tab.tone === "warning"
                  ? { bg: "warning.100", color: "warning.800" }
                  : tab.tone === "error"
                    ? { bg: "error.100", color: "error.800" }
                    : tab.tone === "success"
                      ? { bg: "success.100", color: "success.800" }
                      : tab.tone === "info"
                        ? { bg: "info.100", color: "info.800" }
                        : { bg: "neutral.100", color: "neutral.600" };
              return (
                <Tab
                  key={tab.key}
                  minH="52px"
                  px={{ base: 3, md: 4 }}
                  flexShrink={0}
                  position="relative"
                  isolation="isolate"
                  border={0}
                  borderRadius="12px"
                  color="workbench.muted"
                  fontWeight="680"
                  whiteSpace="nowrap"
                  transition="color 160ms cubic-bezier(0.16,1,0.3,1), transform 100ms cubic-bezier(0.16,1,0.3,1)"
                  _hover={{
                    bg: selected ? "transparent" : "primary.50",
                    color: selected ? "white" : "workbench.text",
                  }}
                  _selected={{ color: "white" }}
                  _active={{ transform: "translateY(1px)" }}
                  _focusVisible={{
                    boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
                  }}
                >
                  {selected && (
                    <MotionBox
                      layoutId="analysis-detail-active-tab"
                      position="absolute"
                      inset={0}
                      zIndex={-1}
                      borderRadius="12px"
                      bg="workbench.control"
                      transition={
                        reducedMotion
                          ? { duration: 0 }
                          : { duration: 0.22, ease: [0.16, 1, 0.3, 1] }
                      }
                    />
                  )}
                  <HStack position="relative" zIndex={1} spacing={2}>
                    <Text as="span">{tab.label}</Text>
                    {tab.badge ? (
                      <Badge
                        {...badgeStyle}
                        px={1.5}
                        minW="22px"
                        textAlign="center"
                        borderRadius="full"
                        fontFamily="mono"
                        fontSize="10px"
                        textTransform="none"
                        aria-label={tab.statusLabel}
                      >
                        {tab.badge}
                      </Badge>
                    ) : null}
                  </HStack>
                </Tab>
              );
            })}
          </TabList>
        </DataSurface>
        <TabPanels
          flex={1}
          minH={0}
          overflowY={{ base: "visible", lg: "auto" }}
          className="thin-scrollbars"
        >
          <TabPanel px={0} py={4}>
            <MotionBox
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: reducedMotion ? 0 : 0.15 }}
            >
              <OverviewPanel
                summary={summary}
                categories={categories}
                clauseCount={clauses.length}
                warningCount={project.warning_count || 0}
                warningGroups={warningGroups}
                retryingStage={retryingStage}
                resolvingWarning={resolvingWarning}
                onRetryStage={handleRetryStage}
                onResolveWarning={handleResolveWarning}
                onResolveAllInfo={handleResolveAllInfo}
                onJumpChapter={handleJumpToChapter}
                onJumpFields={handleJumpToFields}
                onResolveRisk={handleResolveRisk}
                resolvingRiskIndex={resolvingRiskIndex}
                follows={follows}
                followingKey={followingKey}
                onRemoveFollow={handleRemoveFollow}
                onJumpClause={handleJumpToClause}
              />
            </MotionBox>
          </TabPanel>
          <TabPanel px={0} py={4}>
            <MotionBox
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: reducedMotion ? 0 : 0.15 }}
            >
              <FieldsPanel
                categories={categories}
                activeKey={categoryKey}
                onCategory={setCategoryKey}
                activeCategory={activeCategory}
                notFoundFields={notFoundFields}
                activeValueId={selectedValue?.id}
                sourceOpen={pdfOpen}
                onEvidence={(label, evidenceItems, value) => {
                  setSelectedClauseId(0);
                  showEvidence(label, evidenceItems, value);
                }}
                onEdit={(value) => {
                  setEditing(value);
                  setDraft(value.display_value);
                }}
                followedFieldIds={followedFieldIds}
                onToggleFollow={(field) =>
                  handleToggleFollow("field", field.id)
                }
                followingKey={followingKey}
              />
            </MotionBox>
          </TabPanel>
          <TabPanel px={0} py={4}>
            <MotionBox
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: reducedMotion ? 0 : 0.15 }}
            >
              <ClausesPanel
                chapters={chapters}
                activeChapterId={clauseChapterKey}
                onChapter={setClauseChapterKey}
                activeClauseId={selectedClauseId}
                highlightedClauseId={highlightedClauseId}
                sourceOpen={pdfOpen}
                onEvidence={(clause) => {
                  setSelectedClauseId(clause.id);
                  showEvidence(clause.title, clause.evidences || []);
                }}
                onOpenFields={() => setActiveTab("fields")}
                onOpenSource={openSourceDocument}
                followedClauseIds={followedClauseIds}
                onToggleFollow={(clause) =>
                  handleToggleFollow("clause", clause.id)
                }
                followingKey={followingKey}
              />
            </MotionBox>
          </TabPanel>
          <TabPanel px={0} py={4}>
            <MotionBox
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: reducedMotion ? 0 : 0.15 }}
            >
              <BlueprintPanel
                projectId={id}
                data={blueprintData}
                loading={blueprintLoading}
                loadError={blueprintLoadError}
                mutate={blueprintMutate}
                mutating={blueprintMutating}
                reload={() => loadBlueprint(true)}
                onOpenClause={(clauseId) => {
                  setHighlightedClauseId(clauseId);
                  setActiveTab("clauses");
                  const hostChapter = chapters.find((chapter) =>
                    (chapter.clauses || []).some(
                      (clause) => clause.id === clauseId,
                    ),
                  );
                  if (hostChapter) setClauseChapterKey(hostChapter.id);
                  window.setTimeout(
                    () =>
                      document
                        .getElementById(`clause-${clauseId}`)
                        ?.scrollIntoView({
                          behavior: reducedMotion ? "auto" : "smooth",
                          block: "center",
                        }),
                    160,
                  );
                  window.setTimeout(
                    () =>
                      setHighlightedClauseId((current) =>
                        current === clauseId ? 0 : current,
                      ),
                    2400,
                  );
                }}
                onOpenEvidence={(ids) => {
                  const wanted = new Set(ids);
                  const matches = clauses
                    .flatMap((clause) => clause.evidences || [])
                    .filter((evidence) => wanted.has(evidence.id));
                  if (matches.length) {
                    setSelectedClauseId(0);
                    showEvidence("标书蓝图关联证据", matches);
                  }
                }}
              />
            </MotionBox>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </Box>
  );

  const pdfPanel = (
    <Flex
      display={pdfOpen ? "flex" : "none"}
      position={{ base: "fixed", lg: "relative" }}
      inset={{ base: 0, lg: "auto" }}
      zIndex={{ base: 1600, lg: "auto" }}
      flex={1}
      minW={0}
      h={{ base: "100dvh", lg: "full" }}
      direction="column"
      bg="workbench.canvas"
      borderRadius={{ base: 0, lg: "14px" }}
      overflow="hidden"
      border={{ base: 0, lg: "1px solid" }}
      borderColor="workbench.line"
      sx={{
        ".rpv-core__inner-pages": {
          overscrollBehaviorY: "contain",
        },
      }}
    >
      <Flex
        minH="58px"
        px={3}
        py={2}
        bg="white"
        borderBottom="1px solid"
        borderColor="workbench.line"
        align="center"
        justify="space-between"
        gap={3}
      >
        <Box minW={0}>
          <Text fontWeight="700" noOfLines={1}>
            {selectedLabel || "招标文件原文"}
          </Text>
          <Text fontSize="xs" color="workbench.muted" noOfLines={1}>
            {currentEvidence
              ? `第 ${currentEvidence.page_no} 页 · ${currentEvidence.quote || "来源位置"}`
              : "可浏览全文，也可从字段或条款直接定位来源"}
          </Text>
        </Box>
        <HStack flexShrink={0}>
          {evidences.length > 0 && (
            <HStack spacing={0}>
              <IconButton
                aria-label="上一处证据"
                icon={<FiChevronLeft />}
                variant="ghost"
                minW="44px"
                minH="44px"
                isDisabled={evidenceIndex <= 0}
                onClick={() =>
                  setEvidenceIndex((value) => Math.max(0, value - 1))
                }
              />
              <Text
                minW="54px"
                textAlign="center"
                fontFamily="mono"
                fontSize="sm"
              >
                {evidenceIndex + 1} / {evidences.length}
              </Text>
              <IconButton
                aria-label="下一处证据"
                icon={<FiChevronRight />}
                variant="ghost"
                minW="44px"
                minH="44px"
                isDisabled={evidenceIndex >= evidences.length - 1}
                onClick={() =>
                  setEvidenceIndex((value) =>
                    Math.min(evidences.length - 1, value + 1),
                  )
                }
              />
            </HStack>
          )}
          <Tooltip label="收起原文">
            <IconButton
              aria-label="收起原文"
              icon={<FiEyeOff />}
              minW="44px"
              minH="44px"
              variant="ghost"
              onClick={() => setPdfOpen(false)}
            />
          </Tooltip>
        </HStack>
      </Flex>
      {selectedValue && selectedValue.id > 0 && sourceTables.length > 0 && (
        <Flex
          px={3}
          py={2}
          gap={2}
          align={{ base: "stretch", sm: "center" }}
          direction={{ base: "column", sm: "row" }}
          bg="white"
          borderBottom="1px solid"
          borderColor="workbench.line"
        >
          <Select
            aria-label="选择已识别的原文表格"
            value={selectedSourceTableId || ""}
            onChange={(event) => previewSourceTable(Number(event.target.value))}
            size="sm"
            h="44px"
          >
            <option value="">选择已识别表格 · {sourceTables.length} 张</option>
            {sourceTables.map((table) => (
              <option key={table.id} value={table.id}>
                P{table.page_start}
                {table.page_end > table.page_start
                  ? `–${table.page_end}`
                  : ""}{" "}
                ·{" "}
                {table.caption ||
                  `${table.row_count}×${table.column_count} 表格`}
              </option>
            ))}
          </Select>
          <Button
            minH="44px"
            flexShrink={0}
            isDisabled={!selectedSourceTableId}
            isLoading={mutating}
            onClick={bindSourceTable}
          >
            绑定整张表格
          </Button>
        </Flex>
      )}
      <Box flex={1} minH={0} position="relative">
        <PDFViewer
          url={`/api/zb/v3/projects/${id}/source-pdf`}
          pdfH="100%"
          highlightAreas={highlightAreas}
          onTextSelection={bindSelection}
        />
        {selectedValue?.id && selectedValue.id > 0 && (
          <Box
            position="absolute"
            right={3}
            bottom={3}
            px={3}
            py={2}
            borderRadius="9px"
            bg="workbench.control"
            color="white"
            boxShadow="0 8px 24px rgba(11,27,43,.24)"
            pointerEvents="none"
          >
            <HStack>
              <Icon as={FiLink2} color="gold.300" />
              <Text fontSize="xs">拖选原文可绑定为新证据</Text>
            </HStack>
          </Box>
        )}
      </Box>
    </Flex>
  );

  return (
    <WorkspaceShell
      maxWidth="none"
      h={{ base: "auto", lg: "100dvh" }}
      minH={{ base: "100%", lg: 0 }}
      p={{ base: 2, md: 4 }}
      contentProps={{ h: { base: "auto", lg: "full" }, minH: 0 }}
    >
      <Flex h="100%" minH={0} direction="column" gap={3}>
        <Flex
          minH={{ base: "116px", md: "92px" }}
          px={{ base: 3, md: 5 }}
          py={{ base: 3, md: 4 }}
          align={{ base: "stretch", md: "center" }}
          justify="space-between"
          direction={{ base: "column", md: "row" }}
          gap={3}
          borderRadius="16px"
          bg="workbench.control"
          color="white"
          boxShadow="lg"
        >
          <HStack minW={0} align="flex-start">
            <IconButton
              aria-label="返回招标解析列表"
              icon={<FiArrowLeft />}
              variant="ghost"
              minW="44px"
              minH="44px"
              color="white"
              _hover={{ bg: "whiteAlpha.200", color: "white" }}
              _focusVisible={{
                boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
              }}
              onClick={() => router.push("/bid-analysis")}
            />
            <Box minW={0}>
              <HStack align="flex-start" wrap="wrap" spacing={2}>
                <SignalBadge status={project.status} />
                <Text
                  fontWeight="780"
                  fontSize={{ base: "md", md: "xl" }}
                  lineHeight="1.35"
                  noOfLines={2}
                  color="white"
                >
                  {project.name}
                </Text>
              </HStack>
              <HStack
                mt={2}
                spacing={2}
                color="whiteAlpha.700"
                fontSize="xs"
                wrap="wrap"
              >
                <Text noOfLines={1} maxW={{ base: "240px", md: "640px" }}>
                  {project.source_file_name}
                </Text>
                {project.warning_count > 0 && (
                  <Badge
                    display="inline-flex"
                    alignItems="center"
                    gap={1}
                    bg="warning.100"
                    color="warning.800"
                    textTransform="none"
                    borderRadius="full"
                  >
                    <Icon as={FiAlertTriangle} boxSize={3} />
                    {project.warning_count} 条告警
                  </Badge>
                )}
              </HStack>
            </Box>
          </HStack>
          <HStack flexShrink={0} alignSelf={{ base: "flex-end", md: "center" }}>
            {!pdfOpen && (
              <Button
                display={{ base: "none", lg: "inline-flex" }}
                minH="44px"
                leftIcon={<FiEye />}
                bg="white"
                color="workbench.control"
                border="1px solid"
                borderColor="whiteAlpha.500"
                _hover={{ bg: "primary.50", transform: "translateY(-1px)" }}
                _active={{ transform: "translateY(0)" }}
                _focusVisible={{
                  boxShadow: "0 0 0 3px var(--chakra-colors-gold-300)",
                }}
                onClick={() => setPdfOpen(true)}
              >
                查看原文
              </Button>
            )}
          </HStack>
        </Flex>
        <Flex
          flex={1}
          minH={{ base: "auto", lg: 0 }}
          gap={0}
          overflow={{ base: "visible", lg: "hidden" }}
        >
          <Box
            flex={pdfOpen && !isMobile ? `0 0 ${leftPercent}%` : "1 1 100%"}
            minW={0}
            minH={0}
            h={{ base: "auto", lg: "full" }}
            overflow={{ base: "visible", lg: "hidden" }}
          >
            {verificationContent}
          </Box>
          {pdfOpen && !isMobile && (
            <Box
              w="6px"
              flexShrink={0}
              cursor="col-resize"
              bg={resizing ? "gold.400" : "transparent"}
              _hover={{ bg: "gold.300" }}
              _focusVisible={{ bg: "gold.400", outline: "none" }}
              role="separator"
              aria-label="调整核验结果与原文面板宽度"
              aria-orientation="vertical"
              aria-valuemin={30}
              aria-valuemax={68}
              aria-valuenow={Math.round(leftPercent)}
              tabIndex={0}
              onMouseDown={() => setResizing(true)}
              onKeyDown={(event) => {
                if (event.key === "ArrowLeft") {
                  event.preventDefault();
                  setLeftPercent((value) => Math.max(30, value - 2));
                }
                if (event.key === "ArrowRight") {
                  event.preventDefault();
                  setLeftPercent((value) => Math.min(68, value + 2));
                }
              }}
            />
          )}
          {pdfPanel}
        </Flex>
      </Flex>

      <Button
        display={{ base: "inline-flex", lg: "none" }}
        position="fixed"
        zIndex={1500}
        left={3}
        right={3}
        bottom={3}
        minH="52px"
        leftIcon={pdfOpen ? <FiEyeOff /> : <FiEye />}
        bg="workbench.control"
        color="white"
        boxShadow="0 12px 34px rgba(11,27,43,.28)"
        onClick={() => setPdfOpen((value) => !value)}
        _hover={{ bg: "workbench.controlRaised" }}
      >
        {pdfOpen ? "收起原文" : "查看原文"}
      </Button>

      <Modal
        isOpen={Boolean(editing)}
        onClose={() => setEditing(null)}
        isCentered
      >
        <ModalOverlay />
        <ModalContent mx={3} borderRadius="14px">
          <ModalHeader>修订字段值</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Text fontSize="sm" color="workbench.muted">
              原 AI 值及证据会保留为建议历史。保存新值后，请重新绑定来源。
            </Text>
            <Input
              mt={4}
              minH="48px"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              autoFocus
            />
          </ModalBody>
          <ModalFooter gap={2}>
            <Button onClick={() => setEditing(null)}>取消</Button>
            <Button
              colorScheme="primary"
              leftIcon={<FiSave />}
              isLoading={mutating}
              onClick={saveEdit}
            >
              保存
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

    </WorkspaceShell>
  );
}
