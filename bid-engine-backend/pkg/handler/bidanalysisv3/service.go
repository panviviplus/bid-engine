package bidanalysisv3

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	skbcfg "bid-engine/pkg/config"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	"bid-engine/pkg/repo/docling"
	repollm "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/oss"
	"bid-engine/pkg/repo/redis"
	"bid-engine/pkg/repo/taskqueue"
)

const boundedCleanupTimeout = 5 * time.Second

const (
	queueType             = "tender_parse_v3"
	blueprintQueueType    = "tender_blueprint_v3"
	llmFeatureFactExtract = "bid_analysis_fact_extract"
	llmFeatureConsolidate = "bid_analysis_fact_consolidate"
	llmFeatureChapters    = "bid_analysis_chapter_identify"
	llmFeatureSummary     = "tender_analysis_summary"
	llmFeatureBlueprint   = "bid_analysis_blueprint"
	llmFeatureAIInterpret = "bid_analysis_ai_interpret"
)

type Service struct {
	logger             *zap.SugaredLogger
	repo               *repov3.Repository
	llm                repollm.Service
	oss                oss.Service
	docling            docling.Service
	redis              redis.Service
	queue              *taskqueue.Repo
	doclingConcurrency int
	llmConcurrency     int
	chunkSplitBytes    int
	llmInputCeiling    int
	llmSem             chan struct{}
}

var (
	instance *Service
	once     sync.Once
)

func GetInstance() *Service {
	once.Do(func() {
		r := redis.GetInstance()
		llmConcurrency := configInt("bid_analysis_v3.llm_concurrency", 8, 1, 8)
		instance = &Service{
			logger: logtool.GetLogger().Sugar(), repo: repov3.New(), llm: repollm.GetInstance(),
			oss: oss.GetInstance(), docling: docling.GetInstance(), redis: r, queue: taskqueue.NewRepo(r),
			doclingConcurrency: configInt("bid_analysis_v3.docling_concurrency", 2, 1, 8),
			llmConcurrency:     llmConcurrency,
			llmInputCeiling:    configInt("bid_analysis_v3.llm_input_chunk_ceiling", defaultLLMInputChunkCeiling, llmMinInputBudget, 180000),
			llmSem:             make(chan struct{}, llmConcurrency),
			// 单块 PDF 超过该字节数时 SplitPDFByRanges 会递归二分（默认 1MB：
			// 10 页块超过 1MB 即拆为 5 页左右，降低 Docling 单块解析耗时与 504 概率）。
			chunkSplitBytes: configInt("bid_analysis_v3.chunk_split_bytes", 1<<20, 1, 200<<20),
		}
	})
	return instance
}

func (s *Service) acquireLLMPermit(ctx context.Context) (func(), error) {
	if s.llmSem == nil {
		return func() {}, nil
	}
	select {
	case s.llmSem <- struct{}{}:
		return func() { <-s.llmSem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func runWithBoundedCleanupContext(ctx context.Context, run func(context.Context) error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), boundedCleanupTimeout)
	defer cancel()
	return run(cleanupCtx)
}

func runInterpretationPersistenceBatch(ctx context.Context, writes []func(context.Context) error) error {
	if len(writes) == 0 {
		return nil
	}
	return runWithBoundedCleanupContext(ctx, func(writeCtx context.Context) error {
		failures := make([]error, 0, len(writes))
		for _, write := range writes {
			if write == nil {
				continue
			}
			if err := write(writeCtx); err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	})
}

func configInt(key string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(skbcfg.Get(key)))
	if err != nil || value < minValue || value > maxValue {
		return fallback
	}
	return value
}
