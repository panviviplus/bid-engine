package feedback

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
)

// Service 反馈记录仓库接口
type Service interface {
	// AddRecord 新增反馈记录
	AddRecord(c *gin.Context, r *model.FeedbackRecord) error
	// UpdateRecord 更新反馈记录
	UpdateRecord(c *gin.Context, r *model.FeedbackRecord) error
	// UpdateDescription 仅更新反馈描述
	UpdateDescription(c *gin.Context, id int64, description string) error
	UpdateDescriptionForUser(ctx context.Context, userID, id int64, description string) error
	// UpdateStatus 仅更新反馈状态
	UpdateStatus(c *gin.Context, id int64, status int32) error
	// DeleteRecord 删除反馈记录（物理删除）
	DeleteRecord(c *gin.Context, id int64) error
	DeleteRecordForUser(ctx context.Context, userID, id int64) error
	// GetRecord 获取单条反馈记录
	GetRecord(c *gin.Context, id int64) (*model.FeedbackRecord, error)
	GetRecordForUser(ctx context.Context, userID, id int64) (*model.FeedbackRecord, error)
	HasPhotoKeyForUser(ctx context.Context, userID int64, objectKey string) (bool, error)
	// SearchRecords 条件查询反馈记录（分页）
	SearchRecords(c *gin.Context, param SearchParam) ([]*model.FeedbackRecord, int64, error)
	// GetRecordNum 统计数量（配合分页或导出）
	GetRecordNum(c *gin.Context, param SearchParam) (int64, error)
	// GetTypes 获取反馈类型选项
	GetTypes(c *gin.Context) ([]string, error)
	GetTypesForUser(ctx context.Context, userID int64) ([]string, error)
}

var (
	instance Service
	once     sync.Once
)

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

// GetInstance 获取实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return instance
}
