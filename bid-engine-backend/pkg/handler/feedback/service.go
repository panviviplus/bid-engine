package feedback

import (
	"sync"

	"bid-engine/pkg/repo/user"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/feedback"
	"bid-engine/pkg/repo/oss"
	"bid-engine/lib/common/logtool"
)

var (
	instance Service
	once     sync.Once
)

type Service interface {
	Options(c *gin.Context)
	SearchRecords(c *gin.Context)
	GetRecord(c *gin.Context)
	AddRecord(c *gin.Context)
	UpdateRecord(c *gin.Context)
	UpdateStatus(c *gin.Context)
	DeleteRecord(c *gin.Context)
	GetImageByKey(c *gin.Context)
}

type svcImpl struct {
	logger *zap.SugaredLogger
	repo   feedback.Service
	oss    oss.Service
	user   user.Service
}

func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			repo:   feedback.GetInstance(),
			oss:    oss.GetInstance(),
			user:   user.GetInstance(),
		}
	})
	return instance
}

// sendErr 统一错误返回
func (s *svcImpl) sendErr(c *gin.Context, msg string) {
	handler.SendNormalResp(c, 50001, msg, nil)
}
