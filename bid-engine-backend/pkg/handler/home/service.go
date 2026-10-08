package home

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	homerepo "bid-engine/pkg/repo/home"
)

type Service interface {
	Stats(c *gin.Context)
	RecentWork(c *gin.Context)
	Attention(c *gin.Context)
	LLMConfigStatus(c *gin.Context)
}

type svcImpl struct {
	logger *zap.SugaredLogger
	repo   *homerepo.Repository
}

var (
	instance Service
	once     sync.Once
)

func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			repo:   homerepo.New(),
		}
	})
	return instance
}
