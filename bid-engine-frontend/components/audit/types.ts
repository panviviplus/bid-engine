// ================================================================
// 标书审核模块（V2）：类型与展示元数据
// ================================================================

export interface ReviewProject {
  id: number;
  name: string;
  create_type: string; // manual / gen
  status: string; // running / succeed / failed
  stage: string;
  progress: number;
  run_count: number;
  stage_status: Record<string, string>;
  retryable_stage: string;
  last_error: string;
  started_at: string | null;
  finished_at: string | null;
  cancelled_at: string | null;
  source_bid_gen_project_id: number;
  is_anonymous: boolean;
  tender_file_count: number;
  bid_file_count: number;
  total_items: number;
  passed_items: number;
  warning_items: number;
  error_items: number;
  todo_items: number;
  scoring_total: number;
  scoring_max: number;
  created_at: string;
  updated_at: string;
}

export interface ReviewFile {
  id: number;
  project_id: number;
  file_type: string; // tender / bid
  file_role: string; // main / addendum
  file_name: string;
  file_bucket: string;
  file_object: string;
  file_url: string;
  pdf_object: string;
  pdf_url: string;
  page_count: number;
  char_count: number;
  sort_order: number;
  parse_status: string;
  parse_error?: string;
}

export interface Finding {
  id: number;
  status: string; // pass / warning / error / na / not_found
  severity: string; // high / medium / low
  reason: string;
  suggestion: string;
  confidence: string;
  engine: string; // llm / rule
  model: string;
  latency_ms: number;
  created_at: string;
}

export interface Evidence {
  id: number;
  side: string; // tender / bid
  file_id: number;
  file_name: string;
  page_no: number;
  quote: string;
  bbox_left: number;
  bbox_top: number;
  bbox_width: number;
  bbox_height: number;
  match_score: number;
}

/** 溯源引用（原文预览跳转用；由 Evidence 转换得到） */
export interface TraceRef {
  fileId: number;
  fileName: string;
  page: number;
  quote: string;
}

/** Evidence[] → TraceRef[]（仅保留可定位到文件与页码的证据） */
export function toTraceRefs(evidences?: Evidence[]): TraceRef[] {
  if (!evidences || evidences.length === 0) return [];
  return evidences
    .filter((e) => e.file_id > 0)
    .map((e) => ({
      fileId: e.file_id,
      fileName: e.file_name || "",
      page: e.page_no || 1,
      quote: e.quote || "",
    }));
}

export interface ChecklistItem {
  id: number;
  project_id: number;
  item_key: string;
  dimension: string; // compliance / completeness / competitiveness / format
  category: string;
  title: string;
  requirement: string;
  expected_evidence: string;
  severity: string;
  source: string; // preset / analysis / rule / user / format
  rule_id: number;
  tender_page: number;
  tender_quote: string;
  full_score: number;
  review_status: string; // pending / confirmed / rejected
  review_note: string;
  reviewed_at: string | null;
  sort_order: number;
  is_user_edited: boolean;
  finding: Finding | null;
  evidences: Evidence[];
}

export interface Remediation {
  id: number;
  checklist_item_id: number;
  dimension: string;
  title: string;
  suggestion: string;
  severity: string;
  status: string; // todo / doing / done / ignored
  owner_user_id: number;
  note: string;
  resolved_at: string | null;
}

export interface DimensionStat {
  dimension: string;
  label: string;
  total: number;
  passed: number;
  warning: number;
  error: number;
  not_found: number;
  na: number;
  pending: number;
  score_total: number;
  score_max: number;
}

export interface StageRun {
  id: number;
  stage: string;
  status: string;
  attempts: number;
  total: number;
  completed: number;
  failed: number;
  last_error: string;
  started_at: string | null;
  finished_at: string | null;
}

/** 阶段目录 + 运行状态（后端 detail.stages 下发） */
export interface StageInfo {
  stage: string;
  label: string;
  order: number;
  weight: number;
  status: string;
  attempts: number;
  total: number;
  completed: number;
  failed: number;
  progress: number;
  last_error: string;
  started_at: string | null;
  finished_at: string | null;
  retryable: boolean;
  hint: string;
}

export interface AuditRule {
  id: number;
  dimension: string;
  category: string;
  title: string;
  requirement: string;
  expected_evidence: string;
  severity: string;
  applies_when: string;
  enabled: boolean;
  hit_count: number;
  version: number;
  updated_at: string;
}

export interface ReviewDetail {
  project: ReviewProject;
  files: ReviewFile[];
  dimensions: DimensionStat[];
  scoring_total: number;
  scoring_max: number;
  items: ChecklistItem[];
  remediations: Remediation[];
  stage_runs: StageRun[];
  stages: StageInfo[];
  op_logs: { id: number; action: string; detail: string; created_at: string }[];
}

// ===== 阶段 =====

export const STAGE_STATUS = {
  PENDING: "pending",
  RUNNING: "running",
  SUCCEEDED: "succeeded",
  FAILED: "failed",
  SKIPPED: "skipped",
} as const;

export const STAGE_LABELS: Record<string, string> = {
  tender_parse: "招标文件解析",
  bid_parse: "投标文件解析",
  checklist_build: "清单生成",
  evidence_match: "证据定位",
  verdict: "逐条判定",
  format_scan: "暗标版式检查",
  scoring: "评分对标",
  finalize: "汇总完成",
};

export const STAGE_ORDER = [
  "tender_parse",
  "bid_parse",
  "checklist_build",
  "evidence_match",
  "verdict",
  "format_scan",
  "scoring",
  "finalize",
] as const;

export interface StageStep {
  stage: string;
  label: string;
  status: string;
  pct?: number;
}

export function buildStageSteps(
  stageStatus?: Record<string, string>,
  progress?: number,
  projectStatus?: string,
): StageStep[] {
  const steps: StageStep[] = [];
  let currentSet = false;
  for (const stage of STAGE_ORDER) {
    const status = stageStatus?.[stage] || STAGE_STATUS.PENDING;
    steps.push({ stage, label: STAGE_LABELS[stage] || stage, status });
    if (!currentSet && status === STAGE_STATUS.RUNNING) {
      steps[steps.length - 1].pct = progress ?? 0;
      currentSet = true;
    }
  }
  if (!currentSet && projectStatus === "running") {
    const next = steps.find((s) => s.status === STAGE_STATUS.PENDING);
    if (next) {
      next.status = STAGE_STATUS.RUNNING;
      next.pct = progress ?? 0;
    }
  }
  return steps;
}

/** 可重试阶段：优先后端下发的 retryable_stage，其次回退到首个 failed 阶段 */
export function getRetryableStage(
  project?: ReviewProject | null,
): string | null {
  if (!project) return null;
  if (project.retryable_stage) return project.retryable_stage;
  for (const stage of STAGE_ORDER) {
    if (project.stage_status?.[stage] === STAGE_STATUS.FAILED) return stage;
  }
  return null;
}

// ===== 维度 =====

export const DIMENSION_META: Record<
  string,
  { label: string; short: string; color: string; bg: string; tone: string }
> = {
  compliance: {
    label: "合规性",
    short: "合规",
    color: "error.600",
    bg: "error.50",
    tone: "error",
  },
  completeness: {
    label: "完整性",
    short: "完整",
    color: "info.600",
    bg: "info.50",
    tone: "info",
  },
  competitiveness: {
    label: "竞争力",
    short: "竞争力",
    color: "gold.700",
    bg: "gold.50",
    tone: "gold",
  },
  format: {
    label: "暗标版式",
    short: "暗标",
    color: "purple.600",
    bg: "purple.50",
    tone: "purple",
  },
};

export const DIMENSION_ORDER = [
  "compliance",
  "completeness",
  "competitiveness",
  "format",
] as const;

// ===== 结论 =====

export const FINDING_META: Record<
  string,
  { label: string; color: string; bg: string }
> = {
  pass: { label: "通过", color: "success.600", bg: "success.50" },
  warning: { label: "需整改", color: "warning.600", bg: "warning.50" },
  error: { label: "高风险", color: "error.600", bg: "error.50" },
  na: { label: "不适用", color: "neutral.500", bg: "neutral.100" },
  not_found: { label: "未找到依据", color: "orange.600", bg: "orange.50" },
};

export const SEVERITY_META: Record<
  string,
  { label: string; color: string; rank: number }
> = {
  high: { label: "高风险", color: "error.500", rank: 3 },
  medium: { label: "中风险", color: "warning.500", rank: 2 },
  low: { label: "低风险", color: "neutral.400", rank: 1 },
};

export const SOURCE_LABELS: Record<string, string> = {
  preset: "通用清单",
  analysis: "解析依据",
  rule: "企业规则",
  user: "自定义",
  format: "暗标版式",
};

export const REVIEW_STATUS_META: Record<
  string,
  { label: string; color: string; bg: string }
> = {
  pending: { label: "待处理", color: "neutral.500", bg: "neutral.100" },
  confirmed: { label: "已确认", color: "success.600", bg: "success.50" },
  rejected: { label: "已驳回", color: "error.600", bg: "error.50" },
};

export const REMEDIATION_META: Record<
  string,
  { label: string; color: string; bg: string }
> = {
  todo: { label: "待整改", color: "error.600", bg: "error.50" },
  doing: { label: "整改中", color: "warning.600", bg: "warning.50" },
  done: { label: "已完成", color: "success.600", bg: "success.50" },
  ignored: { label: "已忽略", color: "neutral.500", bg: "neutral.100" },
};

export function findingMeta(status?: string) {
  if (!status)
    return { label: "待判定", color: "neutral.500", bg: "neutral.100" };
  return (
    FINDING_META[status] || {
      label: status,
      color: "neutral.500",
      bg: "neutral.100",
    }
  );
}

/** 结论是否属于需要通过/整改处理的告警项 */
export function isAlertFinding(status?: string): boolean {
  return status === "error" || status === "warning" || status === "not_found";
}
