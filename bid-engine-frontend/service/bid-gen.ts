/* eslint-disable no-unused-vars */
import { useCallback, useEffect, useRef } from "react";
import useAxios from "axios-hooks";

// ===== 项目列表 =====
export const useBidGenList = (params) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    { url: "/zb/file-gen/project/list", method: "GET", params },
    { useCache: false, manual: true },
  );
  return {
    listData: data?.data || { list: [], total: 0 },
    listError: error,
    listLoading: loading,
    fetchList,
  };
};

// ===== 项目详情 =====
// 单飞窗口上限：超过该时长仍未返回就放弃复用，重新发起请求，
// 避免一次卡住的请求让后续轮询永久复用同一个悬空 promise。
const DETAIL_INFLIGHT_MAX_MS = 30000;

export const useBidGenDetail = (projectId) => {
  const [{ data, loading, error }, refetchDetail] = useAxios(
    {
      url: "/zb/file-gen/project/detail",
      method: "GET",
      params: { project_id: projectId },
    },
    // autoCancel 必须关闭：详情在生成期会被 3 秒轮询 + SSE 快照重连 + 各操作后刷新
    // 多处并发触发，axios-hooks 默认的 autoCancel 会把上一个未完成的请求取消掉，
    // 被取消的 promise 以 CanceledError 拒绝；fire-and-forget 的调用点没有 catch，
    // 就会冒泡成未处理的 Promise rejection（Next.js 开发态直接报 Unhandled Runtime Error）。
    // 并发改为单飞收敛：同一时刻只保留一个请求，并发的调用方复用同一个 promise。
    { useCache: false, manual: true, autoCancel: false },
  );

  const inFlightRef = useRef<Promise<any> | null>(null);
  const inFlightStartedAtRef = useRef(0);

  const fetchDetail = useCallback(
    (...args: any[]) => {
      const startedAt = Date.now();
      if (
        inFlightRef.current &&
        startedAt - inFlightStartedAtRef.current < DETAIL_INFLIGHT_MAX_MS
      ) {
        return inFlightRef.current;
      }
      inFlightStartedAtRef.current = startedAt;
      const request = Promise.resolve()
        .then(() => refetchDetail(...(args as [])))
        .finally(() => {
          if (inFlightRef.current === request) {
            inFlightRef.current = null;
          }
        });
      inFlightRef.current = request;
      return request;
    },
    [refetchDetail],
  );

  // 详情拉取是对账用途：失败或并发复用不应该抛出，
  // 调用方按 undefined 处理即可，避免任何调用点产生未处理的拒绝。
  const safeFetchDetail = useCallback(
    async (...args: any[]) => {
      try {
        return await fetchDetail(...args);
      } catch {
        return undefined;
      }
    },
    [fetchDetail],
  );

  useEffect(() => {
    if (projectId > 0) {
      void safeFetchDetail();
    }
  }, [projectId, safeFetchDetail]);

  return {
    detailData: data?.data || null,
    detailError: error,
    detailLoading: loading,
    fetchDetail: safeFetchDetail,
  };
};

// ===== 创建空白标书 =====
export const useBidGenCreateBlank = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/file-gen/project/create", method: "POST" },
    { manual: true },
  );
  return { createError: error, createLoading: loading, fetchCreate };
};

// ===== 从招标文件创建（multipart）=====
export const useBidGenCreateFromTender = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/file-gen/project/create-from-tender", method: "POST" },
    { manual: true },
  );
  return { createError: error, createLoading: loading, fetchCreate };
};

// ===== 从模板创建（multipart）=====
export const useBidGenCreateFromTemplate = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/file-gen/project/create-from-template", method: "POST" },
    { manual: true },
  );
  return { createError: error, createLoading: loading, fetchCreate };
};

// ===== 大纲确认 =====
export const useBidGenConfirmOutline = () => {
  const [{ loading, error }, fetchConfirm] = useAxios(
    { url: "/zb/file-gen/project/confirm-outline", method: "POST" },
    { manual: true },
  );
  return { confirmError: error, confirmLoading: loading, fetchConfirm };
};

// ===== 撤销大纲确认（draft → outline_review，仅未生成章节时）=====
export const useBidGenUnconfirmOutline = () => {
  const [{ loading, error }, fetchUnconfirm] = useAxios(
    { url: "/zb/file-gen/outline/unconfirm", method: "POST" },
    { manual: true },
  );
  return { unconfirmError: error, unconfirmLoading: loading, fetchUnconfirm };
};

// ===== 自动保存文档 =====
export const useBidGenSaveDoc = () => {
  const [{ loading, error }, fetchSave] = useAxios(
    { url: "/zb/file-gen/project/save", method: "PUT" },
    { manual: true },
  );
  return { saveError: error, saveLoading: loading, fetchSave };
};

// ===== 删除项目 =====
export const useBidGenDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/file-gen/project/delete", method: "DELETE" },
    { manual: true },
  );
  return { deleteError: error, deleteLoading: loading, fetchDelete };
};

// ===== 批量删除项目 =====
export const useBidGenBatchDelete = () => {
  const [{ loading, error }, fetchBatchDelete] = useAxios(
    { url: "/zb/file-gen/project/delete/batch", method: "DELETE" },
    { manual: true },
  );
  return {
    batchDeleteError: error,
    batchDeleteLoading: loading,
    fetchBatchDelete,
  };
};

// ===== 大纲 CRUD =====
export const useBidGenOutlineAdd = () => {
  const [{ loading, error }, fetchAdd] = useAxios(
    { url: "/zb/file-gen/outline/add", method: "POST" },
    { manual: true },
  );
  return { addError: error, addLoading: loading, fetchAdd };
};

export const useBidGenOutlineUpdate = () => {
  const [{ loading, error }, fetchUpdate] = useAxios(
    { url: "/zb/file-gen/outline/update", method: "PUT" },
    { manual: true },
  );
  return { updateError: error, updateLoading: loading, fetchUpdate };
};

export const useBidGenOutlineDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/file-gen/outline/delete", method: "DELETE" },
    { manual: true },
  );
  return { deleteError: error, deleteLoading: loading, fetchDelete };
};

// ===== 大纲同步（编辑器文档标题结构 → 大纲表对账）=====
export const useBidGenOutlineSync = () => {
  const [{ loading, error }, fetchSync] = useAxios(
    { url: "/zb/file-gen/outline/sync", method: "POST" },
    { manual: true },
  );
  return { syncError: error, syncLoading: loading, fetchSync };
};

// ===== 人工标记章节写作完成/取消完成 =====
// 人工撰写的章节不走 AI 生成流程，gen_status 需要手工置位，
// 置位后才会被计入项目整体完成度（全部章节完成 → 项目已完成）。
export const useBidGenOutlineComplete = () => {
  const [{ loading, error }, fetchComplete] = useAxios(
    { url: "/zb/file-gen/outline/complete", method: "POST" },
    { manual: true },
  );
  return { completeError: error, completeLoading: loading, fetchComplete };
};

// ===== 大纲结构快照（拖拽排序/跨级移动，单事务）=====
export const useBidGenOutlineApply = () => {
  const [{ loading, error }, fetchApply] = useAxios(
    { url: "/zb/file-gen/outline/apply", method: "POST" },
    { manual: true },
  );
  return { applyError: error, applyLoading: loading, fetchApply };
};

// ===== 取消生成 =====
export const useBidGenCancel = () => {
  const [{ loading, error }, fetchCancel] = useAxios(
    { url: "/zb/file-gen/generate/cancel", method: "POST" },
    { manual: true },
  );
  return { cancelError: error, cancelLoading: loading, fetchCancel };
};

// ===== 导出 =====
export const useBidGenRecordExport = () => {
  const [{ loading, error }, fetchRecord] = useAxios(
    { url: "/zb/file-gen/export/record", method: "POST" },
    { manual: true },
  );
  return { recordError: error, recordLoading: loading, fetchRecord };
};

export const useBidGenExportPdf = () => {
  const [{ loading, error }, fetchExport] = useAxios(
    { url: "/zb/file-gen/export/pdf", method: "POST" },
    { manual: true },
  );
  return { exportError: error, exportLoading: loading, fetchExport };
};

// ===== 导出页码定位（两遍导出第一遍：DOCX → 每章真实页码）=====
export const useBidGenExportPageMap = () => {
  const [{ loading, error }, fetchPageMap] = useAxios(
    { url: "/zb/file-gen/export/page-map", method: "POST" },
    { manual: true },
  );
  return { pageMapError: error, pageMapLoading: loading, fetchPageMap };
};

// ===== 可恢复后台生成 =====
// 章节生成模式：write-撰写 / rewrite-重写 / expand-扩写 / condense-缩写
export type GenMode = "write" | "rewrite" | "expand" | "condense";

export type GenerationPhase =
  | "idle"
  | "queued"
  | "running"
  | "reconnecting"
  | "stopping";

export type GenerationTask = {
  id: number;
  projectId: number;
  taskType: string;
  status: string;
  progress: number;
  currentOutlineId: number;
  completedCount: number;
  totalCount: number;
  errorMsg?: string;
  lengthTier?: string;
  genMode?: string;
  attemptCount?: number;
};

export type GenStreamHandlers = {
	// EventSource 连接本身的状态；断线不代表后台任务失败。
  onConnection?: (state: "open" | "reconnecting") => void;
  onTaskState?: (data: any) => void;
  onSnapshot?: (data: any) => void;
  onChapterStart?: (data: { outlineId: number; title: string }) => void;
  onDelta?: (data: { outlineId: number; text: string }) => void;
  onChapterDone?: (data: { outlineId: number; json: any[] }) => void;
  onChapterError?: (data: { outlineId: number; msg: string }) => void;
  onProgress?: (data: { current: number; total: number; pct: number }) => void;
  onError?: (data: { msg: string }) => void;
  onDone?: () => void;
  onCancelled?: () => void;
};

export async function startGenerate(
  url: string,
  body: {
    projectId: number;
    outlineIds?: number[];
    length?: string;
    mode?: GenMode;
    instruction?: string;
  },
): Promise<GenerationTask> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  let payload: any = null;
  try {
    payload = await res.json();
  } catch {
    throw new Error("服务器返回了无效的生成任务响应");
  }
  if (!res.ok) {
    throw new Error(payload?.msg || `请求失败（${res.status}）`);
  }
  if (payload?.code !== 200 || !payload?.data?.id) {
    throw new Error(payload?.msg || "创建生成任务失败");
  }
  return payload.data as GenerationTask;
}

export function subscribeGenerate(
  projectId: number,
  taskId: number,
  handlers: GenStreamHandlers,
): () => void {
  const source = new EventSource(
    `/api/zb/file-gen/generate/events?projectId=${encodeURIComponent(projectId)}&taskId=${encodeURIComponent(taskId)}`,
    { withCredentials: true },
  );
  let closed = false;
  const parse = (event: MessageEvent): any | null => {
    try {
      return JSON.parse(event.data || "{}");
    } catch {
      return null;
    }
  };
  const listen = (name: string, callback: (payload: any) => void) => {
    source.addEventListener(name, (event) => {
      const payload = parse(event as MessageEvent);
      if (payload) callback(payload);
    });
  };

  source.onopen = () => handlers.onConnection?.("open");
  source.onerror = () => {
    if (!closed) handlers.onConnection?.("reconnecting");
  };
  listen("task_state", (data) => handlers.onTaskState?.(data));
  listen("snapshot", (data) => handlers.onSnapshot?.(data));
  listen("chapter_start", (data) => handlers.onChapterStart?.(data));
  listen("delta", (data) => handlers.onDelta?.(data));
  listen("chapter_done", (data) => handlers.onChapterDone?.(data));
  listen("chapter_error", (data) => handlers.onChapterError?.(data));
  listen("progress", (data) => handlers.onProgress?.(data));
  listen("error", (data) => {
    closed = true;
    source.close();
    handlers.onError?.(data);
  });
  listen("done", () => {
    closed = true;
    source.close();
    handlers.onDone?.();
  });
  listen("cancelled", () => {
    closed = true;
    source.close();
    handlers.onCancelled?.();
  });

  return () => {
    closed = true;
    source.close();
  };
}

// ===== AI 重写选中片段（SSE，终态审阅场景，独立于全文生成）=====
export type RewriteStreamHandlers = {
  onDelta?: (data: { text: string }) => void;
  onDone?: (data: { text: string }) => void;
  onError?: (data: { msg: string }) => void;
  onCancelled?: () => void;
};

function dispatchRewriteBlock(block: string, handlers: RewriteStreamHandlers) {
  let event = "message";
  let data = "";
  for (const line of block.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data = line.slice(5).trim();
  }
  if (!data) return;
  let payload: any = {};
  try {
    payload = JSON.parse(data);
  } catch {
    return;
  }
  switch (event) {
    case "delta":
      handlers.onDelta?.(payload);
      break;
    case "done":
      handlers.onDone?.(payload);
      break;
    case "error":
      handlers.onError?.(payload);
      break;
    case "cancelled":
      handlers.onCancelled?.();
      break;
    default:
      break;
  }
}

export async function streamRewrite(
  body: {
    projectId: number;
    text: string;
    context?: string;
    instruction?: string;
  },
  handlers: RewriteStreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch("/api/zb/file-gen/rewrite", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    });
  } catch (e) {
    if ((e as any)?.name === "AbortError") {
      handlers.onCancelled?.();
    } else {
      handlers.onError?.({ msg: "网络错误，无法连接服务器" });
    }
    return;
  }
  if (!res.ok) {
    handlers.onError?.({ msg: `请求失败（${res.status}）` });
    return;
  }
  const reader = res.body?.getReader();
  if (!reader) {
    handlers.onError?.({ msg: "响应流不可用" });
    return;
  }
  const decoder = new TextDecoder("utf-8");
  let buffer = "";
  for (;;) {
    // eslint-disable-next-line no-await-in-loop
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let idx = buffer.indexOf("\n\n");
    while (idx >= 0) {
      const block = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      dispatchRewriteBlock(block, handlers);
      idx = buffer.indexOf("\n\n");
    }
  }
  if (buffer.trim()) dispatchRewriteBlock(buffer, handlers);
}
