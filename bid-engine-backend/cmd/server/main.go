package main

import (
	"context"
	"os"

	commoncfg "bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/handler/bidanalysisv3"
	"bid-engine/pkg/handler/bidgen"
	"bid-engine/pkg/handler/bidreview"
	feishuHandler "bid-engine/pkg/handler/feishu"
	intelHandler "bid-engine/pkg/handler/tenderintel"
	"bid-engine/pkg/middleware"
	"bid-engine/pkg/repo/redis"
	"bid-engine/pkg/repo/taskqueue"
	"bid-engine/pkg/router"
	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

func init() {
	// 设置时区
	_ = os.Setenv("TZ", "Asia/Shanghai")
	// 初始化配置
	initConfig()
	// 初始化日志
	logtool.MustInitLogger()
	logger = logtool.GetLogger()
	// 初始化数据库
	logger.Sugar().Info("初始化数据库...")
	storage.MustInitDB()
	// 初始化环境
	middleware.Init()
	// 初始化 Redis
	logger.Sugar().Info("初始化 Redis...")
	_ = redis.GetInstance()
}

func initConfig() {
	var configFile string
	if os.Getenv("CONF_FILE") != "" {
		configFile = os.Getenv("CONF_FILE")
	} else {
		candidates := []string{"/service/conf/conf.yml", "./conf/conf-local.yml", "/conf/conf.yml"}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				configFile = p
				break
			}
		}
	}

	if configFile != "" {
		commoncfg.MustLoadConfig(configFile)
		_ = os.Setenv("BID_ENGINE_CONF_FILE", configFile)
	}
}

func main() {
	logger.Sugar().Infow("开始启动HTTP服务...")
	e := gin.Default()
	// 添加requestID
	e.Use(requestid.New())
	// 访问控制
	e.Use(middleware.AccessControl())
	// 日志
	e.Use(ginzap.Ginzap(logger, "2006-01-02T15:04:05.000Z0700", false))
	e.Use(ginzap.RecoveryWithZap(logger, true))
	// 打印请求和返回日志
	e.Use(middleware.LogReqAndResp())
	// router
	router.RegisterRouter(e)
	go feishuHandler.GetInstance().Start(context.Background())

	// 加载队列配置
	taskqueue.QueueConfigs = taskqueue.LoadQueueConfigs()

	// 启动 Redis 任务队列 Worke（每个模块独立消费）
	redisSvc := redis.GetInstance()
	// 启动招标解析 V3 Worker。每个项目内部 Docling/LLM 并发由 V3 流水线单独控制。
	v3Service := bidanalysisv3.GetInstance()
	v3Cfg := taskqueue.QueueConfigs["tender_parse_v3"]
	v3Worker := taskqueue.NewWorker(v3Cfg, redisSvc, &bidanalysisv3.ParseTaskHandler{Svc: v3Service}, logtool.GetLogger().Sugar())
	v3Worker.SetOnTaskFinal(func(ctx context.Context, task *taskqueue.Task, _ bool) {
		v3Service.HandleTaskFinal(ctx, task)
	})
	go v3Worker.Run(context.Background())
	go v3Service.StartDeletionWorker(context.Background())
	go v3Service.StartControlSweeper(context.Background())

	// 标书蓝图仅在用户主动发起后生成，使用独立队列，不影响招标解析项目状态。
	blueprintCfg := taskqueue.QueueConfigs["tender_blueprint_v3"]
	if blueprintCfg.QueueKey != "" {
		blueprintWorker := taskqueue.NewWorker(blueprintCfg, redisSvc, &bidanalysisv3.BlueprintTaskHandler{Svc: v3Service}, logtool.GetLogger().Sugar())
		blueprintWorker.SetOnTaskFinal(func(ctx context.Context, task *taskqueue.Task, _ bool) {
			v3Service.HandleTaskFinal(ctx, task)
		})
		go blueprintWorker.Run(context.Background())
	}

	// 启动标书生成解析 Worker（从招标文件/模板创建时入队）
	bidGenCfg := taskqueue.QueueConfigs["bid_gen_parse"]
	if bidGenCfg.QueueKey != "" {
		bidGenService := bidgen.GetInstance()
		bidGenHandler := &bidgen.BidGenParseHandler{Svc: bidGenService}
		bidGenWorker := taskqueue.NewWorker(bidGenCfg, redisSvc, bidGenHandler, logtool.GetLogger().Sugar())
		if impl, ok := bidGenService.(interface {
			HandleParseTaskFinal(context.Context, *taskqueue.Task, bool)
		}); ok {
			bidGenWorker.SetOnTaskFinal(impl.HandleParseTaskFinal)
		}
		go bidGenWorker.Run(context.Background())
	}

	// 标书正文生成使用独立持久队列，浏览器断开不会终止后台 LLM 任务。
	generateCfg := taskqueue.QueueConfigs["bid_gen_generate"]
	if generateCfg.QueueKey != "" {
		bidGenService := bidgen.GetInstance()
		generateWorker := taskqueue.NewWorker(generateCfg, redisSvc, &bidgen.BidGenGenerateTaskHandler{Svc: bidGenService}, logtool.GetLogger().Sugar())
		if impl, ok := bidGenService.(interface {
			HandleGenerationTaskFinal(context.Context, *taskqueue.Task, bool)
			StartGenerationReconciler(context.Context)
		}); ok {
			generateWorker.SetOnTaskFinal(impl.HandleGenerationTaskFinal)
			go impl.StartGenerationReconciler(context.Background())
		}
		go generateWorker.Run(context.Background())
	}

	// 启动投标书审核 Worker
	bidReviewCfg := taskqueue.QueueConfigs["bid_review"]
	if bidReviewCfg.QueueKey != "" {
		bidReviewHandler := &bidreview.ReviewTaskHandler{Svc: bidreview.GetInstance()}
		bidReviewWorker := taskqueue.NewWorker(bidReviewCfg, redisSvc, bidReviewHandler, logtool.GetLogger().Sugar())
		go bidReviewWorker.Run(context.Background())
	}

	// 启动招标情报站 Worker（采集 + 打标两个队列），并注册定时采集与保留期清理任务
	intelService := intelHandler.GetInstance()
	if collectCfg := taskqueue.QueueConfigs["tender_intel_collect"]; collectCfg.QueueKey != "" {
		collectHandler := &intelHandler.CollectTaskHandler{Svc: intelService}
		collectWorker := taskqueue.NewWorker(collectCfg, redisSvc, collectHandler, logtool.GetLogger().Sugar())
		// 采集任务重试耗尽时兜底收尾批次，避免批次永久停在 running 阻塞后续触发
		collectWorker.SetOnTaskFinal(collectHandler.HandleTaskFinal)
		go collectWorker.Run(context.Background())
	}
	if enrichCfg := taskqueue.QueueConfigs["tender_intel_enrich"]; enrichCfg.QueueKey != "" {
		enrichHandler := &intelHandler.EnrichTaskHandler{Svc: intelService}
		enrichWorker := taskqueue.NewWorker(enrichCfg, redisSvc, enrichHandler, logtool.GetLogger().Sugar())
		enrichWorker.SetOnTaskFinal(enrichHandler.HandleTaskFinal)
		go enrichWorker.Run(context.Background())
	}
	// 订阅匹配独立成队列：与采集、打标互不阻塞
	if matchCfg := taskqueue.QueueConfigs["tender_intel_match"]; matchCfg.QueueKey != "" {
		matchHandler := &intelHandler.MatchTaskHandler{Svc: intelService}
		matchWorker := taskqueue.NewWorker(matchCfg, redisSvc, matchHandler, logtool.GetLogger().Sugar())
		// 重试耗尽时把任务账本落到失败态，避免永远停在执行中
		matchWorker.SetOnTaskFinal(matchHandler.HandleTaskFinal)
		go matchWorker.Run(context.Background())
	}
	if err := intelHandler.RegisterSchedule(intelService); err != nil {
		logger.Sugar().Warnw("注册招标情报站定时采集失败", "err", err)
	}
	if err := intelHandler.StartCleanupSchedule(intelService); err != nil {
		logger.Sugar().Warnw("注册招标情报站保留期清理失败", "err", err)
	}

	// 启动全局延迟任务回收器 + 僵尸扫描器
	go taskqueue.RunDelayedReclaimer(context.Background(), redisSvc, logtool.GetLogger().Sugar())
	go taskqueue.RunZombieScanner(context.Background(), redisSvc, logtool.GetLogger().Sugar())

	// 启动web服务
	if err := e.Run(":" + skbcfg.Get("server.http_port")); err != nil {
		logger.Sugar().Fatalw("启动HTTP服务失败", "err", err)
	}
}
