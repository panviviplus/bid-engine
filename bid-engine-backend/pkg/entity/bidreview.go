package entity

// ===== 投标书审核模块（bid-audit V2）请求结构 =====

// BidReviewListReq 审核项目列表
type BidReviewListReq struct {
	Keyword  string `json:"keyword" form:"keyword"`
	Name     string `json:"name" form:"name"` // 兼容 useStatusTotals 钩子传参
	Status   string `json:"status" form:"status"`
	PageNum  int    `json:"pageNum" form:"pageNum"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}

// BidReviewItemAddReq 用户自定义检查项
type BidReviewItemAddReq struct {
	ProjectID        int64  `json:"project_id" binding:"required"`
	Dimension        string `json:"dimension"` // compliance/completeness/competitiveness/format
	Category         string `json:"category"`
	Title            string `json:"title" binding:"required"`
	Requirement      string `json:"requirement"`
	ExpectedEvidence string `json:"expected_evidence"`
	Severity         string `json:"severity"`
}

// BidReviewItemUpdateReq 清单项人工确认
type BidReviewItemUpdateReq struct {
	ItemID       int64  `json:"item_id" binding:"required"`
	ReviewStatus string `json:"review_status" binding:"required"` // pending/confirmed/rejected
	Note         string `json:"note"`
}

// BidReviewItemRecheckReq 复检（单项或按维度整批）
type BidReviewItemRecheckReq struct {
	ProjectID int64  `json:"project_id" binding:"required"`
	ItemID    int64  `json:"item_id"`
	Dimension string `json:"dimension"`
}

// BidReviewItemDeleteReq 删除清单项（用户自定义项）
type BidReviewItemDeleteReq struct {
	ItemID int64 `json:"item_id" binding:"required"`
}

// BidReviewStageReq 阶段重试（stage 缺省时由服务端回退到可重试阶段）
type BidReviewStageReq struct {
	Stage string `json:"stage"`
}

// BidReviewRemediationUpdateReq 整改项更新
type BidReviewRemediationUpdateReq struct {
	ID          int64  `json:"id" binding:"required"`
	Status      string `json:"status"` // todo/doing/done/ignored
	Note        string `json:"note"`
	OwnerUserID int64  `json:"owner_user_id"`
}

// BidReviewRuleSaveReq 企业审核规则保存
type BidReviewRuleSaveReq struct {
	ID               int64  `json:"id"`
	Dimension        string `json:"dimension"`
	Category         string `json:"category"`
	Title            string `json:"title" binding:"required"`
	Requirement      string `json:"requirement"`
	ExpectedEvidence string `json:"expected_evidence"`
	Severity         string `json:"severity"`
	AppliesWhen      string `json:"applies_when"`
	Enabled          *bool  `json:"enabled"`
}

// BidReviewRuleDeleteReq 企业审核规则删除
type BidReviewRuleDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// BidReviewRuleFromItemReq 清单项沉淀为规则
type BidReviewRuleFromItemReq struct {
	ItemID int64 `json:"item_id" binding:"required"`
}
