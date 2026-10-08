// Package tenderintel 实现“招标情报站”模块。
//
// 模块由三段组成：
//   - 采集：cron 轮次 → tender_intel_collect 队列 → 调用独立采集服务（tender-collection）
//   - 理解：tender_intel_enrich 队列 → 规则抽字段 + 全局模型批量打标 → 入库
//   - 分发：订阅匹配 → 站内提醒；情报大厅检索、收藏、AI 解读与招标解析联动
package tenderintel

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/redis"
	"bid-engine/pkg/repo/sysllm"
	"bid-engine/pkg/repo/taskqueue"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

// Service 招标情报站业务接口。
type Service interface {
	// ===== 情报大厅 =====
	ListNotices(c *gin.Context)     // 公告列表（关键词/行业/地区/类型/时间/预算/来源/收藏筛选）
	GetNotice(c *gin.Context)       // 公告详情
	SetFavorite(c *gin.Context)     // 收藏/取消收藏
	Filters(c *gin.Context)         // 筛选项元数据（行业、地区、公告类型、来源）
	GenerateInsight(c *gin.Context) // 公告 AI 解读（带全局缓存）
	ParseLink(c *gin.Context)       // 生成“发起招标解析”预填参数

	// ===== 订阅与提醒 =====
	ListSubscriptions(c *gin.Context)
	CreateSubscription(c *gin.Context)
	UpdateSubscription(c *gin.Context)
	DeleteSubscription(c *gin.Context)
	SetSubscriptionEnabled(c *gin.Context)
	ParseSubscription(c *gin.Context) // 自然语言 → 结构化草稿（不落库）
	ListAlerts(c *gin.Context)
	SetAlertsStatus(c *gin.Context) // 标记已读 / 未读（单条、批量、全部已读）
	DeleteAlerts(c *gin.Context)    // 删除提醒（硬删除）
	UnreadCount(c *gin.Context)

	// ===== 采集运维（超管）=====
	ListSources(c *gin.Context)
	UpdateSource(c *gin.Context)           // 编辑采集源（名称/地址/优先级/参数/启停等）
	DeleteSource(c *gin.Context)           // 删除采集源
	ProbeSource(c *gin.Context)            // 采集源联通性探测（调用采集服务 /discover）
	DownloadSourceTemplate(c *gin.Context) // 下载采集源导入模板
	ImportSources(c *gin.Context)          // 上传采集源表格并落库
	ListRuns(c *gin.Context)
	GetRunDetail(c *gin.Context)
	RetryRun(c *gin.Context)  // 重试历史批次（按原范围重新发起）
	DeleteRun(c *gin.Context) // 删除批次及其源明细
	TriggerCollect(c *gin.Context)

	// ===== 自动采集任务配置（超管）=====
	GetSchedule(c *gin.Context)
	UpdateSchedule(c *gin.Context)

	// ===== 订阅匹配作业面（超管）=====
	MatchOverview(c *gin.Context)        // 匹配任务概览
	ListMatchTasks(c *gin.Context)       // 匹配任务列表
	GetMatchTask(c *gin.Context)         // 匹配任务详情
	SetMatchTaskPriority(c *gin.Context) // 调整排队中任务的优先级
	CancelMatchTask(c *gin.Context)      // 取消任务（pending 出队 / running 协作式取消）
	RetryMatchTask(c *gin.Context)       // 用同一份固定范围重新入队
	DeleteMatchTask(c *gin.Context)      // 删除终态任务
	RescanMatches(c *gin.Context)        // 按时间窗手动补扫

	// ===== 情报管理（超管）=====
	ListAdminNotices(c *gin.Context)       // 情报管理列表（含隐藏与下架）
	CreateAdminNotice(c *gin.Context)      // 手工发布一条情报（来源=系统录入）
	UpdateAdminNotice(c *gin.Context)      // 编辑已入库情报
	SetNoticeStatus(c *gin.Context)        // 隐藏 / 下架 / 恢复（支持批量）
	SetNoticePinned(c *gin.Context)        // 置顶 / 取消置顶（支持批量）
	DeleteAdminNotices(c *gin.Context)     // 删除情报及其派生数据（支持批量）
	DownloadNoticeTemplate(c *gin.Context) // 下载情报导入模板
	ImportNotices(c *gin.Context)          // 上传情报表格并落库

	// ===== 后台调度 =====
	// StartCollectRound 生成一轮采集批次并投递各源采集任务。
	StartCollectRound(ctx context.Context, triggerType string, sourceKeys []string) (string, error)
	// CountTodayNew 今日新增公告数（首页卡片）。
	CountTodayNew(ctx context.Context) (int64, error)
	// UnreadAlertsFor 指定用户未读提醒数（首页卡片）。
	UnreadAlertsFor(ctx context.Context, userID int64) (int64, error)
	// SubscriptionCountFor 指定用户订阅数（首页卡片）。
	SubscriptionCountFor(ctx context.Context, userID int64) (int64, error)
}

type svcImpl struct {
	logger    *zap.SugaredLogger
	repo      *intelRepo.Repository
	sysllm    sysllm.Service
	llm       repoLLM.Service
	collector *collectorClient
	queue     *taskqueue.Repo
	cache     redis.Service
}

var (
	instance Service
	once     sync.Once
)

// GetInstance 返回招标情报站业务单例。
func GetInstance() Service {
	once.Do(func() {
		instance = newService()
	})
	return instance
}

func newService() *svcImpl {
	return &svcImpl{
		logger:    logtool.GetLogger().Sugar(),
		repo:      intelRepo.New(),
		sysllm:    sysllm.GetInstance(),
		llm:       repoLLM.GetInstance(),
		collector: newCollectorClient(),
		queue:     taskqueue.NewRepo(redisInstance()),
		cache:     redisInstance(),
	}
}

// redisInstance 返回 Redis 单例，供任务队列使用。
func redisInstance() redis.Service {
	return redis.GetInstance()
}

// 配置项与常量集中在此，避免散落的硬编码。
var (
	// collectMaxItemsPerSource 每源每轮抽取条数上限。
	collectMaxItemsPerSource = 50
	// collectMaxPagesPerSource 每源每轮翻页上限。
	collectMaxPagesPerSource = 1
	// enrichBatchSize 打标批次大小（与提示词配合控制 token 消耗）。
	enrichBatchSize = tagBatchSize
	// retentionDays 公告保留天数。
	retentionDays = 180
	// staleRunAfter 批次超过该时长仍未结束即视为卡死（worker 未消费/采集服务不可用），
	// 触发新一轮前会自动收尾为失败，避免永久阻塞后续采集。
	staleRunAfter = 30 * time.Minute
)

// NowFunc 便于测试替换时间源。
var NowFunc = func() time.Time { return time.Now() }
