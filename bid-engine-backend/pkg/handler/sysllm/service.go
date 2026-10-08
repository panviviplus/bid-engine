// Package sysllm 提供“系统管理 → 系统模型配置”接口。
//
// 全局模型配置（system_llm_config）供平台级后台任务使用（例如招标情报站的公告打标），
// 与用户级 user_llm_config 完全隔离，仅系统超管可维护。
package sysllm

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/sysllm"
)

// Service 系统模型配置接口。
type Service interface {
	List(c *gin.Context)           // 配置列表（api_key 脱敏）
	Create(c *gin.Context)         // 新增配置
	Update(c *gin.Context)         // 修改配置
	Delete(c *gin.Context)         // 删除配置
	Reorder(c *gin.Context)        // 批量调整顺序
	TestConnection(c *gin.Context) // 连通性测试
	Resolve(c *gin.Context)        // 查看当前生效的配置（脱敏）
}

type svcImpl struct {
	logger *zap.SugaredLogger
	repo   sysllm.Service
	llm    repoLLM.Service
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 返回系统模型配置服务单例。
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			repo:   sysllm.GetInstance(),
			llm:    repoLLM.GetInstance(),
		}
	})
	return instance
}
