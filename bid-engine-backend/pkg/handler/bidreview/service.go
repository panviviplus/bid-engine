package bidreview

import (
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	bidanalysisRepo "bid-engine/pkg/repo/bidanalysisv3"
	bidgenRepo "bid-engine/pkg/repo/bidgen"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
	"bid-engine/pkg/repo/docling"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/oss"
	"bid-engine/pkg/repo/pdf"
	"bid-engine/pkg/repo/redis"
	"bid-engine/pkg/repo/taskqueue"

	skbcfg "bid-engine/pkg/config"
)

// Service 投标书审核模块业务接口
type Service interface {
	// ===== 项目管理 =====
	CreateProject(c *gin.Context)       // 上传招/投文件创建（可关联招标解析项目）
	CreateFromGen(c *gin.Context)       // 从标书生成项目发起
	PageListProject(c *gin.Context)     // 项目列表（含三维结论摘要）
	GetProjectDetail(c *gin.Context)    // 项目详情（项目 + 文件 + 维度 + 清单 + 判定 + 证据 + 整改 + 阶段）
	DeleteProject(c *gin.Context)       // 删除项目（级联清理）
	RetryStage(c *gin.Context)          // 阶段断点重跑（重置该阶段及后续）
	CancelProject(c *gin.Context)       // 取消执行中的审核任务
	UpdateAnonymousFlag(c *gin.Context) // 暗标评审开关
	GetSourcePdf(c *gin.Context)        // 原文 PDF 流（溯源预览）

	// ===== 清单项 =====
	AddChecklistItem(c *gin.Context)     // 自定义检查项
	UpdateChecklistItem(c *gin.Context)  // 人工确认/驳回 + 备注
	RecheckChecklistItem(c *gin.Context) // 单项/整批复检
	DeleteChecklistItem(c *gin.Context)  // 删除检查项（用户自定义/手工新增）

	// ===== 整改闭环 =====
	UpdateRemediation(c *gin.Context) // 整改状态/责任人/备注

	// ===== 企业规则库 =====
	ListRules(c *gin.Context)
	SaveRule(c *gin.Context)
	DeleteRule(c *gin.Context)
	SaveRuleFromItem(c *gin.Context) // 把清单项沉淀为规则

	// ===== 导出 =====
	ExportReport(c *gin.Context) // 导出审核报告（多 sheet Excel）
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 获取业务单例
func GetInstance() Service {
	once.Do(func() {
		llmConcurrency := configInt("bid_review.llm_concurrency", 4, 1, 16)
		instance = &svcImpl{
			logger:           logtool.GetLogger().Sugar(),
			repo:             bidreviewRepo.GetInstance(),
			analysisRepo:     bidanalysisRepo.New(),
			genRepo:          bidgenRepo.GetInstance(),
			llm:              repoLLM.GetInstance(),
			oss:              oss.GetInstance(),
			docling:          docling.GetInstance(),
			pdf:              pdf.GetInstance(),
			redisSvc:         redis.GetInstance(),
			taskqueueRepo:    taskqueue.NewRepo(redis.GetInstance()),
			llmConcurrency:   llmConcurrency,
			llmSem:           make(chan struct{}, llmConcurrency),
			maxOutputCeiling: configInt("bid_review.max_output_ceiling", 16384, 512, 65536),
			retrievalTopK:    configInt("bid_review.retrieval_top_k", 8, 1, 40),
			chunkSize:        configInt("bid_review.chunk_chars", 1800, 400, 6000),
			chunkOverlap:     configInt("bid_review.chunk_overlap_chars", 200, 0, 1000),
		}
	})
	return instance
}

type svcImpl struct {
	logger        *zap.SugaredLogger
	repo          bidreviewRepo.Service
	analysisRepo  *bidanalysisRepo.Repository
	genRepo       bidgenRepo.Service
	llm           repoLLM.Service
	oss           oss.Service
	docling       docling.Service
	pdf           pdf.Service
	redisSvc      redis.Service
	taskqueueRepo *taskqueue.Repo

	// 审核链路调优参数（配置项见 conf-local.yml / conf-container.yml）
	llmConcurrency   int
	llmSem           chan struct{}
	maxOutputCeiling int
	retrievalTopK    int
	chunkSize        int
	chunkOverlap     int
}

// configInt 读取整型配置（越界回退默认值）
func configInt(key string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(skbcfg.Get(key)))
	if err != nil || value < minValue || value > maxValue {
		return fallback
	}
	return value
}
