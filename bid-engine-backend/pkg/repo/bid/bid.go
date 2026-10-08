package bid

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
)

var (
	once        sync.Once
	svcInstance *svcImpl
)

// Service 投标项目相关数据库操作（仅保留 V2 终审确认创建 + dashboard 统计所需）
type Service interface {
	// AddBidProject 新增投标生成项目（V2 终审确认创建标书项目）
	AddBidProject(ctx *gin.Context, p *model.BidProject) error
	// CountBidProjects 按用户与创建时间范围统计 bid_project 数量（dashboard“本月生成”卡片）
	CountBidProjects(ctx *gin.Context, userID int64, startTime, endTime int64) (int64, error)
}

// GetInstance 获取服务实例
func GetInstance() Service {
	once.Do(func() {
		svcInstance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return svcInstance
}

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}
