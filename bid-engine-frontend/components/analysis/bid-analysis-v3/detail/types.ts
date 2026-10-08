import type {
  BlueprintStatus,
  V3BlueprintData,
  V3BlueprintNode,
} from "@/service/bid-analysis";

/* eslint-disable no-unused-vars */

export type DetailTabKey = "overview" | "fields" | "clauses" | "blueprint";

export type Evidence = {
  id: number;
  source_kind: string;
  block_id: number;
  source_table_id: number;
  page_no: number;
  quote: string;
  bbox_left: number;
  bbox_top: number;
  bbox_width: number;
  bbox_height: number;
};

export type DerivedTable = {
  id: number;
  title: string;
  row_count: number;
  column_count: number;
  data: unknown;
  source_count: number;
  evidences?: unknown[];
};

export type FieldValue = {
  id: number;
  display_value: string;
  origin: string;
  status: string;
  confidence: string;
  is_user_edited: boolean;
  needs_evidence: boolean;
  evidence_count: number;
  evidences: Evidence[];
  derived_tables?: DerivedTable[];
};

export type Field = {
  id: number;
  field_key: string;
  display_name: string;
  origin: string;
  value_type: string;
  extract_status: string;
  ai_interpretation?: string;
  values: FieldValue[];
};

export type Category = {
  key: string;
  name: string;
  description: string;
  fields: Field[];
};

export type Clause = {
  id: number;
  title: string;
  content: string;
  importance: string;
  status: string;
  ai_interpretation?: string;
  evidences: Evidence[];
};

export type Chapter = {
  id: number;
  title: string;
  type: string;
  page_start: number;
  page_end: number;
  clauses: Clause[];
};

export type SummaryRiskItem = {
  index: number;
  text: string;
  kind?: "risk" | "confirm" | "unknown";
  label?: string;
  resolved: boolean;
};

export type Summary = {
  overview?: string;
  key_points?: string[];
  risks?: SummaryRiskItem[];
};

export type FollowItem = {
  id: number;
  target_type: "field" | "clause";
  target_id: number;
  title: string;
  content: string;
  remark: string;
};

export type WarningChapterRef = {
  id: number;
  title: string;
  page_start: number;
  page_end: number;
};

export type WarningGroup = {
  id?: number;
  group_key: string;
  code: string;
  stage: string;
  severity: "critical" | "warning" | "info" | string;
  count: number;
  occurrences: number;
  sample_message: string;
  affected_chapters: WarningChapterRef[];
  reasons?: Record<string, number>;
  retry_target_stage: string;
};

export type SourceRegion = {
  page_no: number;
  left: number;
  top: number;
  width: number;
  height: number;
};

export type SourceTable = {
  id: number;
  caption: string;
  page_start: number;
  page_end: number;
  row_count: number;
  column_count: number;
  regions?: SourceRegion[];
  bbox_left: number;
  bbox_top: number;
  bbox_width: number;
  bbox_height: number;
};

export type BlueprintViewStatus = BlueprintStatus | "load_failed";

export type BlueprintPanelProps = {
  projectId: string;
  data: V3BlueprintData | null;
  loading: boolean;
  loadError: string;
  mutate: (config: any) => Promise<any>;
  mutating: boolean;
  reload: () => Promise<void>;
  onOpenClause: (id: number) => void;
  onOpenEvidence: (ids: number[]) => void;
};

export type { V3BlueprintData, V3BlueprintNode };
