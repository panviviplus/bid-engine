// Package company 公司相关操作
package company

import (
	"sync"

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

type Service interface {

	// AddCompany 添加公司
	AddCompanyRecord(c *gin.Context, company *model.Company) (err error)
	// GetCompany 获取公司信息
	GetCompany(c *gin.Context, companyID int32) (*model.Company, error)

	// GetCompanyByName 获取公司信息
	GetCompanyByName(c *gin.Context, name string) (*model.Company, error)

	// GetCompanyByNameBINARY 获取公司信息
	GetCompanyByNameBINARY(c *gin.Context, name string) (*model.Company, error)

	// ListCompany 分页查询公司列表
	ListCompany(c *gin.Context, pageSize, pageNum int, name string) ([]*model.Company, int64, error)
	// ListCompanyWithoutPage 不分页查询公司列表（按名称模糊筛选）
	ListCompanyWithoutPage(c *gin.Context, name string) ([]*model.Company, error)

	// DeleteCompany 删除公司
	DeleteCompany(c *gin.Context, companyID int32) error

	// UpdateCompany 更新公司信息
	UpdateCompany(c *gin.Context, companyID int32, userId int64) error

	// GetCompanyWithOwner 获取公司信息及负责人信息
	GetCompanyWithOwner(c *gin.Context, companyID int32) (*model.Company, *model.User, error)

	// ResetUserCompanyID 重置用户的公司ID为0
	ResetUserCompanyID(c *gin.Context, companyID int32) error

	// UpdateCompanyOwner 更新公司负责人
	UpdateCompanyOwner(c *gin.Context, companyID int32, ownerID int64) error

	// ListCompanyWithCounts 分页查询公司列表，包含资质和图片数量
	ListCompanyWithCounts(c *gin.Context, pageSize, pageNum int, companyID int32) ([]*entity.CompanyListItem, int64, error)

	// HasCompanyOwner 判断是否存在任意公司负责人为指定用户（已认证：status==1）
	HasCompanyOwner(c *gin.Context, ownerID int64) (bool, error)
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
