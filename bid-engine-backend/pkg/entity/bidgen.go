package entity

// ===== 标书生成 · 请求 =====

// BidGenCreateBlankReq 创建空白标书
type BidGenCreateBlankReq struct {
	Name string `json:"name" binding:"required"`
}

// BidGenListReq 项目列表
type BidGenListReq struct {
	Name     string `json:"name" form:"name"`
	Status   string `json:"status" form:"status"`
	PageSize int64  `json:"pageSize" form:"pageSize"`
	PageNum  int64  `json:"pageNum" form:"pageNum"`
}

// BidGenSaveDocReq 自动保存文档内容
type BidGenSaveDocReq struct {
	ProjectID int64  `json:"projectId" binding:"required"`
	DocJSON   string `json:"docJson"`
	DocHTML   string `json:"docHtml"`
}

// BidGenConfirmOutlineReq 大纲确认
type BidGenConfirmOutlineReq struct {
	ProjectID int64 `json:"projectId" binding:"required"`
}

// BidGenUnconfirmOutlineReq 撤销大纲确认（draft → outline_review，仅未生成章节时）
type BidGenUnconfirmOutlineReq struct {
	ProjectID int64 `json:"projectId" binding:"required"`
}

// BidGenOutlineNodeReq 大纲节点增删改
type BidGenAddOutlineReq struct {
	ProjectID int64  `json:"projectId" binding:"required"`
	ParentID  int64  `json:"parentId"`
	Level     int32  `json:"level"`
	Title     string `json:"title" binding:"required"`
	// SortAfter 插入到指定同级节点之后（0=追加到末尾）
	SortAfter int64 `json:"sortAfter"`
}

type BidGenUpdateOutlineReq struct {
	ID    int64  `json:"id" binding:"required"`
	Title string `json:"title"`
}

// BidGenCompleteOutlineReq 人工标记章节写作完成/取消完成
// （人工撰写的章节不经过 AI 生成流程，gen_status 一直停留在 pending，需要手工置位）
type BidGenCompleteOutlineReq struct {
	ProjectID int64 `json:"projectId" binding:"required"`
	ID        int64 `json:"id" binding:"required"`
	// Completed 缺省按 true 处理：true-设为已完成，false-取消已完成
	Completed *bool `json:"completed"`
}

type BidGenDeleteOutlineReq struct {
	ID        int64 `json:"id" binding:"required"`
	ProjectID int64 `json:"projectId" binding:"required"`
}

// BidGenApplyOutlineNode 大纲结构快照节点（apply 用）：仅更新排序/层级
type BidGenApplyOutlineNode struct {
	ID        int64 `json:"id" binding:"required"`
	ParentID  int64 `json:"parentId"`
	Level     int32 `json:"level"`
	SortOrder int32 `json:"sortOrder"`
}

// BidGenApplyOutlineReq 大纲结构快照（拖拽排序/跨级移动）：提交本项目全部大纲节点的权威结构
type BidGenApplyOutlineReq struct {
	ProjectID int64                    `json:"projectId" binding:"required"`
	Nodes     []BidGenApplyOutlineNode `json:"nodes"`
}

// BidGenGenerateReq 生成请求（整篇/章节共用）
type BidGenGenerateReq struct {
	ProjectID   int64   `json:"projectId" binding:"required"`
	OutlineIDs  []int64 `json:"outlineIds"`  // 指定章节生成；为空=整篇
	Length      string  `json:"length"`      // concise-精简 / standard-标准 / detailed-详细
	Mode        string  `json:"mode"`        // write-撰写 / rewrite-重写 / expand-扩写 / condense-缩写（默认 write）
	Instruction string  `json:"instruction"` // 可选补充要求（仅 rewrite/expand/condense 生效）
}

// BidGenCancelReq 取消生成
type BidGenCancelReq struct {
	ProjectID int64 `json:"projectId" binding:"required"`
	TaskID    int64 `json:"taskId"`
}

// BidGenRewriteReq AI 重写选中片段请求（终态文档审阅场景，与全文生成独立）
type BidGenRewriteReq struct {
	ProjectID   int64  `json:"projectId" binding:"required"`
	Text        string `json:"text" binding:"required"` // 选中的文本片段（可能非完整句子）
	Context     string `json:"context"`                 // 所在段落上下文（用于句子扩展/上下文连贯）
	Instruction string `json:"instruction"`             // 可选改写要求（如“更简洁”）
}

// BidGenExportPdfReq 导出 PDF（前端已生成 DOCX 上传）
type BidGenExportPdfReq struct {
	ProjectID int64  `json:"projectId"`
	FileName  string `json:"fileName"`
}

// BidGenExportRecordReq 记录导出历史（DOCX 由前端生成）
type BidGenExportRecordReq struct {
	ProjectID  int64  `json:"projectId" binding:"required"`
	ExportType string `json:"exportType" binding:"required"` // docx / pdf
	FileName   string `json:"fileName"`
	FileSize   int64  `json:"fileSize"`
}

// ===== 标书生成 · 响应 =====

// BidGenProjectItemResp 列表项
type BidGenProjectItemResp struct {
	ID              int64             `json:"id"`
	Name            string            `json:"name"`
	CreateType      string            `json:"createType"`
	Status          string            `json:"status"`
	Progress        int32             `json:"progress"`
	Stage           string            `json:"stage"`
	SourceFileName  string            `json:"sourceFileName"`
	StageStatus     map[string]string `json:"stageStatus,omitempty"`
	ReviewProjectID int64             `json:"reviewProjectId"`
	CreatedTime     int64             `json:"createdTime"`
	UpdatedTime     int64             `json:"updatedTime"`
	OutlineCount    int64             `json:"outlineCount"`
	SucceededCount  int64             `json:"succeededCount"`
}

// BidGenOutlineResp 大纲节点
type BidGenOutlineResp struct {
	ID             int64  `json:"id"`
	ProjectID      int64  `json:"projectId"`
	ParentID       int64  `json:"parentId"`
	Level          int32  `json:"level"`
	SortOrder      int32  `json:"sortOrder"`
	Title          string `json:"title"`
	ClauseIds      string `json:"clauseIds"`
	MaterialIds    string `json:"materialIds"`
	GenStatus      string `json:"genStatus"`
	Source         string `json:"source"`
	IsRequiredFile bool   `json:"isRequiredFile"`
	IsAiSuggested  bool   `json:"isAiSuggested"`
}

// BidGenProjectDetailResp 详情
type BidGenProjectDetailResp struct {
	ID               int64               `json:"id"`
	Name             string              `json:"name"`
	CreateType       string              `json:"createType"`
	Status           string              `json:"status"`
	Progress         int32               `json:"progress"`
	Stage            string              `json:"stage"`
	StageStatus      map[string]string   `json:"stageStatus,omitempty"`
	ReviewProjectID  int64               `json:"reviewProjectId"`
	SourceFileName   string              `json:"sourceFileName"`
	SourceFileURL    string              `json:"sourceFileUrl"`
	SourceFileObject string              `json:"sourceFileObject"`
	DocJSON          string              `json:"docJson"`
	DocHTML          string              `json:"docHtml"`
	Outline          []BidGenOutlineResp `json:"outline"`
	RunningTask      *BidGenTaskResp     `json:"runningTask,omitempty"`
	Cover            *BidGenCoverResp    `json:"cover,omitempty"`
	LastError        string              `json:"lastError"`
	CreatedTime      int64               `json:"createdTime"`
	UpdatedTime      int64               `json:"updatedTime"`
}

// BidGenCoverResp 封面页数据（导出时装配，前端零拼装）
type BidGenCoverResp struct {
	DocTitle      string `json:"docTitle"`
	ProjectName   string `json:"projectName"`
	ProjectNumber string `json:"projectNumber"`
	LotLabel      string `json:"lotLabel"`
	TendererName  string `json:"tendererName"`
	BidderName    string `json:"bidderName"`
	Date          string `json:"date"`
}

// BidGenTaskResp 生成任务
type BidGenTaskResp struct {
	ID               int64  `json:"id"`
	ProjectID        int64  `json:"projectId"`
	TaskType         string `json:"taskType"`
	Status           string `json:"status"`
	Progress         int32  `json:"progress"`
	CurrentOutlineID int64  `json:"currentOutlineId"`
	CompletedCount   int32  `json:"completedCount"`
	TotalCount       int32  `json:"totalCount"`
	ErrorMsg         string `json:"errorMsg"`
	LengthTier       string `json:"lengthTier"`
	GenMode          string `json:"genMode"`
	AttemptCount     int32  `json:"attemptCount"`
}

// BidGenMaterialRefResp 素材引用
type BidGenMaterialRefResp struct {
	ID           int64  `json:"id"`
	OutlineID    int64  `json:"outlineId"`
	MaterialType string `json:"materialType"`
	MaterialID   int64  `json:"materialId"`
	FileID       int64  `json:"fileId"`
	FileURL      string `json:"fileUrl"`
	Inserted     bool   `json:"inserted"`
}

// BidGenSyncOutlineReq 大纲同步请求（编辑器文档标题结构 → 大纲表全量对账）
type BidGenSyncOutlineReq struct {
	ProjectID int64               `json:"projectId" binding:"required"`
	Headings  []BidGenSyncHeading `json:"headings"`
}

// BidGenSyncHeading 文档中一个标题节点（按文档顺序）
type BidGenSyncHeading struct {
	OutlineID int64  `json:"outlineId"` // 已存在大纲节点ID；0=新增标题
	Level     int32  `json:"level"`
	Title     string `json:"title"`
}

// BidGenOutlineSyncItem 对账中间产物：parent 以数组下标引用（避免依赖未落库的新 id）
type BidGenOutlineSyncItem struct {
	Index       int64  `json:"index"`
	OutlineID   int64  `json:"outlineId"`
	ParentIndex int64  `json:"parentIndex"` // -1=顶层
	Level       int32  `json:"level"`
	Title       string `json:"title"`
	SortOrder   int32  `json:"sortOrder"`
}

// BatchDeleteBidGenProjectsReq 批量删除标书生成项目请求
type BatchDeleteBidGenProjectsReq struct {
	IDs []int64 `json:"ids"`
}
