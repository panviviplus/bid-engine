package entity

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// FeedbackPhotoKeySep 反馈图片对象键的分隔符（用于存储多个 key）
const FeedbackPhotoKeySep = "|"

// Options 接口无请求体，略

// SearchFeedbackRecordsReq 查询反馈记录请求
type SearchFeedbackRecordsReq struct {
	Keyword   string `form:"keyword" json:"keyword"`
	Type      string `form:"type" json:"type"`
	StartTime int64  `form:"startTime" json:"startTime"`
	EndTime   int64  `form:"endTime" json:"endTime"`
	PageSize  int    `form:"pageSize" json:"pageSize"`
	PageNum   int    `form:"pageNum" json:"pageNum"`
}

// Valid 参数合法性校验（GET 查询，兼容 query/form）
func (a *SearchFeedbackRecordsReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(a)
	_ = c.ShouldBind(a)
	if a.PageNum <= 0 {
		a.PageNum = 1
	}
	if a.PageSize <= 0 || a.PageSize > 50 {
		a.PageSize = 10
	}
	return nil
}

type SearchFeedbackRespItem struct {
	Id          int64  `json:"id"`
	UserId      int64  `json:"userId"`
	UserName    string `json:"userName"`
	Mobile      string `json:"mobile"`
	Type        string `json:"type"`
	Description string `json:"description" comment:"反馈描述"`
	Status      int32  `json:"status"`
	CreateTime  string `json:"createTime"`
	UpdateTime  string `json:"updateTime"`
}

// AddFeedbackRecordReq 新增反馈记录（POST，使用 form-data 以支持文件上传）
type AddFeedbackRecordReq struct {
	Type        string `form:"type" json:"type" comment:"反馈类型，枚举：suggest、question、bug、other"`
	Description string `form:"description" json:"description" comment:"反馈描述"`
	Status      *int32 `form:"status" json:"status" comment:"状态：1可用 0不可用，可选"`
}

// Valid 参数合法性校验（优先 form-data，其次 json）
func (a *AddFeedbackRecordReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(a)
	if strings.TrimSpace(a.Type) == "" || strings.TrimSpace(a.Description) == "" {
		// 兜底尝试 JSON 绑定
		_ = c.ShouldBindJSON(a)
	}
	if strings.TrimSpace(a.Type) == "" {
		return fmt.Errorf("反馈类型必填")
	}
	if strings.TrimSpace(a.Description) == "" {
		return fmt.Errorf("反馈描述必填")
	}
	return nil
}

// UpdateFeedbackRecordReq 仅允许编辑 description 字段
type UpdateFeedbackRecordReq struct {
	Description string `json:"description" form:"description" comment:"反馈描述（必填）"`
}

// Valid 参数合法性校验（POST/PUT，使用 JSON）
func (a *UpdateFeedbackRecordReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(a)
	if strings.TrimSpace(a.Description) == "" {
		return fmt.Errorf("反馈描述不能为空")
	}
	return nil
}

// DeleteFeedbackRecordReq 删除记录（Path 参数）
type DeleteFeedbackRecordReq struct {
	RecordID int64 `uri:"record_id" comment:"反馈记录ID"`
}

// Valid 参数合法性校验（URI 绑定）
func (a *DeleteFeedbackRecordReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(a)
	if a.RecordID <= 0 {
		return fmt.Errorf("record_id 参数错误")
	}
	return nil
}
