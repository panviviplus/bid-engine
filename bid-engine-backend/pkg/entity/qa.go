package entity

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/gin-gonic/gin"
)

// QaLibrary 问答库
type QaLibrary struct {
	QaLibraryID  int32   `json:"qa_lib_id"` // 问答库ID
	Name         string  `json:"name"`      // 问答库名称
	UserID       int64   `json:"user_id"`   // 问答库名称
	PubType      PubType `json:"pub_type"`  // 问答库是否公开 0:私有 1:公开
	Status       int32   `json:"status"`    // 状态信息 0:不可用/不可访问 1:可用
	UpdateTime   int64   `json:"update_time"`
	Role         Role    `json:"role"`          // 当前用户角色
	SearchStatus int32   `json:"search_status"` // 状态信息 0:不可搜索 1:可搜索
	QaNum        int32   `json:"qa_num"`        // 问答对数量
	Export       bool    `json:"export"`
}

// GetQaLibrariesReq 问答库列表请求参数
type GetQaLibrariesReq struct {
	Query   string `json:"query" form:"query"`
	PubType *int32 `json:"pub_type" form:"pub_type"`
}

// Valid 参数合法性校验
func (a *GetQaLibrariesReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	return nil
}

// SetQaLibrarySearchStatusReq 设置问答库状态
type SetQaLibrarySearchStatusReq struct {
	QaLibraryID  int32 `json:"qa_lib_id"` // 问答库ID
	SearchStatus int32 `json:"search_status"`
}

// Valid 参数合法性校验
func (a *SetQaLibrarySearchStatusReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.QaLibraryID == 0 {
		return fmt.Errorf("问答库ID不能为空")
	}
	return nil
}

// UpdateQaLibraryReq 更新问答库状态
type UpdateQaLibraryReq struct {
	QaLibraryID int32  `json:"qa_lib_id"` // 问答库ID
	Name        string `json:"name"`
}

// Valid 参数合法性校验
func (a *UpdateQaLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.QaLibraryID == 0 {
		return fmt.Errorf("问答库ID不能为空")
	}
	if utf8.RuneCountInString(a.Name) > maxLibraryNameLen {
		return fmt.Errorf("问答库名不能超过%d个字符", maxLibraryNameLen)
	}
	return nil
}

// AddQaLibraryReq 添加
type AddQaLibraryReq struct {
	Name    string `json:"name"`
	PubType int32  `json:"pub_type"`
}

// Valid 参数合法性校验
func (a *AddQaLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	//if err := c.ShouldBind(&a); err != nil {
	//	return fmt.Errorf("参数错误:%s", err.Error())
	//}
	if a.Name == "" {
		return fmt.Errorf("问答库库不能为空")
	}
	if utf8.RuneCountInString(a.Name) > maxLibraryNameLen {
		return fmt.Errorf("问答库库名不能超过%d个字符", maxLibraryNameLen)
	}
	if !IsValidPubType(a.PubType) {
		return fmt.Errorf("问答库类型不支持")
	}
	return nil
}

// DeleteQaLibraryReq 添加
type DeleteQaLibraryReq struct {
	QaLibraryID int32 `uri:"qa_lib_id"`
}

// Valid 参数合法性校验
func (a *DeleteQaLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.QaLibraryID == 0 {
		return fmt.Errorf("qa_lib_id 不能为空")
	}
	return nil
}

type ImportQaData struct {
	Successes []int `json:"successes_row"`
	Fails     []int `json:"fails_row"`
}

// ImportQaReq 导入问答对请求参数
type ImportQaReq struct {
	QaLibraryID int32 `json:"qa_lib_id" form:"qa_lib_id"` // 问答库ID
	PubType     int32 `json:"pub_type" form:"pub_type"`
	Rows        [][]string
}

// MigrateQasData 批量移动qa数据

const (
	maxImportQaFileSize = 1024 * 1024 * 50
	maxImportQaRow      = 100000
)

// Valid 参数合法性校验
func (a *ImportQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if a.QaLibraryID == 0 {
		return fmt.Errorf("qa_lib_id 不能为空")
	}
	// 上传文件解析
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		return fmt.Errorf("获取上传的文档错误")
	}
	fileExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if fileExt != DocExtXLSX {
		return fmt.Errorf("文件类型不支持:%s", fileExt)
	}

	if fileHeader.Size == 0 {
		return fmt.Errorf("文件不能为空")
	}
	if fileHeader.Size > maxImportQaFileSize {
		return fmt.Errorf("文件大小不能超过5M")
	}
	defer file.Close()
	f, err := excelize.OpenReader(file)
	if err != nil {
		return fmt.Errorf("获取上传的文档错误")
	}
	if len(f.GetSheetList()) < 1 {
		return fmt.Errorf("至少有一个sheet")
	}
	sheet := f.GetSheetList()[0]
	// 获取第一个工作表
	sheetName := sheet
	if sheetName == "" {
		return fmt.Errorf("未找到工作表")
	}
	// 获取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("行格式存在错误")
	}

	if len(rows) < 2 {
		return fmt.Errorf("内容不能为空")
	}
	if len(rows) > maxImportQaRow {
		return fmt.Errorf("不超过10万行")
	}

	a.Rows = rows
	return nil
}

// ExportQaReq 导出问答对请求参数
type ExportQaReq struct {
	QaLibraryID int32 `uri:"qa_lib_id"` // 问答库ID
}

// Valid 参数合法性校验
func (a *ExportQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.QaLibraryID == 0 {
		return fmt.Errorf("qa_lib_id 不能为空")
	}
	return nil
}

// GetQasReq 问答对列表请求参数
type GetQasReq struct {
	QaLibraryID int32  `json:"qa_lib_id" form:"qa_lib_id"` // 问答库ID
	Query       string `json:"query" form:"query"`
	PageSize    int    `json:"page_size" form:"page_size"`
	PageNum     int    `json:"page_num" form:"page_num"`
	PubType     *int32 `json:"pub_type" form:"pub_type"`
}

// Valid 参数合法性校验
func (a *GetQasReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)

	//if a.QaLibraryID == 0 {
	//	return fmt.Errorf("qa_lib_id 不能为空")
	//}
	if a.PageSize <= 0 || a.PageSize > 50 {
		a.PageSize = 10
	}
	if a.PageNum < 1 {
		a.PageNum = 1
	}
	return nil
}

// GetQasData 返回结构
type GetQasData struct {
	Total int64 `json:"total"`
	Qas   []Qa  `json:"qas"`
}

// AddQaReq 添加
type AddQaReq struct {
	Question    string   `json:"question"`
	Answer      string   `json:"answer"`
	DocIDs      []string `json:"doc_ids"`
	PubType     int32    `json:"pub_type"`
	QaLibraryID int32    `json:"qa_lib_id"`
}

// Valid 参数合法性校验
func (a *AddQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if a.Question == "" {
		return fmt.Errorf("question 不能为空")
	}
	if a.Answer == "" {
		return fmt.Errorf("answer 不能为空")
	}
	//if len(a.DocIDs) == 0 {
	//	return fmt.Errorf("doc_ids 不能为空")
	//}
	if utf8.RuneCountInString(a.Question) > 200 {
		return fmt.Errorf("question 不能超过200字符")
	}
	if utf8.RuneCountInString(a.Answer) > 2000 {
		return fmt.Errorf("answer 不能超过2000字符")
	}
	return nil
}

// UpdateQaReq 添加
type UpdateQaReq struct {
	QaID        string   `json:"qa_id"`
	Question    string   `json:"question"`
	Answer      string   `json:"answer"`
	DocIDs      []string `json:"doc_ids"`
	PubType     int32    `json:"pub_type"`
	QaLibraryID int32    `json:"qa_lib_id"`
}

// Valid 参数合法性校验
func (a *UpdateQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if a.Question == "" {
		return fmt.Errorf("question 不能为空")
	}
	if a.Answer == "" {
		return fmt.Errorf("answer 不能为空")
	}
	//if len(a.DocIDs) == 0 {
	//	return fmt.Errorf("doc_ids 不能为空")
	//}
	if a.QaID == "" {
		return fmt.Errorf("qa_id 不能为空")
	}
	if a.QaLibraryID == 0 {
		return fmt.Errorf("qa_lib_id 不能为空")
	}
	if utf8.RuneCountInString(a.Question) > 200 {
		return fmt.Errorf("question 不能超过200字符")
	}
	if utf8.RuneCountInString(a.Answer) > 2000 {
		return fmt.Errorf("answer 不能超过2000字符")
	}
	return nil
}

// DeleteQasReq 删除
type DeleteQasReq struct {
	QaIDs []string `json:"qa_ids"`
}

// Valid 参数合法性校验
func (a *DeleteQasReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if len(a.QaIDs) == 0 {
		return fmt.Errorf("qa_ids 不能为空")
	}
	return nil
}

// DeleteQaReq 删除
type DeleteQaReq struct {
	QaID string `uri:"qa_id"`
}

// Valid 参数合法性校验
func (a *DeleteQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.QaID == "" {
		return fmt.Errorf("qa_id 不能为空")
	}
	return nil
}

// DeleteQasData 批量删除qa数据
type DeleteQasData struct {
	Successes []string `json:"successes"`
	Fails     []string `json:"fails"`
}

// GetDocQaReq 添加
type GetDocQaReq struct {
	DocumentID string `uri:"document_id"`
}

// Valid 参数合法性校验
func (a *GetDocQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.DocumentID == "" {
		return fmt.Errorf("doc_id 不能为空")
	}

	return nil
}

type DocQas struct {
	Qas []Qa `json:"qas"`
}

type QaDocInfo struct {
	DocumentID   string `json:"document_id"`
	DocumentName string `json:"document_name"`
}

type Qa struct {
	QaID         string      `json:"qa_id"`
	Question     string      `json:"question"`
	Answer       string      `json:"answer"`
	PubType      int32       `json:"pub_type"`
	Edit         bool        `json:"edit"`
	UpdateTime   int64       `json:"update_time"`
	Documents    []QaDocInfo `json:"documents,omitempty"`
	EditUserName string      `json:"edit_user_name"`
	LibraryName  string      `json:"library_name"`
	QaLibraryID  int32       `json:"qa_lib_id"`
	Role         Role        `json:"role"` // 当前用户角色
	Doing        bool        `json:"doing"`
	DoingMsg     string      `json:"doing_msg"`
}

// MigrateQasReq 迁移
type MigrateQasReq struct {
	QaIDs             []string `json:"qa_ids"`
	TargetQaLibraryID int32    `json:"target_qa_lib_id"`
}

// Valid 参数合法性校验
func (a *MigrateQasReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if len(a.QaIDs) == 0 {
		return fmt.Errorf("qa_ids 不能为空")
	}

	if a.TargetQaLibraryID == 0 {
		return fmt.Errorf("target_qa_lib_id 不能为空")
	}
	return nil
}

// MigrateQasData 批量移动qa数据
type MigrateQasData struct {
	Successes []string `json:"successes"`
	Fails     []string `json:"fails"`
}

// GenerateQaReq 生成
type GenerateQaReq struct {
	DocumentID  string `uri:"document_id"`
	QaLibraryID int32  `json:"qa_lib_id"`
	SampleNum   int32  `json:"sample_num"`
	PubType     int32  `json:"pub_type"`
}

// Valid 参数合法性校验
func (a *GenerateQaReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBind(&a)
	if a.DocumentID == "" {
		return fmt.Errorf("doc_id 不能为空")
	}

	if a.SampleNum > 10 || a.SampleNum < 1 {
		return fmt.Errorf("sample_num 范围[1,10]")
	}

	return nil
}
