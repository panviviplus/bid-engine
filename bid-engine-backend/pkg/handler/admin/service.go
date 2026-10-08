package admin

import (
	"sync"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/pkg/repo/company"
	"bid-engine/pkg/repo/user"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/repo/oss"
	"bid-engine/lib/common/storage"
)

var (
	instance Service
	once     sync.Once
)

// Service 控制台服务需要实现的方法
type Service interface {

	// AddUser 添加用户
	AddUser(c *gin.Context)
	// ListUser 搜索用户
	ListUser(c *gin.Context)
	// UpdateUser 更新用户
	UpdateUser(c *gin.Context)
	// UploadExcel 上传excel批量创建用户
	UploadExcel(c *gin.Context)

	// 公司管理
	AddCompany(c *gin.Context)
	AddCompanyOwner(c *gin.Context)
	ListCompany(c *gin.Context)
	DeleteCompany(c *gin.Context)
	EditCompany(c *gin.Context)
	CompanyDetail(c *gin.Context)
	ListCompanyWithoutPage(c *gin.Context)

	// ListUserWithoutPage 不分页获取所有用户
	ListUserWithoutPage(c *gin.Context)
}

// GetInstance 获取服务实现实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger:    logtool.GetLogger().Sugar(),
			user:      user.GetInstance(),
			company:   company.GetInstance(),
			ossClient: oss.GetInstance(),
			db:        storage.GetDB(),
		}
	})
	return instance
}

type svcImpl struct {
	logger    *zap.SugaredLogger
	user      user.Service
	company   company.Service
	ossClient oss.Service
	db        *gorm.DB
}
