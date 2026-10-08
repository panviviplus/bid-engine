package entity

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// Response 通用返回结果
type Response struct {
	Code      ErrCode     `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data,omitempty"`
}

// UploadDocInfo 上传文档信息
type UploadDocInfo struct {
	DocType     DocType
	Filename    string
	Size        int64
	PageNum     int32
	FileContent []byte
	DocID       string
	Duration    int64
}

func (u UploadDocInfo) GetFileContent() []byte {
	return u.FileContent
}











// SubPassage 段落定义
type SubPassage struct {
	Title   string `json:"title"`
	Passage string `json:"passage"`
}

// ExportWReq 导出参数
type ExportWReq struct {
	Title    string    `json:"title"`
	Passages []Passage `json:"passages"`
}

// Passage 单个段落定义
type Passage struct {
	Title   string       `json:"title"`
	Content []SubPassage `json:"content"`
}

// Valid 参数合法性校验
func (e ExportWReq) Valid() error {
	if e.Title == "" {
		return fmt.Errorf("title 不能为空")
	}
	if len(e.Passages) == 0 {
		return fmt.Errorf("passages 不能为空")
	}
	return nil
}

// FeedbackReq 反馈接口
type FeedbackReq struct {
	ConversationID string `uri:"conversation_id"`
	LikeStatus     *int32 `json:"like_status"`
	Score          *int32 `json:"score"`
}

// Valid 参数合法性校验
func (f FeedbackReq) Valid() error {
	if f.ConversationID == "" {
		return fmt.Errorf("conversation_id 不能为空")
	}
	return nil
}

// GenWTemplateReq 生成模板
type GenWTemplateReq struct {
	DocumentIDs []string `json:"document_ids"`
}

// Valid 参数合法性校验
func (a *GenWTemplateReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if len(a.DocumentIDs) == 0 {
		return fmt.Errorf("文档id不能为空")
	}
	return nil
}

const Local = "local"
