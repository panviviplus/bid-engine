package open

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/pkg/repo/user"
	"bid-engine/lib/common/logtool"
)

var (
	instance Service
	once     sync.Once
)

// Service 控制台服务需要实现的方法
type Service interface {
	// GetUsers 批量获取用户信息
	GetUsers(c *gin.Context)
}

// GetInstance 获取服务实现实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			user:   user.GetInstance(),
		}
	})
	return instance
}

type svcImpl struct {
	logger *zap.SugaredLogger
	user   user.Service
}
