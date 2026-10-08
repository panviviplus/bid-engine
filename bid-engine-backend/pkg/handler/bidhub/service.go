package bidhub

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	bidanalysisRepo "bid-engine/pkg/repo/bidanalysisv3"
	bidgenRepo "bid-engine/pkg/repo/bidgen"
)

var (
	instance Service
	once     sync.Once
)

type Service interface {
	// GetAnalysisStats 获取招标解析 V3 项目统计（dashboard“今日解析”卡片）
	GetAnalysisStats(c *gin.Context)
	// GetGenerationStats 获取投标生成项目统计（dashboard“本月生成”卡片）
	GetGenerationStats(c *gin.Context)
}

type svcImpl struct {
	logger          *zap.SugaredLogger
	bidgenRepo      bidgenRepo.Service
	bidanalysisRepo *bidanalysisRepo.Repository
}

// GetInstance 获取实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger:          logtool.GetLogger().Sugar(),
			bidgenRepo:      bidgenRepo.GetInstance(),
			bidanalysisRepo: bidanalysisRepo.New(),
		}
	})
	return instance
}
