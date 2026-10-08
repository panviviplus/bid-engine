package llmconfig

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
)

// Service 系统管理-模型配置（独立功能模块）
type Service interface {
	// Modules 获取业务功能模块列表（不含全局 all）
	Modules(c *gin.Context)
	// Get 获取当前用户 LLM 配置（脱敏返回，未配置为 null）
	Get(c *gin.Context)
	// SaveModule 保存单个模块配置（module=all 或具体业务模块；全量覆盖 upsert）
	SaveModule(c *gin.Context)
	// ClearModule 清空单个模块配置
	ClearModule(c *gin.Context)
	// Clear 清空当前用户全部 LLM 配置
	Clear(c *gin.Context)
	// TestConnection 测试提交的 LLM 配置是否可用（真实调用，不落库）
	TestConnection(c *gin.Context)
	// Exist 校验模块 LLM 配置是否存在（纯 DB 查询，无 LLM 调用）
	Exist(c *gin.Context)
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 获取服务实现实例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{logger: logtool.GetLogger().Sugar()}
	})
	return instance
}

type svcImpl struct {
	logger *zap.SugaredLogger
}
