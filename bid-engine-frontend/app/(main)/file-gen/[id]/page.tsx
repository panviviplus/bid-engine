"use client";

/* eslint-disable no-use-before-define */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion, useDragControls } from "framer-motion";
import {
  Box,
  Button,
  Flex,
  Progress,
  Text,
  useToast,
  useMediaQuery,
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerHeader,
  DrawerOverlay,
  Grid,
} from "@chakra-ui/react";
import { FiCheckCircle, FiAlertTriangle } from "react-icons/fi";
import { LuPanelLeftOpen } from "react-icons/lu";
import { LuFileText } from "react-icons/lu";
import { useSidebarCollapse } from "@/components/layout";
import { BackButton } from "@/components/common/header-controls";
import { useParams, useRouter } from "next/navigation";

import BidEditor, { BidEditorHandle } from "@/components/file-gen/bid-editor";
import OutlineNav from "@/components/file-gen/outline-nav";
import OutlineSidebar from "@/components/file-gen/outline-sidebar";
import OutlineReviewGate from "@/components/file-gen/outline-review-gate";
import FloatingPanel from "@/components/file-gen/floating-panel";
import ChapterAiModal from "@/components/file-gen/chapter-ai-modal";
import {
  useBidGenDetail,
  useBidGenSaveDoc,
  useBidGenCancel,
  useBidGenRecordExport,
  useBidGenOutlineAdd,
  useBidGenOutlineUpdate,
  useBidGenOutlineDelete,
  useBidGenOutlineApply,
  useBidGenOutlineSync,
  useBidGenOutlineComplete,
  useBidGenUnconfirmOutline,
  useBidGenExportPdf,
  useBidGenExportPageMap,
  startGenerate,
  subscribeGenerate,
  streamRewrite,
  type GenerationPhase,
  type GenerationTask,
  type GenMode,
} from "@/service/bid-gen";
import { useReviewCreateFromGen } from "@/service/audit";
import { reorderOutline } from "@/components/file-gen/outline-tree-utils";
import {
  isDocumentRootOutline,
  type BidGenOutlineNode,
} from "@/components/file-gen/types";
import {
  textToPMJSON,
  sanitizePMDocWithReport,
} from "@/components/file-gen/pm-json";
import BidDocPreview from "@/components/file-gen/bid-doc-preview";
import {
  buildBidDocxBlob,
  buildTocEntries,
  orderOutlineTree,
  type BidDocxCover,
  type BidDocxTocEntry,
} from "@/components/file-gen/docx-export";

export default function FileGenDetailPage() {
  const params = useParams();
  const id = Number(params?.id || 0);
  const router = useRouter();
  const toast = useToast();

  const { detailData, detailLoading, fetchDetail } = useBidGenDetail(id);
  const { fetchSave } = useBidGenSaveDoc();
  const { fetchCancel } = useBidGenCancel();
  const { fetchRecord } = useBidGenRecordExport();
  const { fetchAdd } = useBidGenOutlineAdd();
  const { fetchUpdate } = useBidGenOutlineUpdate();
  const { fetchDelete } = useBidGenOutlineDelete();
  const { fetchApply } = useBidGenOutlineApply();
  const { fetchSync: fetchOutlineSync } = useBidGenOutlineSync();
  const { fetchComplete } = useBidGenOutlineComplete();
  const { fetchUnconfirm } = useBidGenUnconfirmOutline();
  const { fetchExport: fetchExportPdf } = useBidGenExportPdf();
  const { fetchPageMap } = useBidGenExportPageMap();

  const editorRef = useRef<BidEditorHandle>(null);
  const streamCloseRef = useRef<(() => void) | null>(null);
  const partialTextRef = useRef<Map<number, string>>(new Map());
  const pendingChapterNodesRef = useRef<Map<number, any[]>>(new Map());
  const editorReadyRef = useRef(false);
  const editorRehydratingRef = useRef(false);
  const terminalTaskRef = useRef(0);
  const activeTaskIdRef = useRef(0);
  const activeTaskTypeRef = useRef("full");
  const unconfirmCancelRef = useRef<any>(null);

  const [dirty, setDirty] = useState(false);
  // 生成结束后递增，用于强制编辑器重新加载后端权威文档（修复本地被破坏的编辑状态）
  const [docEpoch, setDocEpoch] = useState(0);
  const [generationPhase, setGenerationPhase] =
    useState<GenerationPhase>("idle");
  const [activeTaskId, setActiveTaskId] = useState(0);
  const [previewDegraded, setPreviewDegraded] = useState(false);
  const [streamInfo, setStreamInfo] = useState<{
    outlineId: number;
    title: string;
  } | null>(null);
  const [progress, setProgress] = useState({ current: 0, total: 0, pct: 0 });
  const [length, setLength] = useState("standard");
  // 导出中标记按格式区分：DOCX / PDF 各自渲染 loading，避免点 DOCX 时 PDF 按钮转圈
  const [exportingType, setExportingType] = useState<"docx" | "pdf" | null>(
    null,
  );
  const exporting = exportingType !== null;
  // 两种导出都基于同一份编辑器文档，串行执行，避免并发构建/下载互相干扰
  const exportInFlightRef = useRef(false);
  // 重新编排结构（撤销大纲确认）：二次确认弹窗
  const [unconfirmOpen, setUnconfirmOpen] = useState(false);
  const [unconfirming, setUnconfirming] = useState(false);
  const [materialOpen, setMaterialOpen] = useState(false);
  const [navCollapsed, setNavCollapsed] = useState(false);
  const [panelMode, setPanelMode] = useState<
    "pinned" | "floating" | "collapsed"
  >("pinned");
  const [isHovering, setIsHovering] = useState(false);
  const [activeOutlineId, setActiveOutlineId] = useState<number>(0);
  const [panelCompact, setPanelCompact] = useState(false);
  // 窄屏（<lg 992px）：大纲自动折叠 + 面板自动缩小；手动调整用 ref 记录，跨回宽屏时尊重用户选择
  const [isBelowLg] = useMediaQuery("(max-width: 61.98em)");
  const wasBelowLgRef = useRef(isBelowLg);
  const manualNavRef = useRef(false);
  const manualCompactRef = useRef(false);
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const dragControls = useDragControls();
  const editorAreaRef = useRef<HTMLDivElement>(null);
  const pageAreaRef = useRef<HTMLDivElement>(null);
  const { collapseForDetail, restoreSidebar } = useSidebarCollapse();

  const project = detailData;
  const generating =
    generationPhase !== "idle" || project?.status === "generating";

  // 进入详情页时自动折叠左侧栏，为编辑工作区留出空间；离开时恢复进入前状态。
  useEffect(() => {
    collapseForDetail();
    return restoreSidebar;
  }, [collapseForDetail, restoreSidebar]);

  // 视口跨过 lg 时：宽→窄 自动折叠大纲并缩小面板；窄→宽 恢复默认（除非窄屏期间手动调整过）
  useEffect(() => {
    const wasBelow = wasBelowLgRef.current;
    if (isBelowLg === wasBelow) return;
    if (isBelowLg) {
      manualNavRef.current = false;
      manualCompactRef.current = false;
      setNavCollapsed(true);
      setPanelCompact(true);
    } else {
      if (!manualNavRef.current) setNavCollapsed(false);
      if (!manualCompactRef.current) setPanelCompact(false);
      manualNavRef.current = false;
      manualCompactRef.current = false;
    }
    wasBelowLgRef.current = isBelowLg;
  }, [isBelowLg]);

  const outlineNodes: BidGenOutlineNode[] = useMemo(
    () => project?.outline || [],
    [project?.outline],
  );

  // 本地大纲状态（生成中实时更新 gen_status）
  const [localOutline, setLocalOutline] = useState<BidGenOutlineNode[]>([]);
  useEffect(() => {
    setLocalOutline(outlineNodes);
  }, [outlineNodes]);
  const contentOutline = useMemo(
    () => localOutline.filter((node) => !isDocumentRootOutline(node)),
    [localOutline],
  );

  // 章节级 AI 弹层目标（重写/扩写/缩写；生成本章/重新生成直接触发不进弹层）
  const [chapterAiTarget, setChapterAiTarget] = useState<{
    outlineId: number;
    title: string;
  } | null>(null);

  // 功能面板当前章节 = 最近在大纲中选中的章节
  const activeChapter = useMemo(() => {
    if (!activeOutlineId) return null;
    return localOutline.find((n) => n.id === activeOutlineId) || null;
  }, [activeOutlineId, localOutline]);

  // 当前章节是否含子章节（章节级写作会连同子章节一起写）
  const activeChapterHasChildren = useMemo(
    () => localOutline.some((n) => n.parentId === activeOutlineId),
    [activeOutlineId, localOutline],
  );

  // 章节级生成开始前确保面板可见（毛玻璃遮罩 + 停止生成依赖展开面板）
  const ensurePanelVisible = useCallback(() => {
    setPanelCompact(false);
    setPanelMode((m) => (m === "collapsed" ? "floating" : m));
  }, []);

  const markOutlineStatus = useCallback((outlineId: number, status: string) => {
    setLocalOutline((prev) =>
      prev.map((n) =>
        n.id === outlineId ? { ...n, genStatus: status as any } : n,
      ),
    );
  }, []);

  // ===== 自动保存（debounce 2.5s，生成期间挂起）=====
  const saveNow = useCallback(async () => {
    if (!editorRef.current) return false;
    const json = editorRef.current.getJSON();
    const html = editorRef.current.getHTML();
    if (!json) return false;
    try {
      const res = await fetchSave({
        data: { projectId: id, docJson: JSON.stringify(json), docHtml: html },
      });
      if (res?.data?.code === 200) {
        setDirty(false);
        return true;
      }
      return false;
    } catch {
      return false;
    }
  }, [fetchSave, id]);

  useEffect(() => {
    if (!dirty || generating) return;
    const t = window.setTimeout(() => {
      saveNow();
    }, 2500);
    return () => window.clearTimeout(t);
    // docEpoch 变更（生成结束重载权威文档）会取消在途防抖，避免把流式中/重载前状态写回
  }, [dirty, generating, saveNow, docEpoch]);

  // ===== 可恢复后台生成 =====
  const finishGenerationSession = useCallback(
    async (kind: "done" | "error" | "cancelled", message?: string) => {
      streamCloseRef.current?.();
      streamCloseRef.current = null;
      terminalTaskRef.current = activeTaskIdRef.current;
      partialTextRef.current.clear();
      pendingChapterNodesRef.current.clear();
      setStreamInfo(null);
      setPreviewDegraded(false);
      try {
        await fetchDetail();
      } catch {
        // 终态以服务端为准；下次详情轮询仍会恢复。
      }
      setDocEpoch((value) => value + 1);
      setDirty(false);
      setGenerationPhase("idle");
      setActiveTaskId(0);
      activeTaskIdRef.current = 0;
      if (kind === "done") {
        toast({
          title:
            activeTaskTypeRef.current === "chapter"
              ? "章节生成完成"
              : "全部章节生成完成",
          status: "success",
          duration: 3000,
        });
      } else if (kind === "cancelled") {
        toast({ title: "已停止生成", status: "info", duration: 3000 });
      } else {
        toast({
          title: message || "生成失败",
          status: "error",
          duration: 4000,
        });
      }
    },
    [fetchDetail, toast],
  );

  const applyPartialText = useCallback((outlineId: number, text: string) => {
    if (
      !editorReadyRef.current ||
      editorRehydratingRef.current ||
      !editorRef.current
    ) {
      return;
    }
    try {
      const applied = editorRef.current.replaceChapter(
        outlineId,
        textToPMJSON(text),
      );
      setPreviewDegraded(applied !== true);
    } catch {
      setPreviewDegraded(true);
    }
  }, []);

  const connectGenerationTask = useCallback(
    (task: GenerationTask) => {
      if (!task?.id) return;
      terminalTaskRef.current = 0;
      activeTaskIdRef.current = task.id;
      activeTaskTypeRef.current = task.taskType || "full";
      streamCloseRef.current?.();
      setActiveTaskId(task.id);
      setGenerationPhase(
        task.status === "pending"
          ? "queued"
          : task.status === "cancelling"
            ? "stopping"
            : "running",
      );
      setProgress({
        current: Number(task.completedCount || 0),
        total: Number(task.totalCount || 0),
        pct: Number(task.progress || 0),
      });
      ensurePanelVisible();
      setPanelMode("pinned");
      setPanelCompact(false);

      streamCloseRef.current = subscribeGenerate(id, task.id, {
        onConnection: (state) => {
          setGenerationPhase((current) => {
            if (current === "stopping" || current === "queued") return current;
            return state === "open" ? "running" : "reconnecting";
          });
        },
        onTaskState: (data) => {
          const status = String(data?.status || "");
          if (status === "pending") setGenerationPhase("queued");
          else if (status === "cancelling") setGenerationPhase("stopping");
          else if (status === "running") setGenerationPhase("running");
          const current = Number(data?.current ?? data?.completedCount ?? 0);
          const total = Number(data?.total ?? data?.totalCount ?? 0);
          const pct = Number(data?.progress ?? 0);
          setProgress({ current, total, pct });
        },
        onSnapshot: (data) => {
          const outlineId = Number(data?.currentOutlineId || 0);
          const partialText = String(data?.partialText || "");
          if (outlineId > 0) {
            setGenerationPhase((current) =>
              current === "stopping" ? current : "running",
            );
            setStreamInfo({
              outlineId,
              title:
                String(data?.title || "") ||
                localOutline.find((node) => node.id === outlineId)?.title ||
                "正在生成",
            });
          }
          if (outlineId > 0 && partialText) {
            partialTextRef.current.set(outlineId, partialText);
          }
          editorRehydratingRef.current = true;
          editorReadyRef.current = false;
          void (async () => {
            const response = await fetchDetail();
            if (response) {
              setDocEpoch((value) => value + 1);
              return;
            }
            // 详情拉取未成功（含并发复用失败）：继续使用当前编辑器内容，
            // 后续详情轮询仍会对账。
            editorRehydratingRef.current = false;
            editorReadyRef.current = true;
            if (outlineId > 0 && partialText) {
              applyPartialText(outlineId, partialText);
            }
          })();
        },
        onChapterStart: (data) => {
          partialTextRef.current.set(data.outlineId, "");
          setStreamInfo({ outlineId: data.outlineId, title: data.title });
          markOutlineStatus(data.outlineId, "generating");
          setGenerationPhase("running");
        },
        onDelta: (data) => {
          const accumulated =
            (partialTextRef.current.get(data.outlineId) || "") +
            String(data.text || "");
          partialTextRef.current.set(data.outlineId, accumulated);
          applyPartialText(data.outlineId, accumulated);
        },
        onChapterDone: (data) => {
          partialTextRef.current.delete(data.outlineId);
          const content =
            Array.isArray(data.json) && data.json.length > 1
              ? data.json.slice(1)
              : [];
          pendingChapterNodesRef.current.set(data.outlineId, content);
          if (
            editorReadyRef.current &&
            !editorRehydratingRef.current &&
            editorRef.current
          ) {
            try {
              const applied = editorRef.current.replaceChapter(
                data.outlineId,
                content,
              );
              if (applied !== true) setPreviewDegraded(true);
              else {
                setPreviewDegraded(false);
                pendingChapterNodesRef.current.delete(data.outlineId);
              }
            } catch {
              setPreviewDegraded(true);
            }
          }
          markOutlineStatus(data.outlineId, "succeeded");
        },
        onChapterError: (data) => {
          partialTextRef.current.delete(data.outlineId);
          markOutlineStatus(data.outlineId, "failed");
          toast({
            title: data.msg || "章节生成失败",
            status: "error",
            duration: 4000,
          });
        },
        onProgress: (value) => setProgress(value),
        onError: (data) => finishGenerationSession("error", data.msg),
        onDone: () => finishGenerationSession("done"),
        onCancelled: () => finishGenerationSession("cancelled"),
      });
    },
    [
      applyPartialText,
      ensurePanelVisible,
      fetchDetail,
      finishGenerationSession,
      id,
      localOutline,
      markOutlineStatus,
      toast,
    ],
  );

  const handleGenerate = useCallback(
    async (
      outlineIds?: number[],
      mode?: GenMode,
      instruction?: string,
      lengthOverride?: string,
    ) => {
      if (!project || generating) return;
      const saved = await saveNow();
      if (!saved) {
        toast({
          title: "当前文档保存失败，未启动生成",
          status: "error",
          duration: 3500,
        });
        return;
      }
      setGenerationPhase("queued");
      setProgress({ current: 0, total: 0, pct: 0 });
      const genLength = lengthOverride || length;
      try {
        const task = await startGenerate(
          outlineIds
            ? "/api/zb/file-gen/generate/chapter"
            : "/api/zb/file-gen/generate/full",
          { projectId: id, outlineIds, length: genLength, mode, instruction },
        );
        connectGenerationTask(task);
        void fetchDetail();
      } catch (error: any) {
        setGenerationPhase("reconnecting");
        try {
          const detailResponse = await fetchDetail();
          const recoveredTask = detailResponse?.data?.data
            ?.runningTask as GenerationTask | undefined;
          if (recoveredTask?.id) {
            connectGenerationTask(recoveredTask);
            return;
          }
        } catch {
          // 继续使用原始错误提示。
        }
        setGenerationPhase("idle");
        toast({
          title: error?.message || "创建生成任务失败",
          status: "error",
          duration: 4000,
        });
      }
    },
    [
      project,
      generating,
      saveNow,
      toast,
      length,
      id,
      connectGenerationTask,
      fetchDetail,
    ],
  );

  // 大纲导航章节菜单回调：write 直接生成；rewrite/expand/condense 打开弹层
  const handleChapterAiAction = useCallback(
    (outlineId: number, mode: GenMode) => {
      const node = localOutline.find((n) => n.id === outlineId);
      if (!node) return;
      if (isDocumentRootOutline(node)) {
        toast({ title: "目录根节点无需生成正文", status: "info", duration: 2500 });
        return;
      }
      if (mode === "write") {
        ensurePanelVisible();
        handleGenerate([outlineId], "write");
      } else {
        setChapterAiTarget({ outlineId, title: node.title });
      }
    },
    [localOutline, ensurePanelVisible, handleGenerate, toast],
  );

  // 功能面板章节 AI 按钮回调（针对当前选中章节）
  const handlePanelChapterAction = useCallback(
    (mode: GenMode) => {
      if (!activeOutlineId) return;
      handleChapterAiAction(activeOutlineId, mode);
    },
    [activeOutlineId, handleChapterAiAction],
  );

  // 章节 AI 弹层确认：按所选档位生成（显式传 lengthOverride，避免 state 异步滞后）
  const handleChapterAiConfirm = useCallback(
    (mode: GenMode, tier: string, instruction: string) => {
      const target = chapterAiTarget;
      if (!target) return;
      setChapterAiTarget(null);
      setLength(tier);
      ensurePanelVisible();
      handleGenerate([target.outlineId], mode, instruction || undefined, tier);
    },
    [chapterAiTarget, ensurePanelVisible, handleGenerate],
  );

  const handleCancel = useCallback(async () => {
    if (!activeTaskId || generationPhase === "stopping") return;
    setGenerationPhase("stopping");
    try {
      const response = await fetchCancel({
        data: { projectId: id, taskId: activeTaskId },
      });
      if (response?.data?.code !== 200) {
        throw new Error(response?.data?.msg || "停止生成失败");
      }
    } catch (error: any) {
      setGenerationPhase("reconnecting");
      toast({
        title:
          error?.response?.data?.msg ||
          error?.message ||
          "停止生成失败，请重试",
        status: "error",
        duration: 3500,
      });
    }
  }, [activeTaskId, fetchCancel, generationPhase, id, toast]);

  // 页面刷新或重新进入详情时，根据数据库活动任务自动恢复订阅。
  useEffect(() => {
    const runningTask = project?.runningTask as GenerationTask | undefined;
    if (
      project?.status === "generating" &&
      runningTask?.id &&
      runningTask.id !== terminalTaskRef.current &&
      runningTask.id !== activeTaskId
    ) {
      connectGenerationTask(runningTask);
      return;
    }
    if (
      activeTaskId > 0 &&
      project?.status !== "generating" &&
      !runningTask
    ) {
      streamCloseRef.current?.();
      streamCloseRef.current = null;
      partialTextRef.current.clear();
      pendingChapterNodesRef.current.clear();
      setGenerationPhase("idle");
      setActiveTaskId(0);
      activeTaskIdRef.current = 0;
      setStreamInfo(null);
      setDocEpoch((value) => value + 1);
      terminalTaskRef.current = 0;
    }
  }, [activeTaskId, connectGenerationTask, project]);

  useEffect(() => {
    if (!generating) return;
    const timer = window.setInterval(() => void fetchDetail(), 3000);
    return () => window.clearInterval(timer);
  }, [fetchDetail, generating]);

  useEffect(
    () => () => {
      streamCloseRef.current?.();
      streamCloseRef.current = null;
    },
    [],
  );

  // ===== 重新编排结构（撤销大纲确认：draft → outline_review，仅未生成章节时）=====
  const canUnconfirm = useMemo(() => {
    if (!project) return false;
    if (project.status !== "draft") return false;
    if (project.createType === "blank") return false;
    if (localOutline.length === 0) return false;
    return localOutline.every(
      (n) => n.genStatus !== "succeeded" && n.genStatus !== "generating",
    );
  }, [project, localOutline]);

  const handleUnconfirm = useCallback(async () => {
    setUnconfirming(true);
    try {
      const res = await fetchUnconfirm({ data: { projectId: id } });
      if (res?.data?.code === 200) {
        toast({
          title: "已回到大纲蓝图，可重新编排结构",
          status: "success",
          duration: 2500,
        });
        setUnconfirmOpen(false);
        void fetchDetail();
      } else {
        toast({
          title: res?.data?.msg || "撤销确认失败",
          status: "error",
          duration: 3000,
        });
      }
    } catch {
      toast({ title: "撤销确认失败", status: "error", duration: 3000 });
    } finally {
      setUnconfirming(false);
    }
  }, [fetchUnconfirm, id, toast, fetchDetail]);

  // ===== 编辑器 → 导航：大纲对账（串行执行，防止并发对账产生重复节点）=====
  const syncChainRef = useRef<Promise<void>>(Promise.resolve());
  const syncOutlineFromEditor = useCallback(
    (
      headings: { outlineId: number | null; level: number; title: string }[],
    ) => {
      syncChainRef.current = syncChainRef.current.then(async () => {
        try {
          // 执行时重新读取最新标题结构，避免在途编辑导致重复创建
          const fresh = editorRef.current?.getHeadingSnapshot?.() ?? headings;
          const res = await fetchOutlineSync({
            data: { projectId: id, headings: fresh },
          });
          if (res?.data?.code === 200) {
            const mapping = res?.data?.data?.mapping || {};
            const outline = res?.data?.data?.outline;
            editorRef.current?.applyOutlineSync(mapping, fresh);
            if (Array.isArray(outline)) setLocalOutline(outline);
          } else {
            editorRef.current?.resetOutlineBaseline();
          }
        } catch {
          // 静默失败：下次结构变化会再次同步
          editorRef.current?.resetOutlineBaseline();
        }
      });
    },
    [fetchOutlineSync, id],
  );

  // ===== 大纲操作（本地优先更新导航与编辑器，不整页刷新）=====
  const addOutline = useCallback(
    async (parentId: number, title: string, level: number) => {
      try {
        const res = await fetchAdd({
          data: { projectId: id, parentId, level, title },
        });
        if (res?.data?.code === 200) {
          toast({ title: "章节已添加", status: "success", duration: 2000 });
          const newId = res?.data?.data?.id;
          if (newId) {
            // 本地更新导航（新节点追加到同级末尾，sortOrder 与后端一致）
            setLocalOutline((prev) => {
              const max = prev
                .filter((n) => n.parentId === parentId)
                .reduce((m, n) => Math.max(m, n.sortOrder), 0);
              const node: BidGenOutlineNode = {
                id: newId,
                projectId: id,
                parentId,
                level,
                sortOrder: max + 100,
                title,
                clauseIds: "",
                materialIds: "",
                genStatus: "pending",
                source: "user",
                isRequiredFile: false,
                isAiSuggested: false,
              };
              return [...prev, node];
            });
            // 编辑器局部插入：顶层→文档末尾；子章节→父章节内容末尾
            if (parentId === 0) {
              editorRef.current?.insertHeading(newId, level, title);
            } else {
              editorRef.current?.insertHeading(newId, level, title, {
                afterOutlineId: parentId,
              });
            }
            editorRef.current?.resetOutlineBaseline();
          }
          void fetchDetail(); // 后台一致性兜底
        } else {
          toast({
            title: res?.data?.msg || "添加失败",
            status: "error",
            duration: 3000,
          });
        }
      } catch (e: any) {
        toast({
          title: e?.response?.data?.msg || "添加失败",
          status: "error",
          duration: 3000,
        });
      }
    },
    [fetchAdd, id, fetchDetail, toast],
  );

  // 人工标记章节写作完成/取消完成：人工撰写的章节不经过 AI 生成流程，
  // 置位后才会被计入项目整体完成度（全部章节完成 → 项目已完成）
  const completeOutline = useCallback(
    async (outlineId: number, completed: boolean) => {
      // 标记完成意味着这一章写完了：先落盘未保存的人工修改，
      // 避免出现状态已完成、正文还在草稿缓存里的窗口期。
      if (dirty) await saveNow();
      try {
        const res = await fetchComplete({
          data: { projectId: id, id: outlineId, completed },
        });
        if (res?.data?.code === 200) {
          // 父章节连带子章节：本地按同一口径整棵子树一起置位（后端返回权威 id 列表）
          const ids = Array.isArray(res?.data?.data?.outlineIds)
            ? (res.data.data.outlineIds as number[])
            : [outlineId];
          const nextStatus = completed ? "succeeded" : "pending";
          setLocalOutline((prev) =>
            prev.map((n) =>
              ids.includes(n.id)
                ? { ...n, genStatus: nextStatus as any }
                : n,
            ),
          );
          const scope =
            ids.length > 1 ? `（含 ${ids.length - 1} 个子章节）` : "";
          toast({
            title: `${completed ? "已标记为完成" : "已取消完成标记"}${scope}`,
            status: "success",
            duration: 2000,
          });
          void fetchDetail(); // 后台一致性兜底：同步整体状态/进度
        } else {
          toast({
            title: res?.data?.msg || "操作失败",
            status: "error",
            duration: 3000,
          });
        }
      } catch (e: any) {
        toast({
          title: e?.response?.data?.msg || "操作失败",
          status: "error",
          duration: 3000,
        });
      }
    },
    [dirty, fetchComplete, id, fetchDetail, saveNow, toast],
  );

  const renameOutline = useCallback(
    async (outlineId: number, title: string) => {
      try {
        const res = await fetchUpdate({ data: { id: outlineId, title } });
        if (res?.data?.code === 200) {
          setLocalOutline((prev) =>
            prev.map((n) => (n.id === outlineId ? { ...n, title } : n)),
          );
          editorRef.current?.renameHeading(outlineId, title);
          editorRef.current?.resetOutlineBaseline();
          setDirty(true);
        }
      } catch {
        // ignore
      }
    },
    [fetchUpdate],
  );

  const deleteOutline = useCallback(
    async (outlineId: number) => {
      try {
        const res = await fetchDelete({
          data: { id: outlineId, projectId: id },
        });
        if (res?.data?.code === 200) {
          toast({ title: "章节已删除", status: "success", duration: 2000 });
          // 本地移除节点及后代
          setLocalOutline((prev) => {
            const ids = new Set<number>([outlineId]);
            let changed = true;
            while (changed) {
              changed = false;
              for (const n of prev) {
                if (ids.has(n.parentId) && !ids.has(n.id)) {
                  ids.add(n.id);
                  changed = true;
                }
              }
            }
            return prev.filter((n) => !ids.has(n.id));
          });
          editorRef.current?.removeChapter(outlineId);
          editorRef.current?.resetOutlineBaseline();
          void fetchDetail();
        }
      } catch {
        // ignore
      }
    },
    [fetchDelete, id, fetchDetail, toast],
  );

  // ===== 大纲拖拽排序/跨级移动（树 → 文档 + 结构快照，串行持久化）=====
  const applyChainRef = useRef<Promise<void>>(Promise.resolve());
  const moveOutline = useCallback(
    (dragId: number, targetParentId: number, nextId: number) => {
      const res = reorderOutline(localOutline, dragId, targetParentId, nextId);
      if (!res) return;
      setLocalOutline(res.nodes);
      editorRef.current?.moveChapter(dragId, {
        parentId: targetParentId,
        nextId,
        newLevel: res.dragLevel,
      });
      editorRef.current?.resetOutlineBaseline();
      applyChainRef.current = applyChainRef.current
        .then(() =>
          fetchApply({
            data: {
              projectId: id,
              nodes: res.nodes.map((n) => ({
                id: n.id,
                parentId: n.parentId,
                level: n.level,
                sortOrder: n.sortOrder,
              })),
            },
          }),
        )
        .then((r: any) => {
          if (r?.data?.code === 200 && Array.isArray(r?.data?.data?.outline)) {
            setLocalOutline(r.data.data.outline);
          }
        })
        .catch(() => {
          // 静默：下次拖拽或保存时会再对齐
        });
    },
    [localOutline, id, fetchApply],
  );

  // ===== 导出 =====
  // 封面页字段由后端装配（招标事实 + 企业信息），前端零拼装。
  const coverData = useMemo<BidDocxCover>(() => {
    const raw = (detailData as any)?.cover || {};
    return {
      docTitle: raw.docTitle || "投 标 文 件",
      projectName: raw.projectName || project?.name || "",
      projectNumber: raw.projectNumber || "",
      lotLabel: raw.lotLabel || "",
      tendererName: raw.tendererName || "",
      bidderName: raw.bidderName || "",
      date: raw.date || "",
    };
  }, [detailData, project?.name]);

  // 目录条目只收录大纲节点（1-4 级），正文内小标题不进目录。
  const tocEntries = useMemo<BidDocxTocEntry[]>(
    () => buildTocEntries(orderOutlineTree(localOutline || [])),
    [localOutline],
  );
  const [tocWithPages, setTocWithPages] = useState<BidDocxTocEntry[]>([]);
  const [pageMapFailed, setPageMapFailed] = useState<string>("");

  /**
   * 两遍导出：第一遍生成说明书式的无页码目录，交给后端定位每章真实页码；
   * 第二遍用真实页码重建目录。页码定位失败时降级为目录完整、页码留空，
   * 不影响导出本身。
   */
  const buildExportDocx = useCallback(async (): Promise<{
    blob: Blob;
    toc: BidDocxTocEntry[];
  }> => {
    const editor = editorRef.current?.getEditor();
    const docNode = editor?.state?.doc;
    if (!docNode) throw new Error("编辑器未就绪");
    // 图片缩放存的是像素宽度，导出时按当前正文内容区宽度换算成 DOCX 列宽百分比
    const contentWidthPx = Number(editor?.view?.dom?.clientWidth || 0);

    const firstPass = await buildBidDocxBlob({
      docNode,
      cover: coverData,
      toc: tocEntries,
      contentWidthPx,
    });

    let pageMap: Map<number, number> | null = null;
    try {
      const formData = new FormData();
      formData.append("projectId", String(id));
      formData.append("file", firstPass, "page-map.docx");
      const res = await fetchPageMap({ data: formData });
      const pages = (res as any)?.data?.data?.pages;
      if (Array.isArray(pages) && pages.length > 0) {
        pageMap = new Map<number, number>(
          pages.map((item: any) => [Number(item.outlineId), Number(item.page)]),
        );
        setPageMapFailed("");
      }
    } catch (e: any) {
      // 页码定位失败不阻塞导出：目录仍输出完整条目，仅缺页码
      setPageMapFailed(e?.response?.data?.msg || e?.message || "页码定位失败");
    }

    const finalToc = tocEntries.map((entry) => ({
      ...entry,
      page: pageMap?.get(entry.outlineId),
    }));
    const blob = await buildBidDocxBlob({
      docNode,
      cover: coverData,
      toc: finalToc,
      contentWidthPx,
    });
    setTocWithPages(finalToc);
    return { blob, toc: finalToc };
  }, [coverData, fetchPageMap, id, tocEntries]);

  const [previewOpen, setPreviewOpen] = useState(false);
  const openPreview = useCallback(() => {
    // 保留上一次导出已定位到的页码，避免每次打开预览都清空
    const knownPages = new Map(
      tocWithPages
        .filter((entry) => typeof entry.page === "number")
        .map((entry) => [entry.outlineId, entry.page]),
    );
    setTocWithPages(
      tocEntries.map((entry) => ({
        ...entry,
        page: knownPages.get(entry.outlineId),
      })),
    );
    setPreviewOpen(true);
  }, [tocEntries, tocWithPages]);

  // 浏览器触发文件下载
  const downloadBlob = useCallback((blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(url);
  }, []);

  const exportDocx = useCallback(async () => {
    if (!editorRef.current || exportInFlightRef.current) return;
    exportInFlightRef.current = true;
    setExportingType("docx");
    try {
      const { blob } = await buildExportDocx();
      downloadBlob(blob, `${project?.name || "标书"}.docx`);
      await fetchRecord({
        data: {
          projectId: id,
          exportType: "docx",
          fileName: `${project?.name || "标书"}.docx`,
          fileSize: blob.size,
        },
      });
      toast({ title: "DOCX 已导出", status: "success", duration: 2500 });
    } catch (e: any) {
      toast({
        title: e?.message || "DOCX 导出失败",
        status: "error",
        duration: 4000,
      });
    } finally {
      exportInFlightRef.current = false;
      setExportingType(null);
    }
  }, [buildExportDocx, downloadBlob, fetchRecord, id, project?.name, toast]);

  const exportPdf = useCallback(async () => {
    if (!editorRef.current || exportInFlightRef.current) return;
    exportInFlightRef.current = true;
    setExportingType("pdf");
    try {
      const { blob } = await buildExportDocx();
      const fd = new FormData();
      fd.append("file", blob, "bid.docx");
      fd.append("projectId", String(id));
      // 后端返回 PDF 二进制流（Content-Disposition: attachment），须以 blob 接收后触发下载
      const res = await fetchExportPdf({ data: fd, responseType: "blob" });
      if (res?.status === 200 && res?.data instanceof Blob) {
        downloadBlob(res.data, `${project?.name || "标书"}-标书.pdf`);
        toast({ title: "PDF 已导出", status: "success", duration: 2500 });
      } else {
        toast({ title: "PDF 导出失败", status: "error", duration: 4000 });
      }
    } catch (e: any) {
      let msg = e?.message || "PDF 导出失败";
      // 后端 4xx/5xx 错误体是 JSON（responseType=blob 时为 Blob），解析出友好提示
      try {
        const data = e?.response?.data;
        if (data instanceof Blob) {
          const parsed = JSON.parse(await data.text());
          if (parsed?.msg) msg = parsed.msg;
        } else if (data?.msg) {
          msg = data.msg;
        }
      } catch {
        // 保留默认错误信息
      }
      toast({ title: msg, status: "error", duration: 4000 });
    } finally {
      exportInFlightRef.current = false;
      setExportingType(null);
    }
  }, [buildExportDocx, downloadBlob, fetchExportPdf, id, project?.name, toast]);

  // ===== 生成审核项目 =====
  const { createLoading: creatingReview, fetchCreate: fetchCreateReview } =
    useReviewCreateFromGen();

  // 审核入口仅在招标来源标书上出现：
  //   tender_file —— 从招标文件创建（上传招标文件，内部解析）
  //   analysis    —— 从招标解析项目流转而来
  // 模板标书 / 空白标书没有招标依据，不渲染入口。
  const isTenderSourced = useMemo(
    () =>
      project?.createType === "tender_file" ||
      project?.createType === "analysis",
    [project?.createType],
  );
  const canCreateReview = useMemo(
    () => isTenderSourced && project?.status === "succeeded",
    [isTenderSourced, project?.status],
  );

  const handleCreateReview = useCallback(async () => {
    if (!editorRef.current) {
      toast({ title: "编辑器未就绪，无法导出标书", status: "warning" });
      return;
    }
    try {
      const { blob } = await buildExportDocx();
      const fd = new FormData();
      fd.append("bid_gen_project_id", String(id));
      fd.append("bid_files", blob, `${project?.name || "标书"}-审核稿.docx`);
      const res = await fetchCreateReview({ data: fd });
      if (res?.data?.code === 200) {
        toast({
          title: "审核项目已创建，正在解析",
          status: "success",
          duration: 3000,
        });
        void fetchDetail();
      } else {
        toast({ title: res?.data?.msg || "创建审核项目失败", status: "error" });
      }
    } catch (e: any) {
      toast({
        title: e?.response?.data?.msg || e?.message || "创建审核项目失败",
        status: "error",
        duration: 4000,
      });
    }
  }, [buildExportDocx, fetchCreateReview, fetchDetail, id, project?.name, toast]);

  const handleViewReview = useCallback(
    (reviewId: number) => {
      router.push(`/bid-audit/${reviewId}`);
    },
    [router],
  );

  // 历史文档可能含非法节点（例如空表格单元格产生的空文本节点），
  // 直接交给编辑器会解析失败导致空白；这里先清理，并把修复结果标记为脏数据写回服务端。
  const docRepair = useMemo(() => {
    if (!project?.docJson) return { doc: null as any, repaired: false };
    try {
      return sanitizePMDocWithReport(JSON.parse(project.docJson));
    } catch {
      return { doc: null as any, repaired: false };
    }
  }, [project?.docJson]);
  const docJSONObj = docRepair.doc;

  useEffect(() => {
    if (!docRepair.repaired || generating) return;
    setDirty(true);
  }, [docRepair.repaired, generating]);

  if (detailLoading && !project) {
    return (
      <Flex align="center" justify="center" h="100%" bg="gray.50">
        <Progress
          size="xs"
          w="200px"
          colorScheme="gold"
          isIndeterminate
          borderRadius="full"
        />
      </Flex>
    );
  }
  if (!project) {
    return (
      <Flex
        direction="column"
        align="center"
        justify="center"
        gap={3}
        h="100%"
        bg="gray.50"
      >
        <FiAlertTriangle size="40" color="var(--chakra-colors-error-500)" />
        <Text color="gray.600">项目不存在或已被删除</Text>
        <Button variant="outline" onClick={() => router.push("/file-gen")}>
          返回列表
        </Button>
      </Flex>
    );
  }

  // ===== 大纲确认闸门 =====
  if (project.status === "outline_review") {
    return (
      <OutlineReviewGate
        project={project}
        onBack={() => router.push("/file-gen")}
        onConfirmed={() => void fetchDetail()}
      />
    );
  }

  return (
    <Flex
      ref={pageAreaRef}
      direction="column"
      h="100%"
      minH={0}
      gap={3}
      p={{ base: 2, md: 4 }}
      bg="workbench.canvas"
    >
      <Flex
        align="center"
        gap={2.5}
        minH={{ base: "68px", md: "76px" }}
        px={{ base: 3, md: 5 }}
        py={3}
        bg="workbench.control"
        color="workbench.paper"
        border="1px solid"
        borderColor="workbench.controlRaised"
        borderRadius="16px"
        boxShadow="lg"
        flexShrink={0}
      >
        <BackButton href="/file-gen" inverted />
        <Text
          minW={0}
          fontSize="lg"
          fontWeight="700"
          color="workbench.paper"
          noOfLines={1}
          title={project?.name}
        >
          {project?.name || "标书生成"}
        </Text>
        <Button
          ml="auto"
          flexShrink={0}
          size="sm"
          variant="outline"
          color="workbench.paper"
          borderColor="workbench.controlRaised"
          borderRadius="8px"
          leftIcon={<LuFileText size={15} />}
          transition="background-color var(--dur-micro) var(--ease-out)"
          _hover={{ bg: "workbench.controlRaised" }}
          _focusVisible={{
            outline: "2px solid",
            outlineColor: "gold.400",
            outlineOffset: "2px",
          }}
          _active={{ bg: "primary.900" }}
          onClick={openPreview}
        >
          预览文档
        </Button>
      </Flex>
      {/* 主体三栏：大纲 + 编辑器包进可横向滚动容器，窄屏展开大纲时左右滑动而非挤压 */}
      <Flex
        flex="1"
        minH={0}
        overflow="hidden"
        position="relative"
        border="1px solid"
        borderColor="workbench.line"
        borderRadius="14px"
        bg="workbench.paper"
      >
        <Flex
          flex="1"
          minW="0"
          overflowX="auto"
          overflowY="hidden"
          position="relative"
        >
          {/* 左侧大纲（宽度可拖拽调整，记忆于 localStorage） */}
          {!navCollapsed && (
            <OutlineSidebar
              defaultWidth={280}
              storageKey="bidgen-outline-width"
              borderRight="1px solid"
              borderColor="gray.200"
              bg="white"
            >
              {canUnconfirm && (
                <Flex
                  align="center"
                  justify="space-between"
                  px={3}
                  py={2}
                  borderBottom="1px solid"
                  borderColor="gray.100"
                >
                  <Flex align="center" gap={1.5}>
                    <FiAlertTriangle
                      size="12"
                      color="var(--chakra-colors-warning-500)"
                    />
                    <Text fontSize="xs" color="gray.500">
                      结构已锁定
                    </Text>
                  </Flex>
                  <Button
                    size="xs"
                    variant="ghost"
                    colorScheme="primary"
                    fontWeight="500"
                    onClick={() => setUnconfirmOpen(true)}
                  >
                    重新编排结构
                  </Button>
                </Flex>
              )}
              <OutlineNav
                nodes={localOutline}
                activeId={activeOutlineId}
                readOnly={generating}
                searchable
                onChapterAiAction={handleChapterAiAction}
                onMove={
                  project?.createType === "blank" ? moveOutline : undefined
                }
                onJump={(oid) => {
                  setActiveOutlineId(oid);
                  editorRef.current?.findHeadingAndScroll(oid);
                }}
                onAdd={addOutline}
                onRename={renameOutline}
                onDelete={deleteOutline}
                onToggleComplete={completeOutline}
                onCollapse={() => {
                  manualNavRef.current = true;
                  setNavCollapsed(true);
                }}
              />
            </OutlineSidebar>
          )}
          {/* 大纲展开入口 — 折叠后显示 */}
          {navCollapsed && (
            <Box
              position="relative"
              w="40px"
              flexShrink={0}
              display="flex"
              alignItems="stretch"
              borderRight="1px solid"
              borderColor="gray.100"
              bg="white"
              _hover={{ bg: "primary.50" }}
              transition="background-color 0.2s ease"
              cursor="pointer"
              onClick={() => {
                manualNavRef.current = true;
                setNavCollapsed(false);
              }}
              role="button"
              aria-label="展开大纲导航"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  setNavCollapsed(false);
                }
              }}
              _focusVisible={{
                boxShadow: "0 0 0 2px var(--chakra-colors-primary-500)",
                outline: "none",
              }}
              _active={{ transform: "scale(0.97)" }}
            >
              {/* 竖向感应条 */}
              <Box
                position="absolute"
                left={0}
                top={0}
                bottom={0}
                w="3px"
                bg="primary.500"
                opacity={0.08}
                _groupHover={{ opacity: 0.2 }}
                transition="opacity 0.2s ease"
                borderRadius="0 2px 2px 0"
              />
              {/* 居中图标 */}
              <Flex
                flex="1"
                align="center"
                justify="center"
                color="gray.400"
                _hover={{ color: "primary.600" }}
                transition="color 0.2s ease, transform 0.15s ease"
              >
                <Box
                  as={LuPanelLeftOpen}
                  size="18px"
                  _hover={{ transform: "scale(1.1)" }}
                  transition="transform 0.15s ease"
                />
              </Flex>
            </Box>
          )}

          {/* 中间编辑器 */}
          <Box
            ref={editorAreaRef}
            flex="1"
            minW={{ base: "100%", md: "560px" }}
            position="relative"
          >
            <BidEditor
              ref={editorRef}
              contentKey={`bidgen-${id}-${docEpoch}`}
              contentJSON={docJSONObj || undefined}
              contentHTML={project.docHtml || undefined}
              outline={localOutline}
              readOnly={generating}
              streaming={generating}
              streamTitle={streamInfo?.title}
              rewriteStream={(args, handlers, signal) =>
                streamRewrite({ projectId: id, ...args }, handlers, signal)
              }
              onReady={() => {
                // 模板导入：编辑器渲染 HTML 后回填 outlineId
                if (project.docHtml && !docJSONObj) {
                  editorRef.current?.syncOutlineIds(
                    localOutline.map((n) => ({
                      id: n.id,
                      level: n.level,
                      title: n.title,
                    })),
                  );
                }
                editorReadyRef.current = true;
                editorRehydratingRef.current = false;
                setPreviewDegraded(false);
                pendingChapterNodesRef.current.forEach(
                  (content, outlineId) => {
                  try {
                    if (editorRef.current?.replaceChapter(outlineId, content)) {
                      pendingChapterNodesRef.current.delete(outlineId);
                    } else {
                      setPreviewDegraded(true);
                    }
                  } catch {
                    setPreviewDegraded(true);
                  }
                  },
                );
                partialTextRef.current.forEach((text, outlineId) => {
                  applyPartialText(outlineId, text);
                });
              }}
              onDirty={(d) => setDirty(d)}
              onOutlineSync={syncOutlineFromEditor}
            />
          </Box>
        </Flex>

        {/* 右侧浮动面板（置于滚动容器外，横向滚动时保持悬浮可见） */}
        {/* 右侧边缘感应区 — collapsed 状态下可见 */}
        {panelMode === "collapsed" && (
          <Box
            position="absolute"
            top={0}
            right={0}
            w="40px"
            h="full"
            zIndex={9}
            onMouseEnter={() => {
              if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);
              setIsHovering(true);
            }}
            onMouseLeave={() => {
              hoverTimerRef.current = setTimeout(
                () => setIsHovering(false),
                200,
              );
            }}
          >
            {/* 视觉感应条 */}
            <Box
              position="absolute"
              top={0}
              right={0}
              w="4px"
              h="full"
              bg="primary.500"
              opacity={isHovering ? 0.18 : 0.06}
              transition="opacity 0.25s ease, width 0.25s ease, box-shadow 0.25s ease"
              _hover={{
                width: "8px",
                opacity: 0.25,
                boxShadow: "0 0 12px rgba(30,58,95,0.15)",
              }}
              borderRadius="2xl 0 0 2xl"
            />
          </Box>
        )}
        {/* 浮动面板本体 */}
        <AnimatePresence>
          {(panelMode === "pinned" ||
            panelMode === "floating" ||
            isHovering ||
            panelCompact ||
            generating) && (
            <motion.div
              key="floating-panel"
              drag
              dragControls={dragControls}
              dragListener={false}
              dragMomentum={false}
              initial={{ opacity: 0, x: 24 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, scale: 0.85, x: 24 }}
              transition={{
                duration: 0.3,
                ease: [0.16, 1, 0.3, 1],
                exit: { duration: 0.28, ease: [0.4, 0, 1, 1] },
              }}
              style={{
                position: "absolute",
                top: 16,
                right: 16,
                zIndex: 10,
                pointerEvents: "auto",
              }}
              onMouseEnter={() => {
                if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);
                setIsHovering(true);
              }}
              onMouseLeave={() => {
                // floating mode: collapse after 200ms delay (skip in compact mode)
                if (!generating && panelMode === "floating" && !panelCompact) {
                  hoverTimerRef.current = setTimeout(() => {
                    setPanelMode("collapsed");
                    setIsHovering(false);
                  }, 200);
                }
                // collapsed mode (hover-reveal): re-hide after delay
                if (!generating && panelMode === "collapsed") {
                  hoverTimerRef.current = setTimeout(
                    () => setIsHovering(false),
                    200,
                  );
                }
              }}
            >
              <FloatingPanel
                generating={generating}
                generationPhase={generationPhase}
                previewDegraded={previewDegraded}
                progress={progress}
                streamTitle={streamInfo?.title}
                length={length}
                onLengthChange={setLength}
                onGenerateAll={() => handleGenerate()}
                onCancel={handleCancel}
                onExportDocx={exportDocx}
                onExportPdf={exportPdf}
                exportingType={exportingType}
                canReview={canCreateReview}
                showReviewEntry={isTenderSourced}
                reviewProjectId={project?.reviewProjectId}
                onCreateReview={handleCreateReview}
                onViewReview={handleViewReview}
                creatingReview={creatingReview}
                materialCount={
                  contentOutline.filter((n) => n.genStatus === "succeeded").length
                }
                onShowMaterials={() => setMaterialOpen(true)}
                pinned={panelMode === "pinned"}
                onTogglePin={() =>
                  setPanelMode((m) => (m === "pinned" ? "floating" : "pinned"))
                }
                onCollapse={() => {
                  setPanelMode("collapsed");
                  setIsHovering(false);
                }}
                compact={panelCompact}
                onToggleCompact={() => {
                  manualCompactRef.current = true;
                  setPanelCompact((c) => !c);
                }}
                dragControls={dragControls}
                canCollapse={panelMode !== "pinned" && !panelCompact}
                activeChapter={activeChapter}
                activeChapterHasChildren={activeChapterHasChildren}
                onChapterAiAction={handlePanelChapterAction}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </Flex>

      {/* 素材预览抽屉 */}
      <Drawer
        isOpen={materialOpen}
        onClose={() => setMaterialOpen(false)}
        placement="right"
        size="sm"
      >
        <DrawerOverlay />
        <DrawerContent>
          <DrawerHeader borderBottom="1px solid" borderColor="gray.100">
            章节素材
          </DrawerHeader>
          <DrawerBody py={4}>
            <Text fontSize="sm" color="gray.500" mb={4}>
              已完成{" "}
              {contentOutline.filter((n) => n.genStatus === "succeeded").length}{" "}
              个章节（含人工标记完成），素材图随 AI 生成内容自动插入章末。
            </Text>
            <Grid templateColumns="repeat(2, 1fr)" gap={3}>
              {contentOutline
                .filter((n) => n.genStatus === "succeeded")
                .map((n) => (
                  <Box
                    key={n.id}
                    p={3}
                    borderRadius="lg"
                    border="1px solid"
                    borderColor="gray.200"
                    bg="gray.50"
                  >
                    <Flex align="center" gap={2}>
                      <FiCheckCircle color="var(--chakra-colors-success-500)" />
                      <Text
                        fontSize="xs"
                        fontWeight="600"
                        color="gray.700"
                        noOfLines={2}
                      >
                        {n.title}
                      </Text>
                    </Flex>
                  </Box>
                ))}
            </Grid>
          </DrawerBody>
        </DrawerContent>
      </Drawer>

      {/* 重新编排结构确认 */}
      <AlertDialog
        isOpen={unconfirmOpen}
        leastDestructiveRef={unconfirmCancelRef}
        onClose={() => {
          if (!unconfirming) setUnconfirmOpen(false);
        }}
        isCentered
      >
        <AlertDialogOverlay bg="blackAlpha.500" backdropFilter="blur(2px)">
          <AlertDialogContent borderRadius="2xl">
            <AlertDialogHeader fontSize="lg" fontWeight="700">
              重新编排结构？
            </AlertDialogHeader>
            <AlertDialogBody fontSize="sm" color="gray.600">
              返回大纲蓝图页后，正文将恢复为只读预览，直至再次确认大纲。
            </AlertDialogBody>
            <AlertDialogFooter gap={3}>
              <Button
                ref={unconfirmCancelRef}
                variant="ghost"
                onClick={() => setUnconfirmOpen(false)}
                isDisabled={unconfirming}
              >
                取消
              </Button>
              <Button
                colorScheme="primary"
                isLoading={unconfirming}
                loadingText="处理中…"
                onClick={handleUnconfirm}
              >
                确认返回
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>

      {/* 章节级 AI 弹层：重写 / 扩写 / 缩写 */}
      <ChapterAiModal
        isOpen={!!chapterAiTarget}
        outlineId={chapterAiTarget?.outlineId || 0}
        title={chapterAiTarget?.title || ""}
        defaultLength={length}
        onClose={() => setChapterAiTarget(null)}
        onConfirm={handleChapterAiConfirm}
      />

      {/* 只读文档预览：封面页 / 目录页 / 页眉页脚（导出格式） */}
      <BidDocPreview
        isOpen={previewOpen}
        onClose={() => setPreviewOpen(false)}
        cover={coverData}
        toc={tocWithPages.length > 0 ? tocWithPages : tocEntries}
        loading={exporting}
        errorMessage={pageMapFailed}
      />
    </Flex>
  );
}
