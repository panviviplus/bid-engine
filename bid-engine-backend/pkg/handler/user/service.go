package user

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	"bid-engine/pkg/repo/company"
	"bid-engine/pkg/repo/oss"
	"bid-engine/pkg/repo/user"
	"bid-engine/pkg/sms"
)

var (
	instance Service
	once     sync.Once
)

// Service 控制台服务需要实现的方法
type Service interface {

	// Info 获取当前用户信息
	Info(c *gin.Context)
	// GetAvatar 获取当前用户头像文件
	GetAvatar(c *gin.Context)
	// Login 登录
	Login(c *gin.Context)
	// Logout 登出
	Logout(c *gin.Context)
	// SendSmsCode 发送短信验证码
	SendSmsCode(c *gin.Context)
	// Register 手机号注册（组织信息选填），注册成功自动登录
	Register(c *gin.Context)
	// UpdateProfile 更新当前用户资料（仅昵称/手机号/邮箱/头像）
	UpdateProfile(c *gin.Context)
	// ChangePassword 修改登录密码（独立于编辑资料）
	ChangePassword(c *gin.Context)
	// UploadAvatar 上传头像文件并返回 objectKey 与预览URL
	UploadAvatar(c *gin.Context)

	// JudgeCurUserIdentity 判断目标用户的身份：admin\company_owner\member
	JudgeCurUserIdentity(c *gin.Context, userId int64) string

	// CancelApply 取消申请/创建加入公司
	CancelApply(c *gin.Context)

	// AuthLogin 单点登陆回调，种票
	AuthLogin(c *gin.Context)

	// GetUserBySNFromIAM 根据SN码从IAM获取用户信息
	GetUserBySNFromIAM(c *gin.Context)

	// ListCompanyUser 不分页查询目标公司的用户列表
	ListCompanyUser(c *gin.Context)
}

// GetInstance 获取服务实现实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger:      logtool.GetLogger().Sugar(),
			user:        user.GetInstance(),
			company:     company.GetInstance(),
			oss:         oss.GetInstance(),
			smsProvider: sms.NewNoopProvider(),
			otpStore:    sms.NewOTPStore(),
		}
	})
	return instance
}

type svcImpl struct {
	logger      *zap.SugaredLogger
	user        user.Service
	company     company.Service
	oss         oss.Service
	smsProvider sms.Provider
	otpStore    *sms.OTPStore
}
