import { useCallback, useEffect, useState } from "react";
import useAxios from "axios-hooks";
import useSWR from "swr";

// ================================================================
// 招标情报站模块接口（/zb/intel/*、/sys/llm-config/*）
// ================================================================

export interface IntelNotice {
  id: number;
  title: string;
  url: string;
  source_key: string;
  source_name: string;
  source_category: string;
  publisher: string;
  agency: string;
  project_code: string;
  budget_text: string;
  budget_amount: number | null;
  region_province: string;
  region_city: string;
  notice_type: string;
  notice_type_name: string;
  notice_stage: string;
  publish_date: string;
  deadline_at: string;
  industries: string[];
  industry_names: string[];
  tag_status: string;
  favorited: boolean;
  pinned: boolean;
  origin: string;
  status: string;
  import_batch: string;
  first_seen_at: string;
}

/** 情报管理列表项：在情报大厅字段之上补充管理态信息。 */
export interface IntelAdminNotice extends IntelNotice {
  admin_note: string;
  pinned_at: string;
}

export interface IntelNoticeAdminSummary {
  total: number;
  normal: number;
  hidden: number;
  archived: number;
  pinned: number;
  manual: number;
}

/** 手工录入 / 编辑情报的表单载荷（budget_amount 单位为元）。 */
export interface IntelNoticePayload {
  id?: number;
  title?: string;
  publisher?: string;
  agency?: string;
  project_code?: string;
  budget_text?: string;
  budget_amount?: number | null;
  region_province?: string;
  region_city?: string;
  notice_type?: string;
  publish_date?: string;
  deadline_at?: string;
  url?: string;
  source_name?: string;
  body_text?: string;
  body_markdown?: string;
  admin_note?: string;
  industries?: string[];
}

export interface IntelNoticeImportResult {
  created: number;
  updated: number;
  skipped: number;
  failed: Array<{ row: number; title?: string; message: string }>;
  total: number;
  overwrite: boolean;
  batch: string;
  notice: string;
}

export interface IntelNoticeDetail extends IntelNotice {
  body_html: string;
  body_markdown: string;
  body_text: string;
  attachments: Array<{ name: string; url: string }>;
  fetch_strategy: string;
  extract_confidence: number;
  extract_warnings: string[];
  insight: {
    content_md: string;
    status: string;
    created_at: string;
  } | null;
}

export interface IntelFilters {
  industries: Array<{ code: string; name: string }>;
  regions: string[];
  notice_types: Array<{ code: string; name: string }>;
  sources: Array<{ source_key: string; name: string; category: string }>;
}

export interface IntelNoticeQuery {
  keyword?: string;
  industries?: string[];
  notice_types?: string[];
  regions?: string[];
  source_keys?: string[];
  budget_min?: number;
  budget_max?: number;
  date_from?: string;
  date_to?: string;
  /** 采集时间下限：最近 N 天（含今天），由后端按服务器时区换算 */
  collect_within_days?: number;
  collect_from?: string;
  favorite_only?: boolean;
  order?: string;
  pageNum?: number;
  pageSize?: number;
}

/** 情报管理列表查询参数：在情报大厅条件上增加状态与来源类型。 */
export interface IntelNoticeAdminQuery extends IntelNoticeQuery {
  statuses?: string[];
  origins?: string[];
  import_batch?: string;
  pinned_only?: boolean;
}

export interface IntelSubscription {
  id: number;
  name: string;
  keywords: string[];
  match_mode: string;
  industries: string[];
  industry_names: string[];
  regions: string[];
  notice_types: string[];
  budget_min: number | null;
  budget_max: number | null;
  enabled: boolean;
  last_matched_at: string;
  matched_count: number;
  /** 该订阅命中的未读提醒数（由订阅列表一次性聚合返回，抽屉头部与卡片入口共用） */
  unread_count: number;
  created_at: string;
}

export interface IntelAlert {
  id: number;
  subscription_id: number;
  subscription_name: string;
  notice_id: number;
  notice_title: string;
  matched_keywords: string[];
  matched_reason: string;
  is_read: boolean;
  created_at: string;
}

export interface IntelSubscriptionDraft {
  name: string;
  keywords: string[];
  match_mode: string;
  industries: string[];
  industry_names: string[];
  regions: string[];
  notice_types: string[];
  budget_min: number | null;
  budget_max: number | null;
  summary?: string;
}

export interface IntelSource {
  source_key: string;
  name: string;
  homepage_url: string;
  list_url: string;
  category: string;
  region: string;
  discovery_mode: string;
  needs_browser: boolean;
  enabled: boolean;
  priority: number;
  cursor: string;
  last_run_at: string;
  last_success_at: string;
  last_error: string;
  consecutive_failures: number;
  params: string;
  industry_hint: string;
}

export interface IntelSourceImportResult {
  created: number;
  updated: number;
  skipped: number;
  failed: Array<{ row: number; source_key?: string; message: string }>;
  total: number;
  overwrite: boolean;
  notice: string;
}

export interface IntelSourceProbeResult {
  reachable: boolean;
  items_found?: number;
  sample?: string[];
  errors?: string[];
  cursor?: string;
  probed_at?: string;
  message?: string;
}

export interface IntelRunSourceDetail {
  source_key: string;
  source_name: string;
  status: string;
  cursor_before: string;
  cursor_after: string;
  discovered: number;
  extracted: number;
  inserted: number;
  skipped: number;
  duration_ms: number;
  error: string;
}

export interface IntelSchedule {
  cron_expr: string;
  enabled: boolean;
  next_runs: string[];
  example: string;
  updated_at: string;
  updated_by: number;
}

export interface IntelRun {
  run_id: string;
  trigger_type: string;
  scope: string;
  scope_label: string;
  source_keys: string[];
  retry_of: string;
  status: string;
  started_at: string;
  finished_at: string | null;
  source_total: number;
  source_success: number;
  source_failed: number;
  discovered_count: number;
  extracted_count: number;
  inserted_count: number;
  duplicate_count: number;
  enriched_count: number;
  /** 本轮匹配出的提醒数（由订阅匹配任务回填） */
  matched_count: number;
  error_summary: string;
}

export interface IntelRunRetryResult {
  run_id: string;
  retry_of: string;
  scope: string;
  source_keys: string[];
  trigger_type: string;
}

// 采集批次概览：后端按全量批次统计，避免前端只统计当前分页导致口径失真。
export interface IntelRunSummary {
  total: number;
  running: number;
  success: number;
  partial: number;
  failed: number;
}

/** 订阅匹配任务（管理端作业面）。 */
export interface IntelMatchTask {
  task_no: string;
  task_type: string;
  task_type_name: string;
  scope_kind: string;
  scope_ref: string;
  range_from: string;
  range_to: string;
  notice_total: number;
  scanned_count: number;
  matched_notice_count: number;
  alert_count: number;
  status: string;
  priority: string;
  cancel_requested: boolean;
  attempts: number;
  retry_of: string;
  subscription_id: number;
  user_id: number;
  /** 订阅回溯类任务对应的订阅名称（其它类型为空） */
  subscription_name: string;
  /** 订阅回溯类任务对应的用户昵称/登录名与手机号 */
  user_name: string;
  user_mobile: string;
  last_error: string;
  started_at: string;
  finished_at: string;
  created_at: string;
  /** 排队中的执行顺位（1 = 下一个执行）；0 表示不在队列或无法确定 */
  queue_order: number;
}

export interface IntelMatchOverview {
  counts: Record<string, number>;
  today_alerts: number;
  active: IntelMatchTask[];
  recent: IntelMatchTask[];
}

export interface SystemLLMConfig {
  id: number;
  name: string;
  base_url: string;
  api_key: string;
  model: string;
  endpoint_path: string;
  context_window_tokens: number;
  max_output_tokens: number;
  sort: number;
  complete: boolean;
  update_time: string;
}

// 稳定的空集合常量（必须放在类型声明之后、hooks 之前）。
//
// 教训：接口尚未返回时如果用 `data?.data || []` 兜底，每次渲染都会生成新的引用；
// 一旦调用方把它作为 useEffect 的依赖、或把它拷贝进本地 state，就会形成
// “渲染 → 新数组 → effect → setState → 再渲染”的无限循环（React 报
// Maximum update depth exceeded）。这里统一复用同一个数组实例，杜绝该问题。
const EMPTY_NOTICES: IntelNotice[] = [];
const EMPTY_SUBSCRIPTIONS: IntelSubscription[] = [];
/** 抽屉与列表共用：接口未返回时的稳定空数组引用，避免每次渲染新建数组。 */
export const EMPTY_ALERTS: IntelAlert[] = [];
const EMPTY_SOURCES: IntelSource[] = [];
const EMPTY_RUNS: IntelRun[] = [];
const EMPTY_MATCH_TASKS: IntelMatchTask[] = [];
const EMPTY_LLM_CONFIGS: SystemLLMConfig[] = [];
const EMPTY_ADMIN_NOTICES: IntelAdminNotice[] = [];

// ===== 情报大厅 =====

/**
 * 数组型筛选参数的序列化口径。
 *
 * axios 默认把数组序列化成 `industries[]=a&industries[]=b`，而后端 Gin 用
 * `c.QueryArray("industries")` 读取，取不到带 `[]` 的键 —— 结果是筛选条件静默失效
 * （接口返回全量数据，用户以为筛选没生效）。这里统一改成重复键写法。
 */
export const intelParamsSerializer = { indexes: null } as const;

export const useIntelNotices = (
  params: IntelNoticeQuery,
  immediate = false,
) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    {
      url: "/zb/intel/notices",
      method: "GET",
      params,
      paramsSerializer: intelParamsSerializer,
    },
    { useCache: false, manual: true },
  );
  useEffect(() => {
    if (immediate) fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [immediate]);
  return {
    notices: (data?.data?.list ?? EMPTY_NOTICES) as IntelNotice[],
    total: (data?.data?.total || 0) as number,
    noticesLoading: loading,
    noticesError: error,
    fetchNotices: fetchList,
  };
};

export const useIntelNoticeDetail = (noticeId?: string | number) => {
  const [{ data, loading, error }, fetchDetail] = useAxios(
    { url: `/zb/intel/notices/${noticeId}`, method: "GET" },
    { useCache: false, manual: true },
  );
  useEffect(() => {
    if (noticeId) fetchDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [noticeId]);
  return {
    detail: (data?.data || null) as IntelNoticeDetail | null,
    detailLoading: loading,
    detailError: error,
    fetchDetail,
  };
};

export const useIntelFavorite = () => {
  const [{ loading }, fetchFavorite] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { favoriteLoading: loading, fetchFavorite };
};

export const useIntelFilters = () => {
  const [{ data, loading, error }] = useAxios(
    { url: "/zb/intel/filters", method: "GET" },
    { useCache: false },
  );
  return {
    filters: (data?.data || null) as IntelFilters | null,
    filtersLoading: loading,
    filtersError: error,
  };
};

export const useIntelInsight = () => {
  const [{ loading, error }, fetchInsight] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { insightLoading: loading, insightError: error, fetchInsight };
};

// ===== 订阅与提醒 =====
export const useIntelSubscriptions = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/zb/intel/subscriptions", method: "GET" },
    { useCache: false },
  );
  return {
    subscriptions: (data?.data ?? EMPTY_SUBSCRIPTIONS) as IntelSubscription[],
    subscriptionsLoading: loading,
    subscriptionsError: error,
    refreshSubscriptions: refresh,
  };
};

export const useIntelSubscriptionSave = () => {
  const [{ loading, error }, fetchSave] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  const [{ loading: updating }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return {
    saveLoading: loading || updating,
    saveError: error,
    fetchSave,
    fetchUpdate,
  };
};

export const useIntelSubscriptionDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, deleteError: error, fetchDelete };
};

export const useIntelSubscriptionParse = () => {
  const [{ loading, error }, fetchParse] = useAxios(
    { url: "/zb/intel/subscriptions/parse", method: "POST" },
    { manual: true },
  );
  return { parseLoading: loading, parseError: error, fetchParse };
};

/**
 * 提醒列表查询（手动触发）。
 *
 * 提醒只在订阅卡片的抽屉里按订阅展示，且需要滚动加载，因此这里只暴露
 * `fetchAlerts`，由抽屉配合 `useInfiniteList` 自行管理分页与累积。
 */
export const useIntelAlerts = () => {
  const [{ loading, error }, fetchAlerts] = useAxios(
    { url: "/zb/intel/alerts", method: "GET" },
    { manual: true, useCache: false },
  );
  return { alertsLoading: loading, alertsError: error, fetchAlerts };
};

/** 标记已读 / 未读：ids 为空且 all=true 表示“全部已读”。 */
export const useIntelAlertStatus = () => {
  const [{ loading, error }, fetchStatus] = useAxios(
    { url: "/zb/intel/alerts/status", method: "POST" },
    { manual: true },
  );
  return { statusLoading: loading, statusError: error, fetchStatus };
};

/** 删除提醒（硬删除）。 */
export const useIntelAlertDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/intel/alerts/delete", method: "POST" },
    { manual: true },
  );
  return { deleteLoading: loading, deleteError: error, fetchDelete };
};

/**
 * 未读提醒数：导航角标与“订阅与提醒”页共用；传订阅时收敛到单个订阅（抽屉头部）。
 * 30s 轮询 + 页面重新聚焦时刷新，保证角标不会长时间落后于真实值。
 */
export const useIntelUnreadCount = (
  refreshInterval?: number,
  subscriptionId?: number,
) => {
  // 不传订阅时是全局未读（导航角标）；传订阅时只统计该订阅（抽屉头部）
  const key =
    subscriptionId && subscriptionId > 0
      ? `/api/zb/intel/alerts/unread-count?subscription_id=${subscriptionId}`
      : "/api/zb/intel/alerts/unread-count";
  const { data, error, mutate } = useSWR(key, {
    refreshInterval: refreshInterval ?? 30000,
    revalidateOnFocus: true,
  });
  const unread = (data?.data?.unread ?? 0) as number;
  const refreshUnread = useCallback(() => mutate(), [mutate]);
  return { unread, unreadError: error, refreshUnread };
};

// ===== 采集运维（超管）=====
export const useIntelSources = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/zb/intel/sources", method: "GET" },
    { useCache: false },
  );
  return {
    sources: (data?.data ?? EMPTY_SOURCES) as IntelSource[],
    sourcesLoading: loading,
    sourcesError: error,
    refreshSources: refresh,
  };
};

export const useIntelSourceSave = () => {
  const [{ loading, error }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { sourceSaveLoading: loading, sourceSaveError: error, fetchUpdate };
};

export const useIntelSourceDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return {
    sourceDeleteLoading: loading,
    sourceDeleteError: error,
    fetchDelete,
  };
};

export const useIntelSourceProbe = () => {
  const [{ loading, error }, fetchProbe] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { probeLoading: loading, probeError: error, fetchProbe };
};

// ===== 情报管理（超管）=====
export const useIntelAdminNotices = (
  params: IntelNoticeAdminQuery,
  immediate = false,
) => {
  const [{ data, loading, error }, fetchList] = useAxios(
    {
      url: "/zb/intel/admin/notices",
      method: "GET",
      params,
      paramsSerializer: intelParamsSerializer,
    },
    { useCache: false, manual: true },
  );
  useEffect(() => {
    if (immediate) fetchList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [immediate]);
  return {
    adminNotices: (data?.data?.list ??
      EMPTY_ADMIN_NOTICES) as IntelAdminNotice[],
    adminNoticesTotal: (data?.data?.total || 0) as number,
    adminNoticesSummary: (data?.data?.summary ??
      null) as IntelNoticeAdminSummary | null,
    adminNoticesLoading: loading,
    adminNoticesError: error,
    fetchAdminNotices: fetchList,
  };
};

export const useIntelNoticeSave = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/zb/intel/admin/notices", method: "POST" },
    { manual: true },
  );
  const [{ loading: updating }, fetchUpdate] = useAxios(
    { url: "/zb/intel/admin/notices", method: "PUT" },
    { manual: true },
  );
  return {
    noticeSaveLoading: loading || updating,
    noticeSaveError: error,
    fetchCreate,
    fetchUpdate,
  };
};

export const useIntelNoticeStatus = () => {
  const [{ loading, error }, fetchStatus] = useAxios(
    { url: "/zb/intel/admin/notices/status", method: "POST" },
    { manual: true },
  );
  return {
    noticeStatusLoading: loading,
    noticeStatusError: error,
    fetchStatus,
  };
};

export const useIntelNoticePin = () => {
  const [{ loading, error }, fetchPin] = useAxios(
    { url: "/zb/intel/admin/notices/pin", method: "POST" },
    { manual: true },
  );
  return { noticePinLoading: loading, noticePinError: error, fetchPin };
};

export const useIntelNoticeDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { url: "/zb/intel/admin/notices/delete", method: "POST" },
    { manual: true },
  );
  return {
    noticeDeleteLoading: loading,
    noticeDeleteError: error,
    fetchDelete,
  };
};

export const useIntelNoticeImport = () => {
  const [{ loading, error }, fetchImport] = useAxios(
    { url: "/zb/intel/admin/notices/import", method: "POST" },
    { manual: true },
  );
  return {
    noticeImportLoading: loading,
    noticeImportError: error,
    fetchImport,
  };
};

export const useIntelSourceImport = () => {
  const [{ loading, error }, fetchImport] = useAxios(
    { url: "/zb/intel/sources/import", method: "POST" },
    { manual: true },
  );
  return { importLoading: loading, importError: error, fetchImport };
};

export const useIntelRuns = (pageSize = 10) => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/zb/intel/runs", method: "GET", params: { pageNum: 1, pageSize } },
    { useCache: false },
  );
  return {
    runs: (data?.data?.list ?? EMPTY_RUNS) as IntelRun[],
    runsTotal: (data?.data?.total || 0) as number,
    runsSummary: (data?.data?.summary ?? null) as IntelRunSummary | null,
    runsLoading: loading,
    runsError: error,
    refreshRuns: refresh,
  };
};

export const useIntelTriggerCollect = () => {
  const [{ loading, error }, fetchTrigger] = useAxios(
    { url: "/zb/intel/collect/run", method: "POST" },
    { manual: true },
  );
  return { triggerLoading: loading, triggerError: error, fetchTrigger };
};

export const useIntelRunDetail = () => {
  const [{ loading, error }, fetchDetail] = useAxios(
    { method: "GET" },
    { manual: true },
  );
  return { runDetailLoading: loading, runDetailError: error, fetchDetail };
};

export const useIntelRunRetry = () => {
  const [{ loading, error }, fetchRetry] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { runRetryLoading: loading, runRetryError: error, fetchRetry };
};

export const useIntelRunDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { runDeleteLoading: loading, runDeleteError: error, fetchDelete };
};

// ===== 订阅匹配作业面（超管）=====

export const useIntelMatchOverview = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/zb/intel/matches/overview", method: "GET" },
    { useCache: false },
  );
  return {
    overview: (data?.data || null) as IntelMatchOverview | null,
    overviewLoading: loading,
    overviewError: error,
    refreshOverview: refresh,
  };
};

export const useIntelMatchTasks = (status = "", taskType = "") => {
  const [{ data, loading, error }, refresh] = useAxios(
    {
      url: "/zb/intel/matches/tasks",
      method: "GET",
      params: {
        pageNum: 1,
        pageSize: 20,
        status: status || undefined,
        task_type: taskType || undefined,
      },
    },
    { useCache: false },
  );
  return {
    matchTasks: (data?.data?.list ?? EMPTY_MATCH_TASKS) as IntelMatchTask[],
    matchTasksTotal: (data?.data?.total || 0) as number,
    matchTasksLoading: loading,
    matchTasksError: error,
    refreshMatchTasks: refresh,
  };
};

export const useIntelMatchPriority = () => {
  const [{ loading, error }, fetchPriority] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return { priorityLoading: loading, priorityError: error, fetchPriority };
};

export const useIntelMatchCancel = () => {
  const [{ loading, error }, fetchCancel] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { cancelLoading: loading, cancelError: error, fetchCancel };
};

export const useIntelMatchRetry = () => {
  const [{ loading, error }, fetchRetry] = useAxios(
    { method: "POST" },
    { manual: true },
  );
  return { matchRetryLoading: loading, matchRetryError: error, fetchRetry };
};

export const useIntelMatchDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { matchDeleteLoading: loading, matchDeleteError: error, fetchDelete };
};

export const useIntelMatchRescan = () => {
  const [{ loading, error }, fetchRescan] = useAxios(
    { url: "/zb/intel/matches/rescan", method: "POST" },
    { manual: true },
  );
  return { rescanLoading: loading, rescanError: error, fetchRescan };
};

export const useIntelSchedule = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/zb/intel/schedule", method: "GET" },
    { useCache: false },
  );
  return {
    schedule: (data?.data || null) as IntelSchedule | null,
    scheduleLoading: loading,
    scheduleError: error,
    refreshSchedule: refresh,
  };
};

export const useIntelScheduleSave = () => {
  const [{ loading, error }, fetchSave] = useAxios(
    { url: "/zb/intel/schedule", method: "PUT" },
    { manual: true },
  );
  return { scheduleSaveLoading: loading, scheduleSaveError: error, fetchSave };
};

// ===== 系统模型配置（超管）=====
export const useSystemLLMConfigs = () => {
  const [{ data, loading, error }, refresh] = useAxios(
    { url: "/sys/llm-config", method: "GET" },
    { useCache: false },
  );
  return {
    configs: (data?.data ?? EMPTY_LLM_CONFIGS) as SystemLLMConfig[],
    configsLoading: loading,
    configsError: error,
    refreshConfigs: refresh,
  };
};

export const useSystemLLMConfigSave = () => {
  const [{ loading, error }, fetchCreate] = useAxios(
    { url: "/sys/llm-config", method: "POST" },
    { manual: true },
  );
  const [{ loading: updating }, fetchUpdate] = useAxios(
    { method: "PUT" },
    { manual: true },
  );
  return {
    saveLoading: loading || updating,
    saveError: error,
    fetchCreate,
    fetchUpdate,
  };
};

export const useSystemLLMConfigDelete = () => {
  const [{ loading, error }, fetchDelete] = useAxios(
    { method: "DELETE" },
    { manual: true },
  );
  return { deleteLoading: loading, deleteError: error, fetchDelete };
};

export const useSystemLLMConfigTest = () => {
  const [{ loading, error }, fetchTest] = useAxios(
    { url: "/sys/llm-config/test", method: "POST" },
    { manual: true },
  );
  return { testLoading: loading, testError: error, fetchTest };
};

/** 本地受控筛选状态，避免每次输入都请求接口。 */
export const useDebouncedValue = <T>(value: T, delay = 400) => {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timer);
  }, [value, delay]);
  return debounced;
};
