package entity

import "fmt"

// ErrCode 错误码定义
// 0 ~ 199 通用错误码
// 200~999 对应http状态码
// 1000~1999 模型错误
// 2000~2999 服务错误
type ErrCode = int32

// ErrMsg 通用的错误消息
type ErrMsg = string

const (
	// ErrCodeNotLogin 未登录
	ErrCodeNotLogin ErrCode = -1
	// ErrCodeUserLocked 用户被禁用
	ErrCodeUserLocked ErrCode = -2
	// ErrCodeNotInvite 用户未被邀请,无平台使用权限
	ErrCodeNotInvite ErrCode = -3
	// ErrCodeJoinCompany 用户已申请加入公司
	ErrCodeJoinCompany ErrCode = -4
	// ErrCodeCreateCompany 用户已申请创建公司
	ErrCodeCreateCompany ErrCode = -5

	// ErrCodeOK 正常
	ErrCodeOK ErrCode = 0
	// ErrCodeParam 参数错误
	ErrCodeParam ErrCode = 1
	// ErrCodeInputSensitive 输入内容包含敏感内容
	ErrCodeInputSensitive ErrCode = 3
	// ErrCodeDB 数据库通用错误
	ErrCodeDB ErrCode = 10
	// ErrCodeDBWrite 写数据库失败
	ErrCodeDBWrite ErrCode = 11
	// ErrCodeDBRead 读数据库失败
	ErrCodeDBRead ErrCode = 12
	// ErrCodeRDBWrite 写redis失败
	ErrCodeRDBWrite ErrCode = 13
	// ErrCodeRDBRead 读redis失败
	ErrCodeRDBRead ErrCode = 14
	// ErrCodeES es错误
	ErrCodeES ErrCode = 20
	// ErrCodeGeneral 通用接口操作拦截错误
	ErrCodeGeneral ErrCode = 33

	// ErrCodeS3 S3 存储通用错误
	ErrCodeS3 ErrCode = 30
	// ErrCodeOSSWrite 写S3错误
	ErrCodeOSSWrite ErrCode = 31
	// ErrCodeS3Read 读S3错误
	ErrCodeS3Read ErrCode = 32

	// ErrCodeNoAuth 认证未通过
	ErrCodeNoAuth ErrCode = 401
	// ErrCodeForbidden 无权限
	ErrCodeForbidden ErrCode = 403
	// ErrMsgForbidden 无权限文案
	ErrMsgForbidden ErrMsg = "无权限"

	// ErrCodeNotFound 不存在
	ErrCodeNotFound ErrCode = 404
	// ErrCodeFileTooLarge 文档过大
	ErrCodeFileTooLarge ErrCode = 413
	// ErrCodeRateLimit 频率限制
	ErrCodeRateLimit ErrCode = 429

	// ErrCodeInternal 内部错误
	ErrCodeInternal ErrCode = 500

	// ErrMsgInternal 内部错误
	ErrMsgInternal ErrMsg = "内部错误"

	// ErrCodeModel 模型相关错误
	ErrCodeModel ErrCode = 1000

	// ErrCodeFileNotSupport 文档类型不支持
	ErrCodeFileNotSupport ErrCode = 2001
	// ErrCodeFileParse 文档解析失败
	ErrCodeFileParse ErrCode = 2002
	// ErrCodeDocNotExist 文档不存在
	ErrCodeDocNotExist = 2003
	// ErrCodeDocLimit 额度不足
	ErrCodeDocLimit = 2004
	// ErrCodeConversationNotExist 会话不存在
	ErrCodeConversationNotExist = 2005

	// ErrCodeQaExist qa存在
	ErrCodeQaExist = 2006

	// ErrCodeFileExist 文件存在
	ErrCodeFileExist = 2007

	// ErrMsgNoDoc 会话范围内无可问答的文档
	ErrMsgNoDoc ErrMsg = "会话范围内无可问答的文档"
)

var (
	// ErrDeleted 被删除错误
	ErrDeleted = fmt.Errorf("已经删除")
	// ErrForbidden 无权限
	ErrForbidden = fmt.Errorf("无权限")
)
