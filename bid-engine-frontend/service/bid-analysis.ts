import useAxios from "axios-hooks";

export type AnalysisStatus =
  | "running"
  | "paused"
  | "succeeded"
  | "succeeded_with_warnings"
  | "failed";

export type AnalysisStage =
  | "document_preprocessing"
  | "document_parsing"
  | "document_summary"
  | "chapter_identifying"
  | "chapter_fact_extracting"
  | "content_consolidating";

export interface V3StageRun {
  id: number;
  stage: AnalysisStage;
  status:
    | "pending"
    | "running"
    | "succeeded"
    | "partial"
    | "failed"
    | "skipped";
  total_units: number;
  completed_units: number;
  failed_units: number;
  attempts: number;
  last_error: string;
}

export interface V3RecoveryState {
  available: boolean;
  current_stage: AnalysisStage;
  current_stage_label: string;
  next_stage: AnalysisStage | "pipeline_complete";
  can_retry_current: boolean;
  can_retry_global: boolean;
  can_skip_current: boolean;
  skip_blocked_reason: string;
  last_error: string;
}

export type V3ControlAction = "pause" | "skip";
export type V3ControlMode = "after_stage" | "discard_current";
export type V3ControlStatus =
  | "requested"
  | "applying"
  | "applied"
  | "cancelled"
  | "failed";

export interface V3RunControl {
  id: number;
  project_id: number;
  run_id: number;
  action: V3ControlAction;
  mode: V3ControlMode;
  target_stage: AnalysisStage;
  resume_stage: AnalysisStage | "pipeline_complete";
  status: V3ControlStatus;
  last_error: string;
  requested_at: string;
  applied_at: string | null;
}

export interface V3ProjectListItem {
  id: number;
  run_id: number;
  name: string;
  source_file_name: string;
  status: AnalysisStatus;
  stage: string;
  progress: number;
  page_count: number;
  parsed_pages: number;
  field_count: number;
  evidence_count: number;
  table_count: number;
  warning_count: number;
  last_error: string;
  fact_subtask?: string;
  fact_subtask_label?: string;
  fact_subtask_progress?: {
    completed: number;
    total: number;
    percent: number;
  };
  control: V3RunControl | null;
  can_pause: boolean;
  can_resume: boolean;
  can_skip_current: boolean;
  skip_blocked_reason: string;
  created_at: string;
  updated_at: string;
}

export interface V3StatusCounts {
  all: number;
  running: number;
  paused: number;
  succeeded: number;
  succeeded_with_warnings: number;
  completed: number;
  failed: number;
}

export type BlueprintStatus =
  | "not_generated"
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "invalidated";

export interface V3BlueprintNode {
  id: number;
  project_id: number;
  run_id: number;
  blueprint_generation_id: number;
  parent_id: number;
  level: number;
  sort_order: number;
  title: string;
  clause_ids_json: string;
  evidence_ids_json: string;
  node_source:
    | "system_root"
    | "tender_required"
    | "tender_outline"
    | "ai_suggested"
    | "user_added";
  suggestion_status: "pending" | "accepted" | "removed";
  is_required_file: boolean;
  is_user_added: boolean;
  is_ai_suggested: boolean;
  is_locked: boolean;
}

export interface V3BlueprintData {
  status: BlueprintStatus;
  generation: null | {
    id: number;
    status: BlueprintStatus;
    node_count: number;
    warning_count: number;
    last_error: string;
    associated_bid_project_id: number;
    created_at: string;
    updated_at: string;
  };
  nodes: V3BlueprintNode[];
}

export const useAnalysisProjects = () => {
  const [{ data, loading, error }, execute] = useAxios(
    { url: "/zb/v3/projects", method: "GET" },
    { manual: true, useCache: false },
  );
  return { data: data?.data, loading, error, execute };
};

export const useCreateAnalysisProject = () => {
  const [{ loading, error }, execute] = useAxios(
    { url: "/zb/v3/projects", method: "POST" },
    { manual: true },
  );
  return { loading, error, execute };
};

export const useAnalysisProject = (projectId: string) => {
  const [{ data, loading, error }, execute] = useAxios(
    { url: `/zb/v3/projects/${projectId}`, method: "GET" },
    { manual: true, useCache: false },
  );
  return { data: data?.data, loading, error, execute };
};

export const useAnalysisProgress = (projectId: string) => {
  const [{ data, loading, error }, execute] = useAxios(
    { url: `/zb/v3/projects/${projectId}/progress`, method: "GET" },
    { manual: true, useCache: false },
  );
  return { data: data?.data, loading, error, execute };
};

export const useAnalysisMutation = () => {
  const [{ loading, error }, execute] = useAxios({}, { manual: true });
  return { loading, error, execute };
};
