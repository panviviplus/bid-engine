package entity

// ===== 请求 =====

type PageListBidAnalysisProjectReq struct {
	Name      string `json:"name" form:"name"`
	Status    string `json:"status" form:"status"`
	StartTime int64  `json:"startTime" form:"startTime"`
	EndTime   int64  `json:"endTime" form:"endTime"`
	PageSize  int64  `json:"pageSize" form:"pageSize"`
	PageNum   int64  `json:"pageNum" form:"pageNum"`
}

type UpdateCoreFieldReq struct {
	ID         int64  `json:"id"`
	FieldValue string `json:"fieldValue"`
}

type UpdateClauseReq struct {
	ID            int64  `json:"id"`
	ClauseTitle   string `json:"clauseTitle"`
	ClauseContent string `json:"clauseContent"`
	Importance    string `json:"importance"`
	Status        *bool  `json:"status"`
}

type AddUserClauseReq struct {
	ProjectID     int64  `json:"projectId"`
	ChapterID     int64  `json:"chapterId"`
	ClauseTitle   string `json:"clauseTitle"`
	ClauseContent string `json:"clauseContent"`
	SourceText    string `json:"sourceText"`
	SourcePageNo  int32  `json:"sourcePageNo"`
}

// ===== 响应 =====

type BidAnalysisProjectItemResp struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FileName    string `json:"fileName"`
	Status      string `json:"status"`
	Progress    int32  `json:"progress"`
	Stage       string `json:"stage"`
	CreatedTime int64  `json:"createdTime"`
	StageStatus map[string]string `json:"stageStatus,omitempty"`
	UpdatedTime int64  `json:"updatedTime"`
}

type BidAnalysisProjectDetailResp struct {
	ID             int64                       `json:"id"`
	Name           string                      `json:"name"`
	FileName       string                      `json:"fileName"`
	FileObject     string                      `json:"fileObject"`
	Status         string                      `json:"status"`
	Progress       int32                       `json:"progress"`
	Stage          string                      `json:"stage"`
	IsFinalConfirm bool                        `json:"isFinalConfirm"`
	Chapters       []BidAnalysisChapterResp    `json:"chapters"`
	CoreFields     []BidAnalysisCoreFieldResp  `json:"coreFields"`
	StageStatus    map[string]string            `json:"stageStatus,omitempty"`
	Blueprint      []BidAnalysisBlueprintResp  `json:"blueprint"`
}

type BidAnalysisChapterResp struct {
	ID             int64                   `json:"id"`
	ChapterType    string                  `json:"chapterType"`
	ChapterTitle   string                  `json:"chapterTitle"`
	SortOrder      int32                   `json:"sortOrder"`
	PageRangeStart int32                   `json:"pageRangeStart"`
	PageRangeEnd   int32                   `json:"pageRangeEnd"`
	Summary        string                  `json:"summary"`
	Status         bool                    `json:"status"`
	Clauses        []BidAnalysisClauseResp `json:"clauses"`
}

type BidAnalysisClauseResp struct {
	ID            int64  `json:"id"`
	ChapterID     int64  `json:"chapterId"`
	ClauseTitle   string `json:"clauseTitle"`
	ClauseContent string `json:"clauseContent"`
	SourceText    string `json:"sourceText"`
	SourcePageNo  int32  `json:"sourcePageNo"`
	Importance    string `json:"importance"`
	IsUserAdded   bool   `json:"isUserAdded"`
	Status        bool   `json:"status"`
}

type BidAnalysisCoreFieldResp struct {
	ID           int64  `json:"id"`
	FieldName    string `json:"fieldName"`
	FieldValue   string `json:"fieldValue"`
	SourceText   string `json:"sourceText"`
	Confidence   string `json:"confidence"`
	IsUserEdited bool   `json:"isUserEdited"`
	ValueType    string `json:"valueType"`
	IsUserAdded  bool   `json:"isUserAdded"`
	Remark       string `json:"remark"`
}

// ===== 用户自定义字段 =====

type AddUserFieldReq struct {
	ProjectID   int64  `json:"projectId"`
	FieldName   string `json:"fieldName"`
	Remark      string `json:"remark"`
	FieldValue  string `json:"fieldValue"`
	SourceText  string `json:"sourceText"`
	SourcePageNo int32 `json:"sourcePageNo"`
}

type UpdateUserFieldReq struct {
	ID         int64  `json:"id"`
	FieldValue string `json:"fieldValue"`
	Remark     string `json:"remark"`
}

type ExtractFieldReq struct {
	ID int64 `json:"id"`
}

// BidAnalysisCreateReq 创建V2项目请求
type BidAnalysisCreateReq struct {
	// 文件通过 FormData 上传，无需结构体字段
}

// Stage操作请求
type StageOperationReq struct {
	Stage string `json:"stage" binding:"required"`
}

// BatchDeleteBidAnalysisProjectsReq 批量删除V2项目请求
type BatchDeleteBidAnalysisProjectsReq struct {
	IDs []int64 `json:"ids"`
}

// ===== 标书蓝图 =====

type BidAnalysisBlueprintResp struct {
	ID             int64  `json:"id"`
	ProjectID      int64  `json:"projectId"`
	ParentID       int64  `json:"parentId"`
	Level          int32  `json:"level"`
	SortOrder      int32  `json:"sortOrder"`
	Title          string `json:"title"`
	ClauseIds      string `json:"clauseIds"`
	MaterialIds    string `json:"materialIds"`
	IsRequiredFile bool   `json:"isRequiredFile"`
	IsUserAdded    bool   `json:"isUserAdded"`
	IsAiSuggested  bool   `json:"isAiSuggested"`
}

type UpdateBlueprintNodeReq struct {
	ID             int64   `json:"id"`
	Title          *string `json:"title"`
	IsRequiredFile *bool   `json:"isRequiredFile"`
	IsAiSuggested  *bool   `json:"isAiSuggested"`
}

type AddBlueprintNodeReq struct {
	ProjectID int64  `json:"projectId"`
	ParentID  int64  `json:"parentId"`
	Level     int32  `json:"level"`
	Title     string `json:"title"`
}
