package entity

// AuditAction 审计行为
type AuditAction = int32

const (
	// AuditActionUserJoin 加入公司
	AuditActionUserJoin AuditAction = 1
	// AuditActionUserDelete 用户被删除
	AuditActionUserDelete AuditAction = 2
	// AuditActionUserLogin 登录
	AuditActionUserLogin AuditAction = 10

	// AuditActionLibraryAdd 添加知识库
	AuditActionLibraryAdd AuditAction = 101
	// AuditActionLibraryDelete 删除术语库
	AuditActionLibraryDelete AuditAction = 102
	// AuditActionLibraryUpdate 更新术语库
	AuditActionLibraryUpdate AuditAction = 103

	// AuditActionDocAdd 上传文档
	AuditActionDocAdd AuditAction = 201
	// AuditActionDocDelete 删除文档
	AuditActionDocDelete AuditAction = 202
	// AuditActionDocDownload 下载文档
	AuditActionDocDownload AuditAction = 204
	// AuditActionDocApproval 审批通过
	AuditActionDocApproval AuditAction = 211
	// AuditActionDocDisapproval 审批拒绝
	AuditActionDocDisapproval AuditAction = 212
)

// AuditLog 添加审计日志
type AuditLog struct {
	Action     AuditAction
	EntityID   string
	EntityName string
	Old        string
	New        string
	Err        error
}
