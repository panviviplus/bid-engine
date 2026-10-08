package entity

import (
	"context"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

// PermissionConfigID 权限配置ID
type PermissionConfigID = int32

const (
	// PermissionConfigAllLibraries 全部知识库权限配置ID
	PermissionConfigAllLibraries PermissionConfigID = 4
)

// CommonStatus 状态
type CommonStatus = int32

const (
	// CommonStatusUnavailable 不可用
	CommonStatusUnavailable CommonStatus = 0
	// CommonStatusAvailable 可用
	CommonStatusAvailable CommonStatus = 1

	// CommonStatusErr 出错
	CommonStatusErr CommonStatus = 2

	// DocStatusUnapproved 文档未审批状态
	DocStatusUnapproved CommonStatus = 2

	ParseUndo  CommonStatus = 0
	ParseDone  CommonStatus = 1
	ParseDoing CommonStatus = 2
	ParseErr   CommonStatus = 3
)

// ConvertContext 将 gin.Context 转化为 context.Context, 并包含 x-request-id
func ConvertContext(c *gin.Context) context.Context {
	ctx := context.Background()
	mds := make(map[string]string)
	if c.Request != nil && c.Request.Header != nil {
		mds["x-request-id"] = c.GetHeader("X-Request-Id")
	} else {
		mds["x-request-id"] = GetRequestID(c)
	}
	return metadata.NewOutgoingContext(ctx, metadata.New(mds))
}

// LikeStatus 喜欢状态
type LikeStatus = int32

const (
	// LikeStatusDown 踩
	LikeStatusDown LikeStatus = -1
	// LikeStatusUnset 未设置
	LikeStatusUnset LikeStatus = 0
	// LikeStatusUp 顶
	LikeStatusUp LikeStatus = 1
)

// PubType 公开类型
type PubType = int32

const (
	// PubTypePrivate 私有
	PubTypePrivate PubType = 0
	// PubTypePub 公开
	PubTypePub PubType = 1
)

// IsValidPubType 是否是合法的类型
func IsValidPubType(p PubType) bool {
	return p == PubTypePrivate || p == PubTypePub
}

// IsPrivate 是不是私有的
func IsPrivate(p PubType) bool {
	return p == PubTypePrivate
}

// IsPublic 是不是共享的
func IsPublic(p PubType) bool {
	return p == PubTypePub
}

// Role 角色
type Role = int32

const (
	// RoleInvalid 不合法角色
	RoleInvalid Role = -2
	// RoleForbidden 无权限
	RoleForbidden Role = -1
	// RoleUser 普通用户
	RoleUser Role = 0
	// RoleAdmin 系统管理员/kb管理员
	RoleAdmin Role = 1
)

// ToRole 转换成角色
func ToRole(name string) Role {
	if name == "普通员工" {
		return RoleUser
	}
	if name == "系统管理员" {
		return RoleAdmin
	}
	return RoleInvalid
}

// IsValidRole 是否是合法的角色
func IsValidRole(r Role) bool {
	return r == RoleUser || r == RoleAdmin
}

// ApprovalOP 审批的操作类型
type ApprovalOP = int32

const (
	// ApprovalOPAddDoc 添加文档
	ApprovalOPAddDoc ApprovalOP = 0
	// ApprovalOPDelDoc 删除文档
	ApprovalOPDelDoc ApprovalOP = 1
	// ApprovalOPEditDoc 编辑文档
	ApprovalOPEditDoc ApprovalOP = 2

	// ApprovalOPDelLib 删除知识库
	//ApprovalOPDelLib ApprovalOP = 11

	// ApprovalOPJoinCompany 加入公司
	ApprovalOPJoinCompany ApprovalOP = 3

	// ApprovalOPCreateCompany 创建公司
	ApprovalOPCreateCompany ApprovalOP = 4
)

// ApprovalStatus 审批状态
type ApprovalStatus = int32

const (
	// ApprovalStatusSubmit 已提交
	ApprovalStatusSubmit ApprovalStatus = 0
	// ApprovalStatusPass 通过
	ApprovalStatusPass ApprovalStatus = 1
	// ApprovalStatusReject 驳回
	ApprovalStatusReject ApprovalStatus = 2
)

const (
	// ApprovalSceneMy 我的审批
	ApprovalSceneMy = "my"
	// ApprovalSceneTodo 审批管理
	ApprovalSceneTodo = "todo"
)

// Operation 可进行的操作
type Operation struct {
	Delete   bool `json:"delete"`
	Download bool `json:"download"`
	Edit     bool `json:"edit"`
}

// WritingType 写作类型
type WritingType = int32

const (
	// WritingTypeLoan 贷款报告
	WritingTypeLoan WritingType = 1
	// WritingTypeGeneral 通用文档
	WritingTypeGeneral WritingType = 2
)

type WritingSource = int32

const (
	// WritingSourceDefault 默认/参考文档
	WritingSourceDefault WritingSource = 0
	// WritingSourceCompanyBase 公司基本信息
	WritingSourceCompanyBase WritingSource = 1
	// WritingSourceLoanReport 贷款调查报告
	WritingSourceLoanReport WritingSource = 2
	// WritingSourceOther 其他补充材料
	WritingSourceOther WritingSource = 3
)

// IsValidWritingSource 是否是合法的类型
func IsValidWritingSource(p WritingSource) bool {
	return p == WritingSourceCompanyBase || p == WritingSourceLoanReport || p == WritingSourceOther ||
		p == WritingSourceDefault
}

//type WritingPlugin = int32
//
//var WritingPluginID2name = map[WritingPlugin]string{
//	1: "企查查",
//	2: "信贷系统",
//	3: "内评系统",
//}

const (
	// WritingSourceLibID 辅助写作材料对应的知识库ID
	WritingSourceLibID = -2
	// SqaHiddenLibID 问答上传url文档对应的隐藏知识库ID
	SqaHiddenLibID = -3
)

type Platform = int32

const (
	// PlatformH5 H5
	PlatformH5 Platform = iota // 0
	// PlatformMini 小程序
	PlatformMini
)

func GetPlatformFromCtx(c *gin.Context) Platform {
	if IsMiniprogram(c) {
		return PlatformMini
	}
	return PlatformH5
}
