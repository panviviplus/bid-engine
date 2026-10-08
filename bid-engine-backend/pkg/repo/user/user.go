// Package user 用户相关操作
package user

import (
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
)

var (
	instance *svcImpl
	once     sync.Once
)

// UserInfo 用户基本信息（替代 pb.UserInfo）
type UserInfo struct {
	UserId   int64
	Username string
	Nickname string
	PhoneNum string
}

// Service 用户服务接口定义
type Service interface {

	// GetBaseInfo 获取用户基本信息
	GetBaseInfo(c *gin.Context, userID int64, mobile string) (*UserInfo, error)
	GetBaseInfoByUserName(c *gin.Context, userName string) (*UserInfo, error)

	// AddUser2Company 添加用户到公司
	AddUser2Company(c *gin.Context, u *model.User) error

	// SearchUsers 获取用户
	SearchUsers(c *gin.Context, param SearchParam) ([]*model.User, error)
	// GetUserNum 获取用户数量
	GetUserNum(c *gin.Context, param SearchParam) (int, error)
	// SelectUsers 查询用户列表
	SelectUsers(c *gin.Context, param entity.SearchUserParam) ([]*model.User, error)
	// GetUserCount 获取符合条件的用户数量
	GetUserCount(c *gin.Context, param entity.SearchUserParam) (int, error)
	// GetAllUser 获取所有状态为1的用户
	GetAllUser(c *gin.Context) ([]*model.User, error)

	// GetActiveUsersByKeyword 获取所有状态为1且按关键字匹配的用户（昵称或手机号模糊匹配）
	GetActiveUsersByKeyword(c *gin.Context, keyword string) ([]*model.User, error)

	// GetUsers 获取用户列表
	GetUsers(c *gin.Context, userIDs []int64) ([]*model.User, error)

	// GetUser 获取用户信息
	GetUser(c *gin.Context, userID int64) (*model.User, error)

	// GetUserByUsername 根据用户名获取用户
	GetUserByUsername(c *gin.Context, username string) (*model.User, error)

	// GetUserByMobile 根据手机号获取用户
	GetUserByMobile(c *gin.Context, mobile string) (*model.User, error)

	CheckAndAddUser(c *gin.Context, profile *entity.AuthUser) (*model.User, error)

	// GetFirstAdminCompanyUser 获取用户信息
	GetFirstAdminCompanyUser(c *gin.Context, companyID int32) (*model.User, error)

	// GetCompanyUser 获取公司用户信息
	GetCompanyUser(c *gin.Context, companyID int32) ([]*model.User, error)

	// AddUser 直接添加用户到数据库
	AddUser(c *gin.Context, u *model.User) error

	// UpdateUser 更新用户（全字段保存）
	UpdateUser(c *gin.Context, u *model.User) error

	// UpdateUserFields 精确更新用户资料字段（昵称/手机号/邮箱/头像/更新时间），避免全表覆写
	UpdateUserFields(c *gin.Context, userID int64, nickname, mobile, email, avatarFile string, updateTime int64) error

	// UpdateUserStatus 更新用户状态
	UpdateUserStatus(c *gin.Context, userID int64, status int32) error

	// DeleteUser 删除用户
	DeleteUser(c *gin.Context, userID int64, pubType int32) error

	// DisableUser 禁用用户
	DisableUser(c *gin.Context, userID int64) error

	// EnableUser 启用用户
	EnableUser(c *gin.Context, userID int64) error


}

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

// GetInstance 创建Term的实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return instance
}
