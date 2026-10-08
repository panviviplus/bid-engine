package model

// ================================================================
// 标书审核 V2 数据模型（手写维护；新表不走 gorm.io/gen 生成）
// 约定：表间不建外键，关联与一致性由应用层维护（与库内既有表保持一致）
// ================================================================

import "time"

// BidReviewV2Project 审核项目
type BidReviewV2Project struct {
	ID                    int64      `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	Name                  string     `gorm:"column:name;not null" json:"name"`
	CreateType            string     `gorm:"column:create_type;not null;default:manual" json:"create_type"`
	Status                string     `gorm:"column:status;not null;default:running" json:"status"`
	Stage                 string     `gorm:"column:stage;not null" json:"stage"`
	Progress              int32      `gorm:"column:progress;not null" json:"progress"`
	RunCount              int32      `gorm:"column:run_count;not null" json:"run_count"`
	StageStatus           string     `gorm:"column:stage_status" json:"stage_status"`
	LastError             string     `gorm:"column:last_error" json:"last_error"`
	StartedAt             *time.Time `gorm:"column:started_at" json:"started_at"`
	FinishedAt            *time.Time `gorm:"column:finished_at" json:"finished_at"`
	CancelledAt           *time.Time `gorm:"column:cancelled_at" json:"cancelled_at"`
	SourceBidGenProjectID int64      `gorm:"column:source_bid_gen_project_id;not null" json:"source_bid_gen_project_id"`
	AnalysisProjectID     int64      `gorm:"column:analysis_project_id;not null" json:"analysis_project_id"`
	AnalysisRunID         int64      `gorm:"column:analysis_run_id;not null" json:"analysis_run_id"`
	IsAnonymous           bool       `gorm:"column:is_anonymous;not null" json:"is_anonymous"`
	TenderFileCount       int32      `gorm:"column:tender_file_count;not null" json:"tender_file_count"`
	BidFileCount          int32      `gorm:"column:bid_file_count;not null" json:"bid_file_count"`
	TotalItems            int32      `gorm:"column:total_items;not null" json:"total_items"`
	PassedItems           int32      `gorm:"column:passed_items;not null" json:"passed_items"`
	WarningItems          int32      `gorm:"column:warning_items;not null" json:"warning_items"`
	ErrorItems            int32      `gorm:"column:error_items;not null" json:"error_items"`
	TodoItems             int32      `gorm:"column:todo_items;not null" json:"todo_items"`
	ScoringTotal          float64    `gorm:"column:scoring_total;not null" json:"scoring_total"`
	ScoringMax            float64    `gorm:"column:scoring_max;not null" json:"scoring_max"`
	UserID                int64      `gorm:"column:user_id;not null" json:"user_id"`
	UserTeamID            int32      `gorm:"column:user_team_id;not null" json:"user_team_id"`
	UserCompanyID         int32      `gorm:"column:user_company_id;not null" json:"user_company_id"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2Project) TableName() string { return "bid_review_v2_project" }

// BidReviewV2File 审核文件（招标/投标）
type BidReviewV2File struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID   int64     `gorm:"column:project_id;not null" json:"project_id"`
	FileType    string    `gorm:"column:file_type;not null;default:bid" json:"file_type"`
	FileRole    string    `gorm:"column:file_role;not null;default:main" json:"file_role"`
	FileName    string    `gorm:"column:file_name;not null" json:"file_name"`
	FileBucket  string    `gorm:"column:file_bucket;not null" json:"file_bucket"`
	FileObject  string    `gorm:"column:file_object;not null" json:"file_object"`
	FileURL     string    `gorm:"column:file_url;not null" json:"file_url"`
	PdfObject   string    `gorm:"column:pdf_object;not null" json:"pdf_object"`
	PdfURL      string    `gorm:"column:pdf_url;not null" json:"pdf_url"`
	SHA256      string    `gorm:"column:sha256;not null" json:"sha256"`
	PageCount   int32     `gorm:"column:page_count;not null" json:"page_count"`
	CharCount   int32     `gorm:"column:char_count;not null" json:"char_count"`
	SortOrder   int32     `gorm:"column:sort_order;not null" json:"sort_order"`
	ParseStatus string    `gorm:"column:parse_status;not null;default:pending" json:"parse_status"`
	ParseError  string    `gorm:"column:parse_error" json:"parse_error"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2File) TableName() string { return "bid_review_v2_file" }

// BidReviewV2DocumentPage 逐页文本
type BidReviewV2DocumentPage struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID int64     `gorm:"column:project_id;not null" json:"project_id"`
	FileID    int64     `gorm:"column:file_id;not null" json:"file_id"`
	PageNo    int32     `gorm:"column:page_no;not null" json:"page_no"`
	Content   string    `gorm:"column:content" json:"content"`
	CharCount int32     `gorm:"column:char_count;not null" json:"char_count"`
	Width     float64   `gorm:"column:width;not null" json:"width"`
	Height    float64   `gorm:"column:height;not null" json:"height"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2DocumentPage) TableName() string { return "bid_review_v2_document_page" }

// BidReviewV2DocumentBlock 页块（含坐标，供证据高亮）
type BidReviewV2DocumentBlock struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID  int64     `gorm:"column:project_id;not null" json:"project_id"`
	FileID     int64     `gorm:"column:file_id;not null" json:"file_id"`
	PageNo     int32     `gorm:"column:page_no;not null" json:"page_no"`
	BlockType  string    `gorm:"column:block_type;not null;default:text" json:"block_type"`
	Content    string    `gorm:"column:content" json:"content"`
	BBoxLeft   float64   `gorm:"column:bbox_left;not null" json:"bbox_left"`
	BBoxTop    float64   `gorm:"column:bbox_top;not null" json:"bbox_top"`
	BBoxWidth  float64   `gorm:"column:bbox_width;not null" json:"bbox_width"`
	BBoxHeight float64   `gorm:"column:bbox_height;not null" json:"bbox_height"`
	TableJSON  string    `gorm:"column:table_json" json:"table_json"`
	SortOrder  int32     `gorm:"column:sort_order;not null" json:"sort_order"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2DocumentBlock) TableName() string { return "bid_review_v2_document_block" }

// BidReviewV2DocumentChunk 文档分块（召回单元）
type BidReviewV2DocumentChunk struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID int64     `gorm:"column:project_id;not null" json:"project_id"`
	FileID    int64     `gorm:"column:file_id;not null" json:"file_id"`
	ChunkNo   int32     `gorm:"column:chunk_no;not null" json:"chunk_no"`
	PageStart int32     `gorm:"column:page_start;not null" json:"page_start"`
	PageEnd   int32     `gorm:"column:page_end;not null" json:"page_end"`
	Content   string    `gorm:"column:content" json:"content"`
	CharCount int32     `gorm:"column:char_count;not null" json:"char_count"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2DocumentChunk) TableName() string { return "bid_review_v2_document_chunk" }

// BidReviewV2TermIndex 术语倒排索引
type BidReviewV2TermIndex struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID int64     `gorm:"column:project_id;not null" json:"project_id"`
	FileID    int64     `gorm:"column:file_id;not null" json:"file_id"`
	ChunkID   int64     `gorm:"column:chunk_id;not null" json:"chunk_id"`
	Term      string    `gorm:"column:term;not null" json:"term"`
	PageNo    int32     `gorm:"column:page_no;not null" json:"page_no"`
	Weight    int32     `gorm:"column:weight;not null;default:1" json:"weight"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2TermIndex) TableName() string { return "bid_review_v2_term_index" }

// BidReviewV2AnalysisSnapshot 招标解析快照（审核依据冻结）
type BidReviewV2AnalysisSnapshot struct {
	ID                int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID         int64     `gorm:"column:project_id;not null" json:"project_id"`
	AnalysisProjectID int64     `gorm:"column:analysis_project_id;not null" json:"analysis_project_id"`
	AnalysisRunID     int64     `gorm:"column:analysis_run_id;not null" json:"analysis_run_id"`
	SnapshotJSON      string    `gorm:"column:snapshot_json" json:"snapshot_json"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2AnalysisSnapshot) TableName() string { return "bid_review_v2_analysis_snapshot" }

// BidReviewV2ChecklistItem 审核清单项
type BidReviewV2ChecklistItem struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID        int64      `gorm:"column:project_id;not null;uniqueIndex:uniq_project_key,priority:1" json:"project_id"`
	ItemKey          string     `gorm:"column:item_key;not null;uniqueIndex:uniq_project_key,priority:2" json:"item_key"`
	Dimension        string     `gorm:"column:dimension;not null;default:compliance" json:"dimension"`
	Category         string     `gorm:"column:category;not null" json:"category"`
	Title            string     `gorm:"column:title;not null" json:"title"`
	Requirement      string     `gorm:"column:requirement" json:"requirement"`
	ExpectedEvidence string     `gorm:"column:expected_evidence" json:"expected_evidence"`
	Severity         string     `gorm:"column:severity;not null;default:medium" json:"severity"`
	Source           string     `gorm:"column:source;not null;default:preset" json:"source"`
	RuleID           int64      `gorm:"column:rule_id;not null" json:"rule_id"`
	TenderPage       int32      `gorm:"column:tender_page;not null" json:"tender_page"`
	TenderQuote      string     `gorm:"column:tender_quote" json:"tender_quote"`
	OriginJSON       string     `gorm:"column:origin_json" json:"origin_json"`
	ReviewStatus     string     `gorm:"column:review_status;not null;default:pending" json:"review_status"`
	ReviewNote       string     `gorm:"column:review_note" json:"review_note"`
	ReviewedBy       int64      `gorm:"column:reviewed_by;not null" json:"reviewed_by"`
	ReviewedAt       *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	SortOrder        int32      `gorm:"column:sort_order;not null" json:"sort_order"`
	IsUserEdited     bool       `gorm:"column:is_user_edited;not null" json:"is_user_edited"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2ChecklistItem) TableName() string { return "bid_review_v2_checklist_item" }

// BidReviewV2Finding 判定结论（保留历史，is_latest 标记最新）
type BidReviewV2Finding struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID       int64     `gorm:"column:project_id;not null" json:"project_id"`
	ChecklistItemID int64     `gorm:"column:checklist_item_id;not null" json:"checklist_item_id"`
	Status          string    `gorm:"column:status;not null" json:"status"`
	Severity        string    `gorm:"column:severity;not null" json:"severity"`
	Reason          string    `gorm:"column:reason" json:"reason"`
	Suggestion      string    `gorm:"column:suggestion" json:"suggestion"`
	Confidence      string    `gorm:"column:confidence;not null;default:medium" json:"confidence"`
	Engine          string    `gorm:"column:engine;not null;default:llm" json:"engine"`
	Model           string    `gorm:"column:model;not null" json:"model"`
	LatencyMS       int64     `gorm:"column:latency_ms;not null" json:"latency_ms"`
	IsLatest        bool      `gorm:"column:is_latest;not null;default:1" json:"is_latest"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2Finding) TableName() string { return "bid_review_v2_finding" }

// BidReviewV2Evidence 证据
type BidReviewV2Evidence struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID       int64     `gorm:"column:project_id;not null" json:"project_id"`
	ChecklistItemID int64     `gorm:"column:checklist_item_id;not null" json:"checklist_item_id"`
	FindingID       int64     `gorm:"column:finding_id;not null" json:"finding_id"`
	Side            string    `gorm:"column:side;not null;default:bid" json:"side"`
	FileID          int64     `gorm:"column:file_id;not null" json:"file_id"`
	FileName        string    `gorm:"column:file_name;not null" json:"file_name"`
	PageNo          int32     `gorm:"column:page_no;not null" json:"page_no"`
	Quote           string    `gorm:"column:quote" json:"quote"`
	BBoxLeft        float64   `gorm:"column:bbox_left;not null" json:"bbox_left"`
	BBoxTop         float64   `gorm:"column:bbox_top;not null" json:"bbox_top"`
	BBoxWidth       float64   `gorm:"column:bbox_width;not null" json:"bbox_width"`
	BBoxHeight      float64   `gorm:"column:bbox_height;not null" json:"bbox_height"`
	ChunkID         int64     `gorm:"column:chunk_id;not null" json:"chunk_id"`
	MatchScore      float64   `gorm:"column:match_score;not null" json:"match_score"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2Evidence) TableName() string { return "bid_review_v2_evidence" }

// BidReviewV2Remediation 整改项
type BidReviewV2Remediation struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID       int64      `gorm:"column:project_id;not null" json:"project_id"`
	ChecklistItemID int64      `gorm:"column:checklist_item_id;not null" json:"checklist_item_id"`
	FindingID       int64      `gorm:"column:finding_id;not null" json:"finding_id"`
	Dimension       string     `gorm:"column:dimension;not null" json:"dimension"`
	Title           string     `gorm:"column:title;not null" json:"title"`
	Suggestion      string     `gorm:"column:suggestion" json:"suggestion"`
	Severity        string     `gorm:"column:severity;not null" json:"severity"`
	Status          string     `gorm:"column:status;not null;default:todo" json:"status"`
	OwnerUserID     int64      `gorm:"column:owner_user_id;not null" json:"owner_user_id"`
	Note            string     `gorm:"column:note" json:"note"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at" json:"resolved_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2Remediation) TableName() string { return "bid_review_v2_remediation" }

// BidReviewV2Rule 企业审核规则
type BidReviewV2Rule struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	UserID           int64     `gorm:"column:user_id;not null" json:"user_id"`
	UserTeamID       int32     `gorm:"column:user_team_id;not null" json:"user_team_id"`
	UserCompanyID    int32     `gorm:"column:user_company_id;not null" json:"user_company_id"`
	Dimension        string    `gorm:"column:dimension;not null;default:compliance" json:"dimension"`
	Category         string    `gorm:"column:category;not null" json:"category"`
	Title            string    `gorm:"column:title;not null" json:"title"`
	Requirement      string    `gorm:"column:requirement" json:"requirement"`
	ExpectedEvidence string    `gorm:"column:expected_evidence" json:"expected_evidence"`
	Severity         string    `gorm:"column:severity;not null;default:medium" json:"severity"`
	AppliesWhen      string    `gorm:"column:applies_when" json:"applies_when"`
	Source           string    `gorm:"column:source;not null;default:user" json:"source"`
	Enabled          bool      `gorm:"column:enabled;not null;default:1" json:"enabled"`
	HitCount         int32     `gorm:"column:hit_count;not null" json:"hit_count"`
	Version          int32     `gorm:"column:version;not null;default:1" json:"version"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2Rule) TableName() string { return "bid_review_v2_rule" }

// BidReviewV2StageRun 阶段运行记录
type BidReviewV2StageRun struct {
	ID         int64      `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID  int64      `gorm:"column:project_id;not null" json:"project_id"`
	Stage      string     `gorm:"column:stage;not null" json:"stage"`
	Status     string     `gorm:"column:status;not null;default:pending" json:"status"`
	Attempts   int32      `gorm:"column:attempts;not null" json:"attempts"`
	Total      int32      `gorm:"column:total;not null" json:"total"`
	Completed  int32      `gorm:"column:completed;not null" json:"completed"`
	Failed     int32      `gorm:"column:failed;not null" json:"failed"`
	Progress   int32      `gorm:"column:progress;not null" json:"progress"`
	LastError  string     `gorm:"column:last_error" json:"last_error"`
	DetailJSON string     `gorm:"column:detail_json" json:"detail_json"`
	StartedAt  *time.Time `gorm:"column:started_at" json:"started_at"`
	FinishedAt *time.Time `gorm:"column:finished_at" json:"finished_at"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*BidReviewV2StageRun) TableName() string { return "bid_review_v2_stage_run" }

// BidReviewV2ExportRecord 导出记录
type BidReviewV2ExportRecord struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID  int64     `gorm:"column:project_id;not null" json:"project_id"`
	Kind       string    `gorm:"column:kind;not null;default:report" json:"kind"`
	FileName   string    `gorm:"column:file_name;not null" json:"file_name"`
	FileBucket string    `gorm:"column:file_bucket;not null" json:"file_bucket"`
	FileObject string    `gorm:"column:file_object;not null" json:"file_object"`
	FileURL    string    `gorm:"column:file_url;not null" json:"file_url"`
	UserID     int64     `gorm:"column:user_id;not null" json:"user_id"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2ExportRecord) TableName() string { return "bid_review_v2_export_record" }

// BidReviewV2OpLog 操作日志
type BidReviewV2OpLog struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement:true" json:"id"`
	ProjectID       int64     `gorm:"column:project_id;not null" json:"project_id"`
	ChecklistItemID int64     `gorm:"column:checklist_item_id;not null" json:"checklist_item_id"`
	Action          string    `gorm:"column:action;not null" json:"action"`
	OperatorID      int64     `gorm:"column:operator_id;not null" json:"operator_id"`
	Detail          string    `gorm:"column:detail" json:"detail"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (*BidReviewV2OpLog) TableName() string { return "bid_review_v2_op_log" }
