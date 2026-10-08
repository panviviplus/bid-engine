package entity

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"


	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"bid-engine/pkg/db/model"
)

const (
	maxLibraryNameLen        = 20
	maxLibraryDescriptionLen = 300
	maxDirectoryNameLen      = 20
)

// Library 知识库
type Library struct {
	LibraryID   int32   `json:"library_id"` // 知识库ID
	Name        string  `json:"name"`       // 知识库名称
	UserID      int64   `json:"user_id"`    // 知识库名称
	PubType     PubType `json:"pub_type"`   // 是否公开 0:私有 1:知识库公开
	DocNum      int32   `json:"doc_num"`    // 文档数量
	Status      int32   `json:"status"`     // 状态信息 0:不可用/不可访问 1:可用
	UpdateTime  int64   `json:"update_time"`
	Role        Role    `json:"role,omitempty"` // 当前用户角色
	Description string  `json:"description"`
}

// Directory 目录结构
type Directory struct {
	DirectoryID       int32       `json:"directory_id"`        // 目录ID
	ParentDirectoryID int32       `json:"parent_directory_id"` // 父级目录ID
	Name              string      `json:"name"`                // 名称
	DocNum            int32       `json:"doc_num"`             // 文档数量
	UpdateTime        int64       `json:"update_time"`
	Children          []Directory `json:"children,omitempty"` // 目录ID
}

// LibWitDir 知识库，包含目录结构
type LibWitDir struct {
	Library
	Directories []Directory `json:"directories"` // 包含的目录结构
}

// AddLibraryReq 添加知识库
type AddLibraryReq struct {
	Name        string  `json:"name"`
	PubType     PubType `json:"pub_type"`
	Description string  `json:"description"`
}

// Valid 参数合法性校验
func (a *AddLibraryReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.Name == "" {
		return fmt.Errorf("知识库不能为空")
	}
	if utf8.RuneCountInString(a.Name) > maxLibraryNameLen {
		return fmt.Errorf("知识库名不能超过%d个字符", maxLibraryNameLen)
	}
	if !IsValidPubType(a.PubType) {
		return fmt.Errorf("知识库类型不支持")
	}
	if a.Description != "" && utf8.RuneCountInString(a.Description) > maxLibraryDescriptionLen {
		return fmt.Errorf("描述不能超过300字")
	}
	return nil
}

// UpdateLibraryReq 修改知识库
type UpdateLibraryReq struct {
	Name        string `json:"name"`
	LibraryID   int32  `uri:"library_id"`
	Description string `json:"description"`
}

// Valid 参数合法性校验
func (a *UpdateLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	_ = c.ShouldBindUri(&a)
	if a.Name == "" && a.Description == "" {
		return fmt.Errorf("知识库名和摘要不能同时为空")
	}
	if utf8.RuneCountInString(a.Name) > maxLibraryNameLen {
		return fmt.Errorf("知识库名不能超过%d个字符", maxLibraryNameLen)
	}
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	if a.Description != "" && utf8.RuneCountInString(a.Description) > maxLibraryDescriptionLen {
		return fmt.Errorf("描述不能超过300字")
	}
	return nil
}

// DeleteLibraryReq 删除知识库
type DeleteLibraryReq struct {
	LibraryID int32 `uri:"library_id"`
}

// Valid 参数合法性校验
func (a *DeleteLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	return nil
}

// CopyLibraryReq 复制知识库
type CopyLibraryReq struct {
	LibraryID int32 `uri:"library_id"`
}

// Valid 参数合法性校验
func (a *CopyLibraryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	return nil
}

// AddDirectoryReq 添加目录
type AddDirectoryReq struct {
	Name              string `json:"name"`
	LibraryID         int32  `json:"library_id"`
	ParentDirectoryID int32  `json:"parent_directory_id"`
}

// Valid 参数合法性校验
func (a *AddDirectoryReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.Name == "" {
		return fmt.Errorf("目录名不能为空")
	}
	if utf8.RuneCountInString(a.Name) > maxDirectoryNameLen {
		return fmt.Errorf("目录名不能超过%d个字符", maxDirectoryNameLen)
	}
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	return nil
}

// UpdateDirectoryReq 修改目录
type UpdateDirectoryReq struct {
	Name        string `json:"name"`
	DirectoryID int32  `uri:"directory_id"`
}

// Valid 参数合法性校验
func (a *UpdateDirectoryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	_ = c.ShouldBindUri(&a)
	if a.Name == "" {
		return fmt.Errorf("目录名不能为空")
	}
	if utf8.RuneCountInString(a.Name) > maxDirectoryNameLen {
		return fmt.Errorf("目录名不能超过%d个字符", maxDirectoryNameLen)
	}
	if a.DirectoryID == 0 {
		return fmt.Errorf("目录ID不能为空")
	}
	return nil
}

// DeleteDirectoryReq 删除目录
type DeleteDirectoryReq struct {
	DirectoryID int32 `uri:"directory_id"`
}

// Valid 参数合法性校验
func (a *DeleteDirectoryReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.DirectoryID == 0 {
		return fmt.Errorf("目录ID不能为空")
	}
	return nil
}

// AddUserReq 添加用户
type AddUserReq struct {
	Nickname string `json:"nickname" comment:"姓名"`
	Mobile   string `json:"mobile" comment:"手机号码"`
	Company  string `json:"company" comment:"所属公司"`
	Username string `json:"username" comment:"SN码"`
	Status   int32  `json:"status" comment:"账号状态,0-禁用，1-启用"`
}

// Valid 参数合法性校验
func (a *AddUserReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.Nickname == "" {
		return fmt.Errorf("姓名不能为空")
	}
	if utf8.RuneCountInString(a.Nickname) > 100 {
		return fmt.Errorf("姓名不能超过100个字符")
	}
	if a.Mobile == "" {
		return fmt.Errorf("手机号码不能为空")
	}
	// 手机号码格式校验（11位数字）
	if len(a.Mobile) != 11 {
		return fmt.Errorf("手机号码格式错误,必须为11位")
	}
	for _, r := range a.Mobile {
		if r < '0' || r > '9' {
			return fmt.Errorf("手机号码格式不正确")
		}
	}
	if a.Status != 0 && a.Status != 1 {
		return fmt.Errorf("账号状态只能是0（禁用）或1（启用）")
	}
	if a.Company == "" {
		return fmt.Errorf("所属公司不能为空")
	}
	if a.Username == "" {
		return fmt.Errorf("SN码不能为空")
	}
	return nil
}

// AddUsersReq 批量添加用户
type AddUsersReq struct {
	Users []AddUser
}

// AddUser 添加用户
type AddUser struct {
	Mobile     string `json:"mobile"`
	Role       string `json:"role"`
	FailReason string `json:"fail_reason,omitempty"`
}

// Valid 参数合法性校验
func (a *AddUsersReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	// 上传文件解析
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		return fmt.Errorf("获取上传的文档错误")
	}
	fileExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if fileExt != ".xlsx" {
		return fmt.Errorf("文档格式错误")
	}
	// 读取excel内容
	f, err := excelize.OpenReader(file)
	if err != nil {
		return fmt.Errorf("读取exel错误")
	}
	defer f.Close()
	rows, err := f.GetRows("员工列表")
	if err != nil {
		return fmt.Errorf("未找到【员工列表】sheet")
	}
	var users []AddUser
	for key, row := range rows {
		if len(row) == 0 {
			continue
		}
		if len(row) == 1 {
			users = append(users, AddUser{Mobile: row[0]})
			continue
		}
		// 表头
		if key == 0 && row[0] == "手机号码" {
			continue
		}
		users = append(users, AddUser{
			Mobile: row[0],
			Role:   row[1],
		})
	}
	if len(users) == 0 {
		return fmt.Errorf("excel表无内容")
	}
	a.Users = users
	return nil
}

// AddUsersData 批量添加用户返回结果
type AddUsersData struct {
	Success []AddUser `json:"success"`
	Fail    []AddUser `json:"fail"`
}

// UpdateUserReq 添加目录
type UpdateUserReq struct {
	TeamId   int64   `json:"teamId" comment:"团队ID"`
	UserID   int64   `uri:"user_id" comment:"用户ID"`
	Nickname *string `json:"nickname" comment:"姓名"`
	Mobile   *string `json:"mobile" comment:"手机号码"`
	Status   *int32  `json:"status" comment:"账号状态：0-禁用，1-启用"`
	Role     *string `json:"role" comment:"团队角色，枚举：owner、member"`

	Company    *string `json:"company" comment:"所属公司"`
	Department *string `json:"department" comment:"所属部门"`
	Username   *string `json:"username" comment:"SN码,工号"`
}

// Valid 参数合法性校验
func (a *UpdateUserReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	_ = c.ShouldBindUri(&a)
	if a.TeamId == 0 {
		return fmt.Errorf("团队ID不能为空")
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if a.Nickname != nil && len(*a.Nickname) > 100 {
		return fmt.Errorf("姓名不能超过100个字符")
	}
	if a.Mobile != nil {
		// 手机号码格式校验（11位数字）
		if len(*a.Mobile) != 11 {
			return fmt.Errorf("手机号码必须为11位")
		}
		for _, r := range *a.Mobile {
			if r < '0' || r > '9' {
				return fmt.Errorf("手机号码格式不正确")
			}
		}
	}
	if a.Status != nil && *a.Status != 0 && *a.Status != 1 {
		return fmt.Errorf("账号状态只能是0（禁用）或1（启用）")
	}
	if a.Role != nil && *a.Role != "member" && *a.Role != "owner" {
		return fmt.Errorf("用户的团队角色仅支持：owner或member")
	}
	return nil
}

// UserInfo 用户列表响应体
type UserInfo struct {
	UserID      int64  `json:"user_id" comment:"用户ID"`      
	CompanyID   int32  `json:"company_id" comment:"公司ID"`   
	CompanyName string `json:"company_name" comment:"公司名称"` 
	Nickname    string `json:"nickname" comment:"用户名称（昵称）"`     
	Mobile      string `json:"mobile" comment:"手机号码"`       
	Email       string `json:"email" comment:"邮箱"`            
	AvatarFile  string `json:"avatar_file" comment:"头像文件ObjectKey"` 
	Username    string `json:"username" comment:"工号"`       
	Role        string `json:"role" comment:"用户角色"`        
	Status      int32  `json:"status" comment:"用户状态0:禁用 1:启用"`   
	CreateTime  int64  `json:"create_time" comment:"创建时间"`  
	UpdateTime  int64  `json:"update_time" comment:"更新时间"`  
}

// SearchUserData 用户列表返回结构
type SearchUserData struct {
	Total int        `json:"total"`
	Users []UserInfo `json:"users"`
}

type ListUserReq struct {
	TeamId   int64  `json:"teamId" form:"teamId" comment:"团队ID"`
	Role     string `json:"role" form:"role" comment:"团队角色，枚举：owner、member"`
	Status   *int32 `json:"status" form:"status" comment:"用户状态：0-禁用，1-启用"`
	Nickname string `json:"nickname" form:"nickname" comment:"用户名称"`
	PageSize int    `json:"page_size" form:"page_size" comment:"每页条数"`
	PageNum  int    `json:"page_num" form:"page_num" comment:"页码"`
}

// Valid 参数合法性校验
func (a *ListUserReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.PageSize <= 0 {
		a.PageSize = 10
	}
	if a.PageNum < 1 {
		a.PageNum = 1
	}

	if a.Status != nil && *a.Status != 0 && *a.Status != 1 {
		return fmt.Errorf("用户状态参数错误，0-禁用,1-启用")
	}
	return nil
}

type ListUserWithoutPageReq struct {
	Nickname string `json:"nickname" form:"nickname" comment:"用户名称"`
}

// Valid 参数合法性校验
func (a *ListUserWithoutPageReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindQuery(&a); err != nil {
		return fmt.Errorf("参数绑定失败:%s", err.Error())
	}
	return nil
}

// ToUser 转换成用户
func ToUser(u *model.User) UserInfo {
	if u == nil {
		return UserInfo{}
	}
	info := UserInfo{
		UserID:     u.UserID,
		CompanyID:  u.CompanyID,
		Nickname:   u.Nickname,
		Mobile:     u.Mobile,
		Email:      u.Email,
		AvatarFile: u.AvatarFile,
		CreateTime: u.CreateTime,
		UpdateTime: u.UpdateTime,
		Status:     u.Status,
	}
	if info.Nickname == "" {
		info.Nickname = info.Mobile
	}
	return info
}

// DeleteUserReq 删除用户请求
type DeleteUserReq struct {
	UserID    int64 `uri:"user_id"`
	SaveShare bool  `json:"save_share"`
}

// Valid 参数合法性校验
func (a *DeleteUserReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if a.UserID == GetUserIDFromCtx(c) {
		return fmt.Errorf("不能自己删自己")
	}
	return nil
}

// DissolveTeamRelationReq 删除用户请求
type DissolveTeamRelationReq struct {
	TeamId int64 `json:"team_id"`
	UserID int64 `json:"user_id"`
}

// Valid 参数合法性校验
func (a *DissolveTeamRelationReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if a.TeamId == 0 {
		return fmt.Errorf("团队ID不能为空")
	}

	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if a.UserID == GetUserIDFromCtx(c) {
		return fmt.Errorf("不能自己删自己")
	}
	return nil
}

// DisableUserReq 禁用用户请求
type DisableUserReq struct {
	UserID int64 `uri:"user_id" comment:"用户ID"`
	TeamId int64 `uri:"team_id" comment:"团队ID"`
}

// Valid 参数合法性校验
func (a *DisableUserReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if a.TeamId == 0 {
		return fmt.Errorf("团队ID不能为空")
	}
	return nil
}

// EnableUserReq 启用用户请求
type EnableUserReq struct {
	UserID int64 `uri:"user_id" comment:"用户ID"`
	TeamId int64 `uri:"team_id" comment:"团队ID"`
}

// Valid 参数合法性校验
func (a *EnableUserReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(&a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if a.TeamId == 0 {
		return fmt.Errorf("团队ID不能为空")
	}
	return nil
}

// Privilege 知识库权限
type Privilege struct {
	UserID    int64 `json:"user_id"`
	LibraryID int32 `json:"library_id"`
	Role      Role  `json:"role"`
}

// QaPrivilege 问答库权限
type QaPrivilege struct {
	UserID      int64 `json:"user_id"`
	QaLibraryID int32 `json:"qa_lib_id"`
	Role        Role  `json:"role"`
}

// GetPrivilegeReq 获取用户知识库权限
type GetPrivilegeReq struct {
	UserID int64 `uri:"user_id"`
}

// Valid 参数合法性校验
func (a *GetPrivilegeReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	return nil
}

// PrivilegeData 用户知识库权限数据
type PrivilegeData struct {
	Libraries    []Library     `json:"libraries"`
	Privileges   []Privilege   `json:"privileges"`
	QaLibraries  []QaLibrary   `json:"qa_libraries"`
	QaPrivileges []QaPrivilege `json:"qa_privileges"`
}

// UpdatePrivilegeReq 更新用户知识库权限
type UpdatePrivilegeReq struct {
	UserID       int64         `uri:"user_id"`
	Privileges   []Privilege   `json:"privileges"`
	QaPrivileges []QaPrivilege `json:"qa_privileges"`
}

// Valid 参数合法性校验
func (a *UpdatePrivilegeReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.UserID == 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	return nil
}

type SegmentationConfig struct {
	EnableCustomSegmentation bool    `json:"enable_custom_segmentation" form:"enable_custom_segmentation"`
	SplitSeparator           string  `json:"split_separator" form:"split_separator"`
	MaxSegmentLength         int32   `json:"max_segment_length" form:"max_segment_length"`
	OverlapRatio             float64 `json:"overlap_ratio" form:"overlap_ratio"`
	RemoveWhitespace         bool    `json:"remove_whitespace" form:"remove_whitespace"`
	FilterLinks              bool    `json:"filter_links" form:"filter_links"`
}

// UploadDocReq 上传文档请求参数
type UploadDocReq struct {
	LibraryID          int32   `form:"library_id"`
	DirectoryID        int32   `form:"directory_id"`
	PubType            PubType `form:"pub_type"`
	HandleDuplicateDoc int32   `form:"handle_duplicate_doc"` // 0 首次上传 1 覆盖 2 共存
	ExpireTime         int64   `form:"expire_time"`
	SegmentationConfig
}


// HandleDuplicateDoc 文档类型
type HandleDuplicateDoc = int32

const (
	// HandleDuplicateDocUpload 首次上传
	HandleDuplicateDocUpload HandleDuplicateDoc = 0
	// HandleDuplicateDocOverwrite 覆盖
	HandleDuplicateDocOverwrite HandleDuplicateDoc = 1
	// HandleDuplicateDocCoexist 共存
	HandleDuplicateDocCoexist HandleDuplicateDoc = 2
)

// Valid 参数合法性校验
func (a *UploadDocReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	if !IsValidPubType(a.PubType) {
		return fmt.Errorf("文档权限不支持")
	}
	return nil
}

// UploadDocNotifyReq 文档上传之后通知接口
type UploadDocNotifyReq struct {
	DocumentIDs []string `json:"document_ids"`
}

// Valid 参数合法性校验
func (a *UploadDocNotifyReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if len(a.DocumentIDs) == 0 {
		return fmt.Errorf("文档ids不能为空")
	}
	return nil
}

// UploadURLReq 添加url
type UploadURLReq struct {
	LibraryID          int32    `json:"library_id"`
	DirectoryID        int32    `json:"directory_id"`
	PubType            PubType  `json:"pub_type"`
	URLs               []string `json:"urls"`
	ExpireTime         int64    `json:"expire_time"`
	HandleDuplicateDoc int32    `json:"handle_duplicate_doc"` // 0 首次上传 1 覆盖 2 共存
	SegmentationConfig
}

// Valid 参数合法性校验
func (a *UploadURLReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	if a.LibraryID == 0 {
		return fmt.Errorf("知识库ID不能为空")
	}
	if !IsValidPubType(a.PubType) {
		return fmt.Errorf("文档权限不支持")
	}
	if len(a.URLs) == 0 {
		return fmt.Errorf("urls不能为空")
	}
	return nil
}

// UploadUrlCheckData 上传url检测
type UploadUrlCheckData struct {
	DuplicateURLs []Doc `json:"duplicate_urls"`
}

// UpdateDocReq 更新文档
type UpdateDocReq struct {
	DocumentID string `uri:"document_id"`
	//DocumentName string `json:"document_name"`
	Version    *int64 `json:"version"`
	ExpireTime *int64 `json:"expire_time"`
}

// Valid 参数合法性校验
func (u *UpdateDocReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&u)
	_ = c.ShouldBindUri(&u)
	if u.DocumentID == "" {
		return fmt.Errorf("document_id 不能为空")
	}
	//if u.DocumentName == "" {
	//	return fmt.Errorf("document_name 不能为空")
	//}
	if u.Version != nil && *u.Version < 1 {
		return fmt.Errorf("version 大于等于1")
	}
	//if u.ExpireTime != nil && *u.ExpireTime < -1 {
	//	return fmt.Errorf("expire_time 大于等于-1")
	//}

	return nil
}

// GetDocsReq 文档搜索请求参数
type GetDocsReq struct {
	DocumentIDs []string `json:"document_ids"`
}

// Valid 参数合法性校验
func (a *GetDocsReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	if len(a.DocumentIDs) == 0 {
		return fmt.Errorf("DocumentIDs 不能为空")
	}
	return nil
}

type DocListPreference struct {
	LibraryID    bool `json:"library_id"`    // 知识库id
	UploadTime   bool `json:"upload_time"`   // 上传时间
	Username     bool `json:"username"`      // 用户id
	PubType      bool `json:"pub_type"`      // 文档类型：0-私有 1-公开
	Size         bool `json:"size"`          // 文档大小
	DocumentType bool `json:"document_type"` // 文档类型：0-pdf
	DocumentExt  bool `json:"document_ext"`  // 文档后缀
	ExpireTime   bool `json:"expire_time"`
	Version      bool `json:"version"`
	ParseStatus  bool `json:"parse_status"` // 解析状态
	ParagraphNum bool `json:"paragraph_num"`
}

func (a *DocListPreference) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	return nil
}

// SearchDocReq 文档搜索请求参数
type SearchDocReq struct {
	Query         string  `json:"query" form:"query"`
	LibraryID     int32   `json:"library_id" form:"library_id"`
	Expired       bool    `json:"expired" form:"expired"`
	DirectoryID   int32   `json:"directory_id" form:"directory_id"`
	PageSize      int     `json:"page_size" form:"page_size"`
	PageNum       int     `json:"page_num" form:"page_num"`
	ParseStatuses []int32 `json:"parse_statuses" form:"parse_statuses"`
	FileTypes     []int32 `json:"file_types" form:"file_types"`
}

// Valid 参数合法性校验
func (a *SearchDocReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	if a.PageSize <= 0 || a.PageSize > 50 {
		a.PageSize = 10
	}
	if a.PageNum < 1 {
		a.PageNum = 1
	}
	return nil
}

// Doc 文档信息
type Doc struct {
	DocumentID   string              `json:"document_id"`   // 文档id
	PubType      int32               `json:"pub_type"`      // 文档类型：0-私有 1-公开
	UserID       int64               `json:"user_id"`       // 用户id
	Username     string              `json:"username"`      // 用户id
	LibraryID    int32               `json:"library_id"`    // 知识库id
	LibraryName  string              `json:"library_name"`  // 知识库id
	DirectoryID  int32               `json:"directory_id"`  // 目录id
	DocumentType int32               `json:"document_type"` // 文档类型：0-pdf
	DocumentExt  string              `json:"document_ext"`  // 文档后缀
	DocumentName string              `json:"document_name"` // 文档名
	URL          string              `json:"url"`           // url地址
	ParseStatus  int32               `json:"parse_status"`  // 解析状态
	Size         int64               `json:"size"`          // 文档大小
	PageNum      int64               `json:"page_num"`      // 页数
	ParagraphNum int32               `json:"paragraph_num"` // 段落数
	UploadTime   int64               `json:"upload_time"`   // 上传时间
	Role         Role                `json:"role,omitempty"`
	Operation    *Operation          `json:"operation,omitempty"` // 用户科技行的操作
	Status       int32               `json:"status"`
	Jpg          string              `json:"jpg"`
	Version      int64               `json:"version"`
	ExpireTime   int64               `json:"expire_time"`
	SegConfig    *SegmentationConfig `json:"seg_config"`
}

const JpgSecretKey = "ec2f88cc-43e8-4588-8e18-02a2db49f5b2"

// 生成AES密钥，使用SHA-256哈希函数从secret_key生成固定长度密钥
func generateKey(secretKey string) []byte {
	hash := sha256.Sum256([]byte(secretKey))
	return hash[:]
}

// PKCS7填充
func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// PKCS7去除填充
func pkcs7UnPadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("invalid padding size")
	}
	padding := int(data[length-1])
	if padding > length {
		return nil, errors.New("invalid padding")
	}
	return data[:length-padding], nil
}

// Encrypt 加密函数
func Encrypt(docID string, createTime int64, secretKey string) (string, error) {
	// 将 doc_id 和 create_time（int64 类型）合并为一个字符串
	data := []byte(docID + "," + strconv.FormatInt(createTime, 10))

	// 生成密钥
	key := generateKey(secretKey)

	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// 使用CBC模式需要一个初始化向量（IV）
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// 填充数据
	data = pkcs7Padding(data, aes.BlockSize)

	// 使用CBC模式进行加密
	cipherText := make([]byte, len(data))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText, data)

	// 最终加密结果 = IV + 密文
	finalCipherText := append(iv, cipherText...)

	// 编码为Base64字符串
	return base64.URLEncoding.EncodeToString(finalCipherText), nil
}

// Decrypt 解密函数
func Decrypt(encryptedMessage, secretKey string) (string, int64, error) {
	if len(encryptedMessage) != 88 {
		return "", 0, fmt.Errorf("参数长度非法:%d", len(encryptedMessage))
	}
	// 解码Base64字符串
	cipherText, err := base64.URLEncoding.DecodeString(encryptedMessage)
	if err != nil {
		return "", 0, err
	}

	// 生成密钥
	key := generateKey(secretKey)

	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", 0, err
	}

	// 提取IV和加密数据
	if len(cipherText) < aes.BlockSize {
		return "", 0, errors.New("cipherText too short")
	}
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	// 使用CBC模式解密
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)

	// 去除填充
	plainText, err := pkcs7UnPadding(cipherText)
	if err != nil {
		return "", 0, err
	}

	// 将解密的数据分解为 doc_id 和 create_time
	result := string(plainText)
	parts := bytes.Split([]byte(result), []byte(","))
	if len(parts) != 2 {
		return "", 0, errors.New("invalid decrypted data")
	}

	// 将 create_time 从字符串转换回 int64
	createTime, err := strconv.ParseInt(string(parts[1]), 10, 64)
	if err != nil {
		return "", 0, err
	}

	return string(parts[0]), createTime, nil
}

// AddOperation 添加用户权限
func (d *Doc) AddOperation(userID int64, role Role) {
	op := &Operation{}
	if d.UserID == userID || role == RoleAdmin {
		op.Delete = true
		op.Download = true
		op.Edit = true
	} else {
		if role == RoleUser {
			op.Download = true
		}
	}
	d.Operation = op
}

// SearchDocData 用户搜索返回结构
type SearchDocData struct {
	Total int   `json:"total"`
	Docs  []Doc `json:"docs"`
}

// GetDocsData 获取文档返回结构
type GetDocsData struct {
	Docs []GetDoc `json:"docs"`
}

// GetDoc 文档信息
type GetDoc struct {
	DocumentID   string `json:"document_id"` // 文档id
	DocumentName string `json:"document_name"`
	DocumentExt  string `json:"document_ext"` // 文档后缀
	ParseStatus  int32  `json:"parse_status"` // 解析状态
	Jpg          string `json:"jpg"`
}

// DeleteDocReq 删除文档请求
type DeleteDocReq struct {
	DocumentID string `uri:"document_id"`
}

// Valid 参数合法性校验
func (a *DeleteDocReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.DocumentID == "" {
		return fmt.Errorf("文档不能为空")
	}
	return nil
}

// DeleteDocsReq 批量删除文档请求
type DeleteDocsReq struct {
	DocumentIDs []string `json:"document_ids"`
}

// Valid 参数合法性校验
func (a *DeleteDocsReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if len(a.DocumentIDs) == 0 {
		return fmt.Errorf("文档列表不能为空")
	}
	// 文档去重
	var ids []string
	idMap := make(map[string]bool)
	for _, id := range a.DocumentIDs {
		if idMap[id] {
			continue
		}
		idMap[id] = true
		ids = append(ids, id)
	}
	a.DocumentIDs = ids
	return nil
}

// DeleteDocsData 批量删除文档数据
type DeleteDocsData struct {
	Successes []string `json:"successes"`
	Fails     []string `json:"fails"`
	Approvals []string `json:"approvals"`
}

// DownloadDocReq 删除文档请求
type DownloadDocReq struct {
	DocumentID string `uri:"document_id"`
}

// Valid 参数合法性校验
func (a *DownloadDocReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.DocumentID == "" {
		return fmt.Errorf("文档不能为空")
	}
	return nil
}

// DownloadDocsReq 批量删除文档请求
type DownloadDocsReq struct {
	OrigDocumentIDs string `form:"document_ids"`
	DocumentIDs     []string
}

// Valid 参数合法性校验
func (a *DownloadDocsReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if a.OrigDocumentIDs == "" {
		return fmt.Errorf("文档不能为空")
	}
	a.DocumentIDs = strings.Split(a.OrigDocumentIDs, ",")
	if len(a.DocumentIDs) == 0 {
		return fmt.Errorf("文档不能为空")
	}
	return nil
}

// Approval 审批
type Approval struct {
	ApprovalID       int32  `json:"approval_id"`       // 审批ID
	UserID           int64  `json:"user_id"`           // 用户ID
	Mobile           string `json:"mobile,omitempty"`  // 手机号
	Username         string `json:"username"`          // 用户名
	LibraryID        int32  `json:"library_id"`        // 知识库ID
	LibraryName      string `json:"library_name"`      // 知识库名称
	Op               int32  `json:"op"`                // 0:添加文档 1:删除文档 2:编辑文档 11:删除知识库
	OpExt            string `json:"op_ext"`            // 额外信息
	ApprovalUserID   int64  `json:"approval_user_id"`  // 审批人ID
	ApprovalUsername string `json:"approval_username"` // 审批人员用户名
	ApprovalText     string `json:"approval_text"`     // 审批文案
	CreateTime       int64  `json:"create_time"`       // 提交时间
	UpdateTime       int64  `json:"update_time"`       // 审批时间
	Status           int32  `json:"status"`
}

// MyApprovalReq 我的审批请求参数
type MyApprovalReq struct {
	PageSize  int        `json:"page_size" form:"page_size"`
	PageNum   int        `json:"page_num" form:"page_num"`
	Op        ApprovalOP `json:"op" form:"op"`
	StartTime int64      `json:"start_time" form:"start_time"`
	EndTime   int64      `json:"end_time" form:"end_time"`
	Status    int32      `json:"status" form:"status"`
	LibraryID int32      `json:"library_id" form:"library_id"`
}

// Valid 参数合法性校验
func (a *MyApprovalReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&a)
	if a.PageSize <= 0 || a.PageSize > 50 {
		a.PageSize = 10
	}
	if a.PageNum < 1 {
		a.PageNum = 1
	}
	return nil
}

// MyApprovalData 我的审批结构
type MyApprovalData struct {
	Total     int64      `json:"total"`
	Approvals []Approval `json:"approvals"`
}

// ApprovalTodoReq 审批管理请求参数
type ApprovalTodoReq struct {
	PageSize  int        `form:"page_size"`
	PageNum   int        `form:"page_num"`
	Op        ApprovalOP `form:"op"`
	UserID    int64      `form:"user_id"`
	StartTime int64      `form:"start_time"`
	EndTime   int64      `form:"end_time"`
	Status    int32      `form:"status"`
	LibraryID int32      `form:"library_id"`
}

// Valid 参数合法性校验
func (a *ApprovalTodoReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&a)
	//if a.PageSize <= 0 || a.PageSize > 20 {
	//	a.PageSize = 20
	//}
	if a.PageNum < 1 {
		a.PageNum = 1
	}
	if a.Op == ApprovalOPCreateCompany {
		return fmt.Errorf("操作类型不支持")
	}
	return nil
}

// ApprovalTodoData 审批管理结构
type ApprovalTodoData struct {
	Total     int        `json:"total"`
	Approvals []Approval `json:"approvals"`
}

// UpdateApprovalReq 更新审批状态
type UpdateApprovalReq struct {
	ApprovalID   int32          `uri:"approval_id"`
	Status       ApprovalStatus `json:"status"`
	ApprovalText string         `json:"approval_text"` // 审批文案
}

// Valid 参数合法性校验
func (a *UpdateApprovalReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	_ = c.ShouldBindJSON(&a)
	if a.ApprovalID == 0 {
		return fmt.Errorf("审批ID不能为空")
	}
	if a.Status == ApprovalStatusReject {
		if a.ApprovalText == "" {
			return fmt.Errorf("审批意见不能为空")
		}
		return nil
	}
	if a.Status != ApprovalStatusPass {
		return fmt.Errorf("审批状态不合法")
	}
	return nil
}

// UpdateFeedbackReq 更新反馈
type UpdateFeedbackReq struct {
	FeedbackID int64    `uri:"feedback_id"`
	Score      *float64 `json:"score" comment:"评分，范围[0,5]，步长0.5"`
	Content    *string  `json:"content" comment:"反馈内容"`
	Tags       []string `json:"tags" comment:"标签列表，前端以数组传入"`
}

// Valid 参数合法性校验
func (a *UpdateFeedbackReq) Valid(c *gin.Context) error {
	if err := c.ShouldBindUri(&a); err != nil {
		return fmt.Errorf("参数错误: %s", err.Error())
	}
	_ = c.ShouldBindJSON(&a)
	if a.FeedbackID == 0 {
		return fmt.Errorf("ID不能为空")
	}
	// 至少一个可更新字段
	if a.Score == nil && a.Content == nil && a.Tags == nil {
		return fmt.Errorf("至少提供一个可更新字段：score/content/tags")
	}
	// 评分范围校验
	if a.Score != nil && (*a.Score < 0.0 || *a.Score > 5.0) {
		return fmt.Errorf("评分必须在[0,5]范围内")
	}
	return nil
}

// ApprovalOpinionsReq 审批相关选项请求参数
type ApprovalOpinionsReq struct {
	Scene string `json:"scene" form:"scene"`
}

// Valid 参数合法性校验
func (a *ApprovalOpinionsReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	_ = c.ShouldBindJSON(&a)
	if a.Scene != ApprovalSceneMy && a.Scene != ApprovalSceneTodo {
		return fmt.Errorf("类型不支持")
	}
	return nil
}

// ApprovalOpinionsData 审批相关选项数据结果
type ApprovalOpinionsData struct {
	Libraries     []Library  `json:"libraries"`
	Users         []UserInfo `json:"users,omitempty"`
	ApprovalUsers []UserInfo `json:"approval_users,omitempty"`
}

// ApprovalDetail 审批详情
type ApprovalDetail struct {
	Approval
	Docs []Doc `json:"docs"`
}

// GetApprovalReq 获取审批详情
type GetApprovalReq struct {
	ApprovalID int32 `uri:"approval_id"`
}

// Valid 参数合法性校验
func (a *GetApprovalReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.ApprovalID == 0 {
		return fmt.Errorf("审批ID不能为空")
	}
	return nil
}

// CancelApprovalReq 取消申请请求参数
type CancelApprovalReq struct {
	ApprovalID int32 `json:"approval_id"`
}

// Valid 参数合法性校验
func (a *CancelApprovalReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if a.ApprovalID == 0 {
		return fmt.Errorf("申请信息不能为空")
	}
	return nil
}


// UploadExcelReq 上传Excel文件请求
type UploadExcelReq struct {
	// 文件通过multipart/form-data上传，在handler中处理
	// 文件大小不超过10M，后端接口需要做文件大小校验
}

// Valid 参数合法性校验
func (a *UploadExcelReq) Valid(c *gin.Context) error {
	// 文件大小校验在handler中处理
	return nil
}

// ExcelUserRow Excel文件中的用户行数据
type ExcelUserRow struct {
	Username       string `json:"username" comment:"SN码"`                 // SN码（工号）
	Nickname       string `json:"nickname" comment:"姓名"`                  // 姓名
	Mobile         string `json:"mobile" comment:"手机号码"`                  // 手机号码
	CompanyName    string `json:"company_name" comment:"公司名称"`            // 公司名称
	RowNumber      int    `json:"row_number" comment:"行号"`                // Excel中的行号
	ErrorMessage   string `json:"error_message,omitempty" comment:"错误信息"` // 处理错误信息
}

// UploadExcelData 上传Excel文件处理结果
type UploadExcelData struct {
	Total       int            `json:"total" comment:"总行数,不包括表头"`        // 总处理行数
	Success     int            `json:"success" comment:"成功创建用户数"`        // 成功处理行数
	Failed      int            `json:"failed" comment:"失败行数"`            // 失败处理行数
	IAMCalls    int            `json:"iam_calls" comment:"调用IAM接口次数"`    // 调用IAM接口次数
	Details     []ExcelUserRow `json:"details" comment:"详细处理结果"`         // 详细处理结果
	SuccessList []ExcelUserRow `json:"success_list" comment:"成功创建的用户列表"` // 成功创建的用户列表
	FailedList  []ExcelUserRow `json:"failed_list" comment:"失败的用户列表"`    // 失败的用户列表
}

// PageLibrariesReq 分页获取有权限知识库
type PageLibrariesReq struct {
	Name     string `json:"name" comment:"知识库名称，模糊匹配"`
	PageNum  int    `json:"page_num" comment:"分页参数"`
	PageSize int    `json:"page_size" comment:"分页参数"`
}

// Valid 参数合法性校验
func (a *PageLibrariesReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&a)
	//分页参数校验，纠偏
	if a.PageSize <= 0 || a.PageSize > 50 {
		a.PageSize = 10
	}
	if a.PageNum < 1 {
		a.PageNum = 1
	}
	return nil
}

// LibraryItem 知识库项
type LibraryItem struct {
	LibraryID   int32  `json:"library_id" comment:"知识库ID"`
	Name        string `json:"name" comment:"知识库名称"`
	Description string `json:"description" comment:"知识库描述"`
	PubType     int32  `json:"pub_type" comment:"是否公开 0:私有 1:知识库公开"`
	Status      int32  `json:"status" comment:"状态信息 0:不可用/不可访问 1:可用"`
	CreateTime  int64  `json:"create_time" comment:"创建时间"`
	UpdateTime  int64  `json:"update_time" comment:"更新时间"`
}

// PageLibrariesData 分页查询知识库响应
type PageLibrariesData struct {
	Total     int           `json:"total" comment:"总数"`
	Libraries []LibraryItem `json:"libraries" comment:"知识库列表"`
}
