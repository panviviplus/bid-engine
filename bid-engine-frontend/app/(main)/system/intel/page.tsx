"use client";

/* eslint-disable no-nested-ternary */

/* Hallmark · macrostructure: Workbench · genre: modern-minimal · theme: BidEngine deep-sea/gold (preserved tokens)
 * audience: 系统超管 · use: 招标情报站的一站式运维（源 / 批次 / 定时任务） · tone: technical, austere
 * 三个子 tab 对应三种管理职责，右上角常驻“手动采集一轮”与“管理自动采集任务”。
 * Hallmark · pre-emit critique: P5 H4 E5 S5 R5 V4
 */
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import {
  Badge,
  Box,
  Button,
  Collapse,
  Divider,
  Flex,
  Grid,
  HStack,
  IconButton,
  Input,
  ListItem,
  Skeleton,
  Stack,
  Switch,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Table,
  TableContainer,
  Tabs,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tooltip,
  Tr,
  UnorderedList,
  useToast,
  useDisclosure,
} from "@chakra-ui/react";
import { FiRefreshCw, FiTrash2, FiUpload, FiZap } from "react-icons/fi";

import EmptyState from "@/components/common/empty-state";
import ColumnConfig from "@/components/common/column-config";
import DeleteConfirmModal from "@/components/common/delete-confirm-modal";
import { PageContent, PageViewport } from "@/components/layout/responsive-page";
import { useAppContext } from "@/contexts/app-context";
import { ModuleWorkbenchHeader } from "@/components/common/module-workbench.mjs";
import SourceEditorModal, {
  toSourcePayload,
  type SourceFormValue,
} from "@/components/intel/admin/source-editor-modal";
import SourceCard from "@/components/intel/admin/source-card";
import SourceImportModal from "@/components/intel/admin/source-import-modal";
import RunDetailModal from "@/components/intel/admin/run-detail-modal";
import NoticeManager from "@/components/intel/admin/notice-manager";
import MatchManager from "@/components/intel/admin/match-manager";
import HintToggle from "@/components/intel/admin/hint-toggle";
import {
  useIntelRuns,
  useIntelRunDelete,
  useIntelRunRetry,
  useIntelSchedule,
  useIntelScheduleSave,
  useIntelSourceDelete,
  useIntelSourceProbe,
  useIntelSourceSave,
  useIntelSources,
  useIntelTriggerCollect,
  type IntelRun,
  type IntelSource,
} from "@/service/intel";

const FIELD_H = "44px";

/**
 * 后端业务失败会返回 HTTP 200 + code != 0（例如“已有采集批次正在执行”）。
 * 若不显式判断，页面会把失败当成功弹提示，用户只能看到“已启动”却查不到批次。
 */
function assertBizOK(res: any, fallback = "操作失败") {
  const code = res?.data?.code;
  if (code === undefined || code === null || code === 0) return;
  throw new Error(res?.data?.message || fallback);
}

function bizErrorMessage(error: any, fallback = "请稍后重试") {
  return (
    error?.response?.data?.message ||
    error?.data?.message ||
    error?.message ||
    fallback
  );
}

/** 执行中批次的实时口径：采集阶段还没有入库数，打标阶段已入库但仍在跑模型。 */
function runningRunHint(run: IntelRun) {
  const started = Date.parse((run.started_at || "").replace(" ", "T"));
  const elapsedMin = Number.isFinite(started)
    ? Math.max(0, Math.round((Date.now() - started) / 60000))
    : null;
  const parts = [
    run.inserted_count > 0
      ? `已入库 ${run.inserted_count} 条，正在打标`
      : "正在采集源站",
  ];
  if (elapsedMin !== null) parts.push(`已运行 ${elapsedMin} 分钟`);
  return parts.join(" · ");
}

const RUN_STATUS_COLOR: Record<string, string> = {
  running: "info",
  success: "success",
  partial: "warning",
  failed: "error",
};

const RUN_STATUS_LABEL: Record<string, string> = {
  running: "执行中",
  success: "成功",
  partial: "部分成功",
  failed: "失败",
};

const TAB_KEYS = ["sources", "notices", "matches", "schedule", "runs"];
const TAB_LABELS = [
  "采集源管理",
  "情报管理",
  "订阅匹配",
  "采集任务配置",
  "采集任务历史",
];

// 采集任务历史的可配置列：默认只渲染“批次 / 采集范围 / 状态 / 触发方式 / 操作”，
// 选择结果按用户记在 localStorage，下次进入自动只渲染已选列。
const RUN_COLUMN_STORAGE_KEY = "bid-engine:intel-run-columns";
const RUN_COLUMN_OPTIONS = [
  { label: "批次" },
  { label: "采集范围" },
  { label: "状态" },
  { label: "触发方式" },
  { label: "源（成功/总数）" },
  { label: "发现" },
  { label: "新增" },
  { label: "重复" },
  { label: "打标" },
  { label: "匹配提醒" },
  { label: "错误摘要" },
  { label: "操作" },
];
const RUN_DEFAULT_COLUMNS = ["批次", "采集范围", "状态", "触发方式", "操作"];
const RUN_MANDATORY_COLUMNS = ["批次", "操作"];

// 每列的最小宽度（px）：列宽本身随容器自适应，只有容器装不下这些下限时，
// 才在表格容器内横向滚动——避免操作列被挤到换行。
const RUN_COLUMN_MIN_WIDTHS: Record<string, number> = {
  批次: 200,
  采集范围: 150,
  状态: 96,
  触发方式: 88,
  "源（成功/总数）": 116,
  发现: 72,
  新增: 72,
  重复: 72,
  打标: 72,
  错误摘要: 180,
  操作: 176,
};

/** 只保留合法列名，并补回必选列，避免历史脏数据把表格搞空。 */
function normalizeRunColumns(labels: string[]): string[] {
  const known = new Set(RUN_COLUMN_OPTIONS.map((item) => item.label));
  const seen = new Set<string>();
  const out: string[] = [];
  [...labels, ...RUN_MANDATORY_COLUMNS].forEach((label) => {
    if (!known.has(label) || seen.has(label)) return;
    seen.add(label);
    out.push(label);
  });
  return out.length > 0 ? out : [...RUN_DEFAULT_COLUMNS];
}

function readStoredRunColumns(storageKey: string): string[] {
  if (typeof window === "undefined") return [...RUN_DEFAULT_COLUMNS];
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return [...RUN_DEFAULT_COLUMNS];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [...RUN_DEFAULT_COLUMNS];
    return normalizeRunColumns(parsed.map((item) => String(item)));
  } catch {
    return [...RUN_DEFAULT_COLUMNS];
  }
}

/**
 * 折叠态概览里的单个统计项：彩色圆点 + 标签 + 数值。
 * 数值用等宽数字，多行数字不会左右跳动。
 */
function RunStat({
  label,
  value,
  unit,
  dot,
  tone = "neutral",
}: {
  label: string;
  value: number;
  unit?: string;
  dot?: string;
  tone?: "neutral" | "info" | "warning" | "error";
}) {
  const valueColor = tone === "neutral" ? "workbench.text" : `${tone}.600`;
  return (
    <HStack spacing={1.5} align="center" whiteSpace="nowrap">
      {dot ? <Box w="6px" h="6px" borderRadius="full" bg={dot} /> : null}
      <Text as="span" fontSize="xs" color="workbench.muted">
        {label}
      </Text>
      <Text
        as="span"
        fontSize="sm"
        fontWeight="700"
        color={valueColor}
        sx={{ fontVariantNumeric: "tabular-nums" }}
      >
        {value}
      </Text>
      {unit ? (
        <Text as="span" fontSize="xs" color="workbench.muted">
          {unit}
        </Text>
      ) : null}
    </HStack>
  );
}

export default function IntelAdminPage() {
  const router = useRouter();
  const toast = useToast();
  const { userProfile = {} } = useAppContext();
  const [tabIndex, setTabIndex] = useState(0);

  const { sources, sourcesLoading, refreshSources } = useIntelSources();
  const { runs, runsTotal, runsSummary, runsLoading, refreshRuns } =
    useIntelRuns(10);
  const { runRetryLoading, fetchRetry } = useIntelRunRetry();
  const { runDeleteLoading, fetchDelete: fetchDeleteRun } = useIntelRunDelete();
  const { schedule, scheduleLoading, refreshSchedule } = useIntelSchedule();
  const { triggerLoading, fetchTrigger } = useIntelTriggerCollect();
  const { sourceSaveLoading, fetchUpdate } = useIntelSourceSave();
  const { sourceDeleteLoading, fetchDelete } = useIntelSourceDelete();
  const { probeLoading, fetchProbe } = useIntelSourceProbe();
  const { scheduleSaveLoading, fetchSave: saveSchedule } =
    useIntelScheduleSave();

  const editor = useDisclosure();
  const importer = useDisclosure();
  const runDetail = useDisclosure();
  const deleteDialog = useDisclosure();
  const runDeleteDialog = useDisclosure();
  const [editingSource, setEditingSource] = useState<IntelSource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<IntelSource | null>(null);
  const [runDeleteTarget, setRunDeleteTarget] = useState<IntelRun | null>(null);
  const [retryBusyId, setRetryBusyId] = useState<string | null>(null);
  // 采集源说明默认折叠：折叠态只渲染状态摘要与两个操作按钮
  const [sourceHelpOpen, setSourceHelpOpen] = useState(false);
  // 采集任务历史说明默认折叠：折叠态只渲染刷新按钮
  const [runHelpOpen, setRunHelpOpen] = useState(false);
  const [visibleRunColumns, setVisibleRunColumns] =
    useState<string[]>(RUN_DEFAULT_COLUMNS);

  const currentUserId = (userProfile as { id?: string | number } | undefined)
    ?.id;
  const userKey = currentUserId ? `:${currentUserId}` : "";
  const runColumnStorageKey = `${RUN_COLUMN_STORAGE_KEY}${userKey}`;

  // 读取本机记住的列配置（按用户区分）
  useEffect(() => {
    setVisibleRunColumns(readStoredRunColumns(runColumnStorageKey));
  }, [runColumnStorageKey]);

  const setRunColumns = useCallback(
    (next: string[]) => {
      const normalized = normalizeRunColumns(next);
      setVisibleRunColumns(normalized);
      try {
        window.localStorage.setItem(
          runColumnStorageKey,
          JSON.stringify(normalized),
        );
      } catch {
        // 隐私模式等写入失败时仅当前会话生效，不阻断交互
      }
    },
    [runColumnStorageKey],
  );

  const showRunColumn = useCallback(
    (label: string) =>
      visibleRunColumns.includes(label) ||
      RUN_MANDATORY_COLUMNS.includes(label),
    [visibleRunColumns],
  );

  // 批次列表的刷新函数放进 ref：轮询 / 切 tab 只依赖布尔状态，避免 effect 反复重建。
  const refreshRunsRef = useRef(refreshRuns);
  useEffect(() => {
    refreshRunsRef.current = refreshRuns;
  }, [refreshRuns]);

  const hasRunningRun = runs.some((item) => item.status === "running");

  // 静默刷新：轮询与切 tab 触发的刷新失败不弹错误（列表本身会显示错误态）
  const silentRefreshRuns = useCallback(() => {
    const pending = refreshRunsRef.current?.();
    if (pending && typeof pending.catch === "function") {
      pending.catch(() => {});
    }
  }, []);

  // 有批次在执行中时自动轮询：触发后无需手动点“刷新”就能看到新批次与进度。
  useEffect(() => {
    if (!hasRunningRun) return;
    const timer = window.setInterval(() => {
      silentRefreshRuns();
    }, 6000);
    return () => window.clearInterval(timer);
  }, [hasRunningRun, silentRefreshRuns]);

  // 切到采集任务历史时拉一次最新批次，避免看到进入页面前的旧列表。
  useEffect(() => {
    if (TAB_KEYS[tabIndex] !== "runs") return;
    silentRefreshRuns();
  }, [tabIndex, silentRefreshRuns]);

  // 表格最小宽度 = 已选列的最小宽度之和，保证操作列永远有位置
  const runTableMinWidth = useMemo(
    () =>
      RUN_COLUMN_OPTIONS.filter((item) => showRunColumn(item.label)).reduce(
        (total, item) => total + (RUN_COLUMN_MIN_WIDTHS[item.label] || 96),
        0,
      ),
    [showRunColumn],
  );
  const [activeRun, setActiveRun] = useState<IntelRun | null>(null);
  const [probeResult, setProbeResult] = useState<Record<string, string>>({});
  const [busyKey, setBusyKey] = useState<string | null>(null);
  /**
   * 启停的乐观值：开关立即反映用户意图，不再等接口返回后整页重取，
   * 因此不会出现“列表先变骨架屏再回来”的跳变，也不会丢焦点。
   * 接口成功后等一次后台重取再清掉，保证与服务端一致。
   */
  const [pendingEnabled, setPendingEnabled] = useState<Record<string, boolean>>(
    {},
  );
  const [cronInput, setCronInput] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState(true);

  // 支持 /system/intel?tab=runs 直接落到指定子 tab
  useEffect(() => {
    if (typeof window === "undefined") return;
    const params = new URLSearchParams(window.location.search);
    const index = TAB_KEYS.indexOf(params.get("tab") || "");
    if (index >= 0) setTabIndex(index);
  }, []);

  useEffect(() => {
    if (!schedule) return;
    setCronInput(schedule.cron_expr || "");
    setScheduleEnabled(Boolean(schedule.enabled));
  }, [schedule]);

  const handleTrigger = useCallback(async () => {
    try {
      const res = await fetchTrigger({ data: { source_keys: [] } });
      assertBizOK(res, "触发采集失败");
      const runID = res?.data?.data?.run_id || "";
      const total = res?.data?.data?.source_total;
      toast({
        title: "采集任务已启动",
        description: [
          runID ? `批次 ${runID}` : "已提交采集任务",
          total ? `本次覆盖 ${total} 个启用源` : "本次覆盖全部启用源",
          "进度见“采集任务历史”。",
        ].join("，"),
        status: "success",
        duration: 5000,
        isClosable: true,
      });
      refreshRuns();
      refreshSources();
    } catch (error: any) {
      toast({
        title: "采集任务未启动",
        description: bizErrorMessage(error),
        status: "error",
        duration: 6000,
        isClosable: true,
      });
      // 失败常因上一批次仍显示执行中，刷新后能看到实际卡住的批次
      refreshRuns();
    }
  }, [fetchTrigger, refreshRuns, refreshSources, toast]);

  const handleToggleSource = async (item: IntelSource, enabled: boolean) => {
    setBusyKey(item.source_key);
    // 先乐观更新开关，失败时再回滚并提示
    setPendingEnabled((prev) => ({ ...prev, [item.source_key]: enabled }));
    try {
      const res = await fetchUpdate({
        url: `/zb/intel/sources/${item.source_key}`,
        data: { enabled },
      });
      assertBizOK(res, "操作失败");
      // 后台重取失败不影响启停结果：列表保留乐观值，下次刷新会与服务端对齐
      try {
        await refreshSources();
      } catch {
        // 静默：避免把「重取失败」误报成「启停失败」
      }
      toast({
        title: enabled ? "已启用采集源" : "已停用采集源",
        description: enabled
          ? `${item.name || item.source_key} 将参与每日自动采集`
          : `${item.name || item.source_key} 已暂停采集，历史公告保留`,
        status: "success",
        duration: 2500,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: "操作失败",
        description: bizErrorMessage(error),
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      // 无论成功还是失败都移除乐观值：成功时服务端数据已就绪，失败时需要展示真实状态
      setPendingEnabled((prev) => {
        const next = { ...prev };
        delete next[item.source_key];
        return next;
      });
      setBusyKey(null);
    }
  };

  const handleProbe = async (item: IntelSource) => {
    setBusyKey(item.source_key);
    setProbeResult((prev) => ({ ...prev, [item.source_key]: "" }));
    try {
      const res = await fetchProbe({
        url: `/zb/intel/sources/${item.source_key}/probe`,
      });
      assertBizOK(res, "探测失败");
      const data = res?.data?.data;
      const message = data?.reachable
        ? `可发现 ${data?.items_found ?? 0} 条候选公告${
            data?.sample?.length ? `，例如：${data.sample[0]}` : ""
          }`
        : `探测失败：${data?.message || "列表页不可达"}`;
      setProbeResult((prev) => ({ ...prev, [item.source_key]: message }));
      toast({
        title: data?.reachable ? "探测完成" : "探测失败",
        description: message,
        status: data?.reachable ? "success" : "warning",
        duration: 5000,
        isClosable: true,
      });
    } catch (error: any) {
      const message = bizErrorMessage(error, "探测失败");
      setProbeResult((prev) => ({ ...prev, [item.source_key]: message }));
      toast({
        title: "探测失败",
        description: message,
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setBusyKey(null);
    }
  };

  const handleCollectOne = async (item: IntelSource) => {
    setBusyKey(item.source_key);
    try {
      const res = await fetchTrigger({
        data: { source_keys: [item.source_key] },
      });
      assertBizOK(res, "触发采集失败");
      const runID = res?.data?.data?.run_id || "";
      toast({
        title: "该源采集已启动",
        description: `批次 ${runID}，仅采集 ${item.name || item.source_key}；进度见“采集任务历史”。`,
        status: "success",
        duration: 5000,
        isClosable: true,
      });
      refreshRuns();
    } catch (error: any) {
      toast({
        title: "该源采集未启动",
        description: bizErrorMessage(error),
        status: "error",
        duration: 6000,
        isClosable: true,
      });
      refreshRuns();
    } finally {
      setBusyKey(null);
    }
  };

  const handleDeleteSource = async (item: IntelSource) => {
    setBusyKey(item.source_key);
    try {
      const res = await fetchDelete({
        url: `/zb/intel/sources/${item.source_key}`,
      });
      assertBizOK(res, "删除失败");
      toast({
        title: "采集源已删除",
        description: `已删除 ${item.name || item.source_key}；如需找回，可重新导入或执行 docs/sql/tender-intel.sql 恢复内置源`,
        status: "success",
        duration: 4000,
        isClosable: true,
      });
      refreshSources();
    } catch (error: any) {
      toast({
        title: "删除失败",
        description: bizErrorMessage(error),
        status: "error",
        duration: 3500,
        isClosable: true,
      });
    } finally {
      setBusyKey(null);
      deleteDialog.onClose();
      setDeleteTarget(null);
    }
  };

  const handleSaveSource = async (value: SourceFormValue) => {
    if (!editingSource) return;
    try {
      const res = await fetchUpdate({
        url: `/zb/intel/sources/${editingSource.source_key}`,
        data: toSourcePayload(value),
      });
      assertBizOK(res, "保存失败");
      toast({
        title: "采集源已更新",
        status: "success",
        duration: 2500,
        isClosable: true,
      });
      editor.onClose();
      refreshSources();
    } catch (error: any) {
      toast({
        title: "保存失败",
        description: bizErrorMessage(error),
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    }
  };

  const handleRetryRun = async (run: IntelRun) => {
    setRetryBusyId(run.run_id);
    try {
      const res = await fetchRetry({
        url: `/zb/intel/runs/${run.run_id}/retry`,
      });
      assertBizOK(res, "重试失败");
      const data = res?.data?.data;
      toast({
        title: "已按原范围重新发起采集",
        description: `新批次 ${data?.run_id || ""}（重试自 ${run.run_id}）`,
        status: "success",
        duration: 4500,
        isClosable: true,
      });
      refreshRuns();
    } catch (error: any) {
      toast({
        title: "重试失败",
        description: bizErrorMessage(error),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setRetryBusyId(null);
    }
  };

  const handleDeleteRun = async (run: IntelRun) => {
    try {
      const res = await fetchDeleteRun({ url: `/zb/intel/runs/${run.run_id}` });
      assertBizOK(res, "删除失败");
      toast({
        title: "批次记录已删除",
        description: "仅删除执行历史，已入库公告不受影响",
        status: "success",
        duration: 3500,
        isClosable: true,
      });
      refreshRuns();
    } catch (error: any) {
      toast({
        title: "删除失败",
        description: bizErrorMessage(error),
        status: "error",
        duration: 3500,
        isClosable: true,
      });
    } finally {
      runDeleteDialog.onClose();
      setRunDeleteTarget(null);
    }
  };

  const handleSaveSchedule = async () => {
    try {
      const res = await saveSchedule({
        data: { cron_expr: cronInput.trim(), enabled: scheduleEnabled },
      });
      assertBizOK(res, "保存失败");
      const data = res?.data?.data;
      toast({
        title: scheduleEnabled ? "自动采集任务已保存" : "自动采集任务已停用",
        description: data?.next_runs?.length
          ? `下次执行：${data.next_runs[0]}`
          : "任务已停用，不再自动触发",
        status: "success",
        duration: 4000,
        isClosable: true,
      });
      refreshSchedule();
    } catch (error: any) {
      toast({
        title: "保存失败",
        description: bizErrorMessage(error, "请检查 cron 表达式"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    }
  };

  const visibleSources = useMemo(
    () =>
      sources.map((item) =>
        pendingEnabled[item.source_key] === undefined
          ? item
          : { ...item, enabled: pendingEnabled[item.source_key] },
      ),
    [sources, pendingEnabled],
  );

  const sourceCount = visibleSources.length;
  const enabledCount = useMemo(
    () => visibleSources.filter((item) => item.enabled).length,
    [visibleSources],
  );

  // 折叠态概览：优先用后端全量统计，接口未返回概览时退化为当前分页总数
  const runOverview = useMemo(
    () => ({
      total: runsSummary?.total ?? runsTotal,
      running: runsSummary?.running ?? 0,
      partial: runsSummary?.partial ?? 0,
      failed: runsSummary?.failed ?? 0,
    }),
    [runsSummary, runsTotal],
  );
  const lastRunLabel = runs[0]?.started_at
    ? `最近采集 ${runs[0].started_at}`
    : "暂无采集记录";
  const nextRunLabel = !schedule?.enabled
    ? "自动采集未启用"
    : schedule?.next_runs?.length
      ? `下次自动采集 ${schedule.next_runs[0]}`
      : "自动采集已启用";

  return (
    <PageViewport id="intel-admin-scroll" bg="workbench.canvas">
      <PageContent py={{ base: 5, md: 6 }}>
        <ModuleWorkbenchHeader title="招标情报管理" titleSuffix={null} />

        {/* 显式选中态：选中项用实心胶囊 + 深色文字，未选项为浅色描边，避免只靠下划线区分 */}
        <Tabs
          index={tabIndex}
          onChange={(index) => {
            setTabIndex(index);
            router.replace(`/system/intel?tab=${TAB_KEYS[index]}`);
          }}
          colorScheme="primary"
          variant="unstyled"
        >
          <TabList
            overflowX="auto"
            overflowY="hidden"
            gap={2}
            p={1}
            mb={5}
            bg="white"
            border="1px solid"
            borderColor="neutral.100"
            borderRadius="full"
            w="fit-content"
            maxW="100%"
          >
            {TAB_LABELS.map((label, index) => {
              const selected = tabIndex === index;
              return (
                <Tab
                  key={label}
                  h="40px"
                  px={5}
                  minW="fit-content"
                  whiteSpace="nowrap"
                  fontWeight="600"
                  fontSize="sm"
                  borderRadius="full"
                  bg={selected ? "primary.500" : "transparent"}
                  color={selected ? "white" : "neutral.600"}
                  boxShadow={
                    selected ? "0 4px 12px rgba(30, 58, 95, 0.22)" : "none"
                  }
                  transition="background-color .16s ease-out, color .16s ease-out, box-shadow .16s ease-out"
                  _hover={{
                    bg: selected ? "primary.600" : "primary.50",
                    color: selected ? "white" : "primary.700",
                  }}
                  _selected={{
                    bg: "primary.500",
                    color: "white",
                    fontWeight: "700",
                  }}
                  _focusVisible={{
                    boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                    outline: "none",
                  }}
                >
                  {label}
                </Tab>
              );
            })}
          </TabList>

          <TabPanels>
            {/* ① 采集源管理 */}
            <TabPanel px={0}>
              {/* 操作条：默认折叠，只渲染两个按钮 + 说明入口；展开后按条列出含义 */}
              <Box
                bg="white"
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="14px"
                boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
                px={{ base: 3, md: 4 }}
                py={{ base: 3, md: 3 }}
                mb={4}
              >
                <Flex
                  align={{ base: "stretch", md: "center" }}
                  justify="space-between"
                  direction={{ base: "column", md: "row" }}
                  gap={3}
                >
                  <HStack spacing={3} minW={0} order={{ base: 2, md: 1 }}>
                    <Text
                      fontSize="sm"
                      color="workbench.muted"
                      sx={{ fontVariantNumeric: "tabular-nums" }}
                      noOfLines={1}
                    >
                      共 {sourceCount} 个源 · 启用 {enabledCount} 个 · 停用{" "}
                      {Math.max(0, sourceCount - enabledCount)} 个
                    </Text>
                  </HStack>

                  <HStack
                    spacing={3}
                    order={{ base: 1, md: 2 }}
                    flexShrink={0}
                    w={{ base: "full", md: "auto" }}
                  >
                    <Button
                      flex={{ base: 1, md: "none" }}
                      h={FIELD_H}
                      variant="outline"
                      borderColor="neutral.200"
                      leftIcon={<FiZap aria-hidden />}
                      isLoading={triggerLoading}
                      loadingText="触发中"
                      whiteSpace="nowrap"
                      onClick={handleTrigger}
                      _hover={{
                        borderColor: "primary.300",
                        color: "primary.600",
                      }}
                      _active={{ transform: "scale(0.98)" }}
                      _focusVisible={{
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                        outline: "none",
                      }}
                    >
                      手动采集一轮
                    </Button>
                    <Button
                      flex={{ base: 1, md: "none" }}
                      h={FIELD_H}
                      colorScheme="primary"
                      leftIcon={<FiUpload aria-hidden />}
                      whiteSpace="nowrap"
                      onClick={importer.onOpen}
                      _active={{ transform: "scale(0.98)" }}
                    >
                      导入采集源
                    </Button>
                    {/* 说明开关固定放最右：四个子 tab 位置一致，不用每次重新找 */}
                    <HintToggle
                      label="采集说明"
                      expanded={sourceHelpOpen}
                      onToggle={() => setSourceHelpOpen((prev) => !prev)}
                    />
                  </HStack>
                </Flex>

                <Collapse in={sourceHelpOpen} animateOpacity>
                  <Divider mt={3} mb={3} borderColor="neutral.100" />
                  <UnorderedList
                    spacing={2}
                    pl={5}
                    m={0}
                    fontSize="sm"
                    color="workbench.text"
                  >
                    <ListItem>
                      启用中的源才会参与每日自动采集；停用的源不会被采集，历史公告与批次记录保留。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        探测
                      </Text>
                      ：用采集服务试抓一次列表页，确认当前能否采到公告（新导入的源建议先探测）。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        立即采集
                      </Text>
                      ：只跑这一个源的采集轮次，可在“采集任务历史”查看与重试。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        编辑
                      </Text>
                      ：修改列表地址、采集规则参数（params）与优先级；采集规则随采集服务代码版本管理。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        删除
                      </Text>
                      ：该源不再参与采集，历史公告与批次记录保留，删除前会二次确认。
                    </ListItem>
                  </UnorderedList>
                </Collapse>
              </Box>

              {sourcesLoading && sources.length === 0 ? (
                <Grid
                  templateColumns="repeat(auto-fill,minmax(min(100%,340px),1fr))"
                  gap={4}
                >
                  {[0, 1, 2, 3].map((key) => (
                    <Skeleton key={key} height="336px" borderRadius="14px" />
                  ))}
                </Grid>
              ) : sources.length === 0 ? (
                <EmptyState
                  title="没有采集源"
                  description="先执行 docs/sql/tender-intel.sql 初始化，或用“导入采集源”批量添加。"
                />
              ) : (
                <Grid
                  templateColumns="repeat(auto-fill,minmax(min(100%,340px),1fr))"
                  gap={4}
                >
                  {visibleSources.map((item) => (
                    <SourceCard
                      key={item.source_key}
                      source={item}
                      probing={probeLoading && busyKey === item.source_key}
                      collecting={triggerLoading && busyKey === item.source_key}
                      deleting={
                        sourceDeleteLoading && busyKey === item.source_key
                      }
                      toggling={
                        sourceSaveLoading && busyKey === item.source_key
                      }
                      probeText={probeResult[item.source_key]}
                      onProbe={() => handleProbe(item)}
                      onCollect={() => handleCollectOne(item)}
                      onEdit={() => {
                        setEditingSource(item);
                        editor.onOpen();
                      }}
                      onDelete={() => {
                        setDeleteTarget(item);
                        deleteDialog.onOpen();
                      }}
                      onToggle={(next) => handleToggleSource(item, next)}
                    />
                  ))}
                </Grid>
              )}
            </TabPanel>

            {/* ② 情报管理：渲染口径与情报大厅一致，另加发布、批量导入与管理操作 */}
            <TabPanel px={0}>
              <NoticeManager />
            </TabPanel>

            {/* ③ 订阅匹配：匹配任务的作业面（优先级 / 取消 / 重新入队 / 删除 / 补扫） */}
            <TabPanel px={0}>
              <MatchManager />
            </TabPanel>

            {/* ④ 采集任务配置 */}
            <TabPanel px={0}>
              <Box
                bg="white"
                borderRadius="xl"
                borderWidth="1px"
                borderColor="neutral.100"
                p={{ base: 4, md: 6 }}
                maxW="880px"
              >
                <Text fontWeight="700" color="workbench.text">
                  自动采集任务
                </Text>
                <Text mt={1} fontSize="sm" color="workbench.muted">
                  系统按 cron 表达式自动跑一轮采集（默认每天
                  06:00）。保存后立即生效，无需重启后端。
                </Text>

                <Divider my={5} />

                {scheduleLoading ? (
                  <Skeleton height="120px" borderRadius="lg" />
                ) : (
                  <Stack spacing={5}>
                    <Flex align="center" gap={4} wrap="wrap">
                      <Switch
                        colorScheme="primary"
                        isChecked={scheduleEnabled}
                        onChange={(event) =>
                          setScheduleEnabled(event.target.checked)
                        }
                        aria-label="启用自动采集"
                      />
                      <Text fontSize="sm" color="workbench.text">
                        {scheduleEnabled ? "已启用自动采集" : "已停用自动采集"}
                      </Text>
                    </Flex>

                    <Box>
                      <Text fontSize="sm" fontWeight="600" mb={2}>
                        cron 表达式（秒 分 时 日 月 周）
                      </Text>
                      <Input
                        h={FIELD_H}
                        value={cronInput}
                        onChange={(event) => setCronInput(event.target.value)}
                        fontFamily="mono"
                        placeholder="0 0 6 * * *"
                        maxW="320px"
                        borderColor="neutral.200"
                        _focusVisible={{
                          borderColor: "primary.500",
                          boxShadow:
                            "0 0 0 2px var(--chakra-colors-primary-200)",
                          outline: "none",
                        }}
                      />
                      <Text mt={2} fontSize="xs" color="workbench.muted">
                        {schedule?.example ||
                          "每天 06:00 写作 0 0 6 * * *；每天 06:00 与 18:00 写作 0 0 6,18 * * *"}
                      </Text>
                    </Box>

                    <Box>
                      <Text fontSize="sm" fontWeight="600" mb={2}>
                        下次执行时间
                      </Text>
                      {schedule?.next_runs?.length ? (
                        <Stack spacing={1}>
                          {schedule.next_runs.map((item) => (
                            <Text
                              key={item}
                              fontSize="sm"
                              color="workbench.text"
                              sx={{ fontVariantNumeric: "tabular-nums" }}
                            >
                              {item}
                            </Text>
                          ))}
                        </Stack>
                      ) : (
                        <Text fontSize="sm" color="workbench.muted">
                          暂无（任务停用或表达式尚未校验通过）
                        </Text>
                      )}
                      {schedule?.updated_at && (
                        <Text mt={2} fontSize="xs" color="workbench.muted">
                          上次修改：{schedule.updated_at}
                        </Text>
                      )}
                    </Box>

                    <HStack spacing={3}>
                      <Button
                        h={FIELD_H}
                        colorScheme="primary"
                        isLoading={scheduleSaveLoading}
                        loadingText="保存中"
                        onClick={handleSaveSchedule}
                        _active={{ transform: "scale(0.98)" }}
                      >
                        保存并生效
                      </Button>
                      <Button
                        h={FIELD_H}
                        variant="outline"
                        borderColor="neutral.200"
                        onClick={() => refreshSchedule()}
                        _hover={{
                          borderColor: "primary.300",
                          color: "primary.600",
                        }}
                      >
                        重新读取
                      </Button>
                    </HStack>
                  </Stack>
                )}
              </Box>
            </TabPanel>

            {/* ⑤ 采集任务历史 */}
            <TabPanel px={0}>
              {/* 操作条：默认折叠；折叠态左侧常驻批次概览，右侧是说明开关与刷新 */}
              <Box
                bg="white"
                border="1px solid"
                borderColor="workbench.line"
                borderRadius="14px"
                boxShadow="0 10px 30px rgba(11, 27, 43, 0.06)"
                px={{ base: 3, md: 4 }}
                py={3}
                mb={4}
              >
                <Flex
                  align={{ base: "stretch", md: "center" }}
                  justify="space-between"
                  direction={{ base: "column", md: "row" }}
                  gap={{ base: 3, md: 4 }}
                >
                  <Stack spacing={1.5} minW={0} order={{ base: 2, md: 1 }}>
                    <HStack
                      spacing={{ base: 3, md: 5 }}
                      flexWrap="wrap"
                      rowGap={1.5}
                      align="center"
                    >
                      <RunStat
                        label="共"
                        value={runOverview.total}
                        unit="个批次"
                      />
                      <RunStat
                        label="执行中"
                        value={runOverview.running}
                        dot="info.400"
                        tone={runOverview.running > 0 ? "info" : "neutral"}
                      />
                      <RunStat
                        label="部分成功"
                        value={runOverview.partial}
                        dot="warning.400"
                        tone={runOverview.partial > 0 ? "warning" : "neutral"}
                      />
                      <RunStat
                        label="失败"
                        value={runOverview.failed}
                        dot="error.400"
                        tone={runOverview.failed > 0 ? "error" : "neutral"}
                      />
                    </HStack>
                    <Text
                      fontSize="xs"
                      color="workbench.muted"
                      noOfLines={{ base: 2, md: 1 }}
                    >
                      {lastRunLabel} · {nextRunLabel}
                      {runOverview.running > 0
                        ? " · 执行中批次每 6 秒自动刷新"
                        : ""}
                    </Text>
                  </Stack>

                  <HStack
                    spacing={3}
                    order={{ base: 1, md: 2 }}
                    flexShrink={0}
                    w={{ base: "full", md: "auto" }}
                    justify={{ base: "space-between", md: "flex-end" }}
                  >
                    <Button
                      h={FIELD_H}
                      variant="outline"
                      borderColor="neutral.200"
                      leftIcon={<FiRefreshCw aria-hidden />}
                      whiteSpace="nowrap"
                      onClick={() => refreshRuns()}
                      _hover={{
                        borderColor: "primary.300",
                        color: "primary.600",
                      }}
                      _active={{ transform: "scale(0.98)" }}
                      _focusVisible={{
                        boxShadow: "0 0 0 2px var(--chakra-colors-primary-300)",
                        outline: "none",
                      }}
                    >
                      刷新
                    </Button>
                    <HintToggle
                      label="采集任务历史说明"
                      expanded={runHelpOpen}
                      onToggle={() => setRunHelpOpen((prev) => !prev)}
                    />
                  </HStack>
                </Flex>

                <Collapse in={runHelpOpen} animateOpacity>
                  <Divider mt={3} mb={3} borderColor="neutral.100" />
                  <UnorderedList
                    spacing={2}
                    pl={5}
                    m={0}
                    fontSize="sm"
                    color="workbench.text"
                  >
                    <ListItem>
                      每次采集执行都会留下一条记录：定时整轮、手动整轮、单源“立即采集”都在这里，按时间倒序排列。
                    </ListItem>
                    <ListItem>
                      状态含义：成功＝全部源正常；部分成功＝有源失败但其余源已入库；失败＝全部源失败，
                      或批次超时未完成被自动收尾。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        查看明细
                      </Text>
                      ：逐个源查看本次发现、入库、跳过与失败原因。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        重试
                      </Text>
                      ：按本次的范围（全部源或指定源）重新发起一轮；原记录保留，新记录会标注“重试自
                      …”。
                    </ListItem>
                    <ListItem>
                      <Text as="span" fontWeight="600">
                        删除
                      </Text>
                      ：仅删除这条执行历史与源明细，已入库公告不受影响，删除前会二次确认。
                    </ListItem>
                    <ListItem>
                      表格列可在“操作”列右侧的设置按钮里勾选，选择会记在本机；当前共{" "}
                      {runOverview.total} 条记录。
                    </ListItem>
                  </UnorderedList>
                </Collapse>
              </Box>

              {runsLoading ? (
                <Skeleton height="160px" borderRadius="xl" />
              ) : runs.length === 0 ? (
                <EmptyState
                  title="还没有采集记录"
                  description="可以在“采集源管理”点“手动采集一轮”，或到“采集任务配置”里设置定时自动采集。"
                />
              ) : (
                <TableContainer
                  bg="white"
                  borderRadius="xl"
                  borderWidth="1px"
                  borderColor="neutral.100"
                  overflowX="auto"
                >
                  <Table size="sm" minW={`${runTableMinWidth}px`}>
                    <Thead>
                      <Tr>
                        <Th minW="200px">批次</Th>
                        {showRunColumn("采集范围") && (
                          <Th minW="150px">采集范围</Th>
                        )}
                        {showRunColumn("状态") && <Th minW="96px">状态</Th>}
                        {showRunColumn("触发方式") && (
                          <Th minW="88px">触发方式</Th>
                        )}
                        {showRunColumn("源（成功/总数）") && (
                          <Th isNumeric minW="110px">
                            源（成功/总数）
                          </Th>
                        )}
                        {showRunColumn("发现") && (
                          <Th isNumeric minW="72px">
                            发现
                          </Th>
                        )}
                        {showRunColumn("新增") && (
                          <Th isNumeric minW="72px">
                            新增
                          </Th>
                        )}
                        {showRunColumn("重复") && (
                          <Th isNumeric minW="72px">
                            重复
                          </Th>
                        )}
                        {showRunColumn("打标") && (
                          <Th isNumeric minW="72px">
                            打标
                          </Th>
                        )}
                        {showRunColumn("匹配提醒") && (
                          <Th isNumeric minW="88px">
                            匹配提醒
                          </Th>
                        )}
                        {showRunColumn("错误摘要") && (
                          <Th minW="180px">错误摘要</Th>
                        )}
                        <Th minW="160px" pr={2}>
                          <Flex align="center" justify="space-between" gap={2}>
                            <Text as="span">操作</Text>
                            <ColumnConfig
                              allHeaders={RUN_COLUMN_OPTIONS}
                              visibleLabels={visibleRunColumns}
                              onChange={setRunColumns}
                              mandatoryLabels={RUN_MANDATORY_COLUMNS}
                              triggerProps={{
                                h: "44px",
                                w: "44px",
                                minW: "44px",
                                size: "sm",
                                borderRadius: "md",
                                color: "neutral.500",
                                _hover: {
                                  bg: "primary.50",
                                  color: "primary.700",
                                },
                                _focusVisible: {
                                  boxShadow:
                                    "0 0 0 2px var(--chakra-colors-primary-300)",
                                  outline: "none",
                                },
                              }}
                              onReset={() => setRunColumns(RUN_DEFAULT_COLUMNS)}
                            />
                          </Flex>
                        </Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {runs.map((run) => (
                        <Tr key={run.run_id}>
                          <Td>
                            <Text
                              fontSize="sm"
                              sx={{ fontVariantNumeric: "tabular-nums" }}
                            >
                              {run.run_id}
                            </Text>
                            <Text fontSize="xs" color="workbench.muted">
                              {run.started_at}
                              {run.finished_at ? ` → ${run.finished_at}` : ""}
                            </Text>
                            {run.retry_of && (
                              <Tooltip label={`重试自 ${run.retry_of}`}>
                                <Badge
                                  mt={1}
                                  variant="subtle"
                                  colorScheme="info"
                                  borderRadius="sm"
                                >
                                  重试
                                </Badge>
                              </Tooltip>
                            )}
                          </Td>
                          {showRunColumn("采集范围") && (
                            <Td>
                              <Text
                                fontSize="sm"
                                color="workbench.text"
                                overflowWrap="anywhere"
                                noOfLines={2}
                              >
                                {run.scope_label || "全部启用源"}
                              </Text>
                            </Td>
                          )}
                          {showRunColumn("状态") && (
                            <Td>
                              <Badge
                                colorScheme={
                                  RUN_STATUS_COLOR[run.status] || "neutral"
                                }
                                variant="subtle"
                                whiteSpace="nowrap"
                              >
                                {RUN_STATUS_LABEL[run.status] || run.status}
                              </Badge>
                              {run.status === "running" && (
                                <Text
                                  mt={1}
                                  fontSize="xs"
                                  color="workbench.muted"
                                >
                                  {runningRunHint(run)}
                                </Text>
                              )}
                            </Td>
                          )}
                          {showRunColumn("触发方式") && (
                            <Td fontSize="sm" whiteSpace="nowrap">
                              {run.trigger_type === "manual" ? "手动" : "定时"}
                            </Td>
                          )}
                          {showRunColumn("源（成功/总数）") && (
                            <Td isNumeric>
                              {run.source_success}/{run.source_total}
                            </Td>
                          )}
                          {showRunColumn("发现") && (
                            <Td isNumeric>{run.discovered_count}</Td>
                          )}
                          {showRunColumn("新增") && (
                            <Td isNumeric>{run.inserted_count}</Td>
                          )}
                          {showRunColumn("重复") && (
                            <Td isNumeric>{run.duplicate_count}</Td>
                          )}
                          {showRunColumn("打标") && (
                            <Td isNumeric>{run.enriched_count}</Td>
                          )}
                          {showRunColumn("匹配提醒") && (
                            <Td isNumeric>{run.matched_count ?? 0}</Td>
                          )}
                          {showRunColumn("错误摘要") && (
                            <Td
                              fontSize="xs"
                              color="workbench.muted"
                              whiteSpace="normal"
                              maxW="280px"
                            >
                              {run.error_summary || "—"}
                            </Td>
                          )}
                          <Td pr={2}>
                            {/* 操作列不换行：主操作文字按钮 + 两个图标按钮（带 Tooltip） */}
                            <HStack
                              spacing={0}
                              flexWrap="nowrap"
                              whiteSpace="nowrap"
                            >
                              <Button
                                h="36px"
                                size="sm"
                                variant="ghost"
                                color="primary.600"
                                whiteSpace="nowrap"
                                onClick={() => {
                                  setActiveRun(run);
                                  runDetail.onOpen();
                                }}
                                _hover={{ bg: "primary.50" }}
                                _focusVisible={{
                                  boxShadow:
                                    "0 0 0 2px var(--chakra-colors-primary-300)",
                                  outline: "none",
                                }}
                              >
                                查看明细
                              </Button>
                              <Tooltip
                                label="按本次范围重新发起一轮采集"
                                openDelay={500}
                              >
                                <IconButton
                                  aria-label={`重试批次 ${run.run_id}`}
                                  icon={<FiRefreshCw aria-hidden />}
                                  h="36px"
                                  w="36px"
                                  minW="36px"
                                  variant="ghost"
                                  color="primary.600"
                                  isLoading={
                                    runRetryLoading &&
                                    retryBusyId === run.run_id
                                  }
                                  onClick={() => handleRetryRun(run)}
                                  _hover={{ bg: "primary.50" }}
                                  _focusVisible={{
                                    boxShadow:
                                      "0 0 0 2px var(--chakra-colors-primary-300)",
                                    outline: "none",
                                  }}
                                />
                              </Tooltip>
                              <Tooltip
                                label="删除这条执行历史（公告不受影响）"
                                openDelay={500}
                              >
                                <IconButton
                                  aria-label={`删除批次 ${run.run_id}`}
                                  icon={<FiTrash2 aria-hidden />}
                                  h="36px"
                                  w="36px"
                                  minW="36px"
                                  variant="ghost"
                                  color="neutral.500"
                                  onClick={() => {
                                    setRunDeleteTarget(run);
                                    runDeleteDialog.onOpen();
                                  }}
                                  _hover={{ bg: "red.50", color: "red.600" }}
                                  _focusVisible={{
                                    boxShadow:
                                      "0 0 0 2px var(--chakra-colors-error-200)",
                                    outline: "none",
                                  }}
                                />
                              </Tooltip>
                            </HStack>
                          </Td>
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>
                </TableContainer>
              )}
            </TabPanel>
          </TabPanels>
        </Tabs>

        <SourceEditorModal
          isOpen={editor.isOpen}
          onClose={editor.onClose}
          source={editingSource}
          submitting={sourceSaveLoading}
          onSubmit={handleSaveSource}
        />
        <SourceImportModal
          isOpen={importer.isOpen}
          onClose={importer.onClose}
          onImported={refreshSources}
        />
        <RunDetailModal
          isOpen={runDetail.isOpen}
          onClose={runDetail.onClose}
          run={activeRun}
        />

        <DeleteConfirmModal
          isOpen={runDeleteDialog.isOpen}
          onClose={() => {
            runDeleteDialog.onClose();
            setRunDeleteTarget(null);
          }}
          title="删除采集批次记录"
          description={
            <>
              确定删除批次
              <Text as="span" fontWeight="700" color="workbench.text">
                {runDeleteTarget?.run_id || ""}
              </Text>
              的执行记录吗？只会删除这次执行的历史与源明细，已入库的公告不受影响；操作不可恢复。
            </>
          }
          isLoading={runDeleteLoading}
          handleConfirm={() => {
            if (runDeleteTarget) handleDeleteRun(runDeleteTarget);
          }}
        />

        <DeleteConfirmModal
          isOpen={deleteDialog.isOpen}
          onClose={() => {
            deleteDialog.onClose();
            setDeleteTarget(null);
          }}
          title="删除采集源"
          description={
            <>
              确定删除采集源
              <Text as="span" fontWeight="700" color="workbench.text">
                {deleteTarget?.name || deleteTarget?.source_key || ""}
              </Text>
              吗？删除后该源不再参与自动采集，其历史公告与批次记录仍会保留；如需恢复内置源，可重新执行
              docs/sql/tender-intel.sql 或再次导入表格。
            </>
          }
          isLoading={sourceDeleteLoading}
          handleConfirm={() => {
            if (deleteTarget) handleDeleteSource(deleteTarget);
          }}
        />
      </PageContent>
    </PageViewport>
  );
}
