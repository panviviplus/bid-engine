package bidgen

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	bidanalysisv3 "bid-engine/pkg/handler/bidanalysisv3"
	bidgenRepo "bid-engine/pkg/repo/bidgen"
	"bid-engine/pkg/repo/docling"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/material"
	"bid-engine/pkg/repo/oss"
	"bid-engine/pkg/repo/pdf"
	"bid-engine/pkg/repo/redis"
	"bid-engine/pkg/repo/taskqueue"
)

// Service 标书生成模块业务接口
type Service interface {
	// ===== 项目管理 =====
	CreateProject(c *gin.Context)      // 空白标书
	CreateFromTender(c *gin.Context)   // 从招标文件创建（异步解析）
	CreateFromTemplate(c *gin.Context) // 从模板创建（异步解析）
	ConfirmOutline(c *gin.Context)     // 大纲确认
	PageListProject(c *gin.Context)
	GetProjectDetail(c *gin.Context)
	DeleteProject(c *gin.Context)      // 单个删除（级联清理）
	BatchDeleteProject(c *gin.Context) // 批量删除（级联清理）
	SaveDocContent(c *gin.Context)     // 自动保存

	// ===== 大纲 =====
	GetOutline(c *gin.Context)
	AddOutlineNode(c *gin.Context)
	UpdateOutlineNode(c *gin.Context)
	DeleteOutlineNode(c *gin.Context)
	ApplyOutline(c *gin.Context)        // 大纲结构快照（排序/层级，单事务）
	SyncOutline(c *gin.Context)         // 编辑器文档标题结构 → 大纲表对账
	UnconfirmOutline(c *gin.Context)    // 撤销大纲确认（draft → outline_review，仅未生成章节时）
	CompleteOutlineNode(c *gin.Context) // 人工标记章节写作完成/取消完成（重算整体状态）

	// ===== AI 生成（SSE）=====
	GenerateFull(c *gin.Context)
	GenerateChapter(c *gin.Context)
	SubscribeGenerateEvents(c *gin.Context)
	CancelGenerate(c *gin.Context)
	Rewrite(c *gin.Context) // AI 重写选中片段（终态审阅场景，无落库）

	// ===== 导出 =====
	ExportPDF(c *gin.Context)
	ExportPageMap(c *gin.Context) // 两遍导出：回读章节真实页码
	RecordExport(c *gin.Context)
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 获取业务单例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger:        logtool.GetLogger().Sugar(),
			repo:          bidgenRepo.GetInstance(),
			analysisSvc:   bidanalysisv3.GetInstance(),
			llm:           repoLLM.GetInstance(),
			oss:           oss.GetInstance(),
			docling:       docling.GetInstance(),
			mat:           material.GetInstance(),
			pdf:           pdf.GetInstance(),
			redisSvc:      redis.GetInstance(),
			taskqueueRepo: taskqueue.NewRepo(redis.GetInstance()),
			quotaResolver: configGenerationQuotaResolver{},
		}
	})
	return instance
}

type svcImpl struct {
	logger        *zap.SugaredLogger
	repo          bidgenRepo.Service
	analysisSvc   *bidanalysisv3.Service
	llm           repoLLM.Service
	oss           oss.Service
	docling       docling.Service
	mat           material.Service
	pdf           pdf.Service
	redisSvc      redis.Service
	taskqueueRepo *taskqueue.Repo
	quotaResolver GenerationQuotaResolver
}
