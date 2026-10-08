package tenderintel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"bid-engine/pkg/middleware/schedule"
	"bid-engine/pkg/repo/redis"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

// 默认采集节奏：每天 06:00 自动采集一轮。
// 管理员可在“系统管理 → 招标情报管理 → 采集任务配置”里改成任意秒级 cron。
const defaultCollectCron = intelRepo.DefaultCollectCron

// redisLockKey 保证多实例部署时同一时刻只触发一轮采集。
const redisLockKey = "lock:tender_intel:collect_round"

const redisLockTTL = 10 * time.Minute

// cronParser 秒级 cron（秒 分 时 日 月 周），与 schedule 调度器的解析口径一致。
var cronParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// NormalizeCronExpr 校验 cron 表达式，返回去除首尾空格后的结果。
func NormalizeCronExpr(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", fmt.Errorf("cron 表达式不能为空")
	}
	if _, err := cronParser.Parse(expr); err != nil {
		return "", fmt.Errorf("cron 表达式非法：%v（格式为 秒 分 时 日 月 周，每天 06:00 写作 0 0 6 * * *）", err)
	}
	return expr, nil
}

// NextRunTimes 返回接下来 n 次触发时间（前端预览用）。
func NextRunTimes(expr string, n int) []string {
	sched, err := cronParser.Parse(strings.TrimSpace(expr))
	if err != nil || n <= 0 {
		return nil
	}
	out := make([]string, 0, n)
	next := time.Now()
	for i := 0; i < n; i++ {
		next = sched.Next(next)
		if next.IsZero() {
			break
		}
		out = append(out, next.Format("2006-01-02 15:04:05"))
	}
	return out
}

// RegisterSchedule 启动时注册招标情报站的定时采集任务。
//
// 调度分两层：进程内 cron 触发“轮次”，实际采集在 Redis 队列中由 worker 并发执行，
// 因此单次触发很快返回，不阻塞调度线程。
func RegisterSchedule(svc Service) error {
	impl, ok := svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("招标情报站调度注册失败：非预期的服务实现")
	}
	return impl.registerScheduleFromStore(context.Background())
}

// registerScheduleFromStore 从数据库读取任务配置并注册；保存配置后也会调用，实现热生效。
func (s *svcImpl) registerScheduleFromStore(ctx context.Context) error {
	cfg, err := s.repo.GetCollectSchedule(ctx)
	if err != nil {
		return fmt.Errorf("读取自动采集任务配置失败: %w", err)
	}
	spec := strings.TrimSpace(cfg.CronExpr)
	if spec == "" {
		spec = defaultCollectCron
	}
	if _, err := NormalizeCronExpr(spec); err != nil {
		// 库里的配置非法时回退默认节奏，避免整个调度直接失效
		s.logger.Warnw("自动采集任务 cron 非法，回退默认节奏",
			"cron", spec, "default", defaultCollectCron, "err", err)
		spec = defaultCollectCron
	}
	if _, err := schedule.Add(spec, "tender_intel_collect_round", func() {
		s.runScheduledRound()
	}); err != nil {
		return fmt.Errorf("注册招标情报站定时任务失败: %w", err)
	}
	s.logger.Infow("招标情报站自动采集任务已注册",
		"cron", spec, "enabled", cfg.Enabled == 1, "next_runs", NextRunTimes(spec, 3))
	return nil
}

// saveScheduleConfig 保存自动采集任务配置并立即生效（无需重启后端）。
func (s *svcImpl) saveScheduleConfig(ctx context.Context, cronExpr string, enabled bool, updatedBy int64) ([]string, error) {
	spec, err := NormalizeCronExpr(cronExpr)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveCollectSchedule(ctx, spec, enabled, updatedBy); err != nil {
		return nil, fmt.Errorf("保存自动采集任务配置失败: %w", err)
	}
	if err := s.registerScheduleFromStore(ctx); err != nil {
		return nil, err
	}
	s.logger.Infow("招标情报站自动采集任务已更新",
		"cron", spec, "enabled", enabled, "updated_by", updatedBy)
	return NextRunTimes(spec, 5), nil
}

// runScheduledRound 定时轮次入口：确认启用后抢锁，生成批次并投递各源采集任务。
func (s *svcImpl) runScheduledRound() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 配置在触发时再读一次：管理员刚停用自动采集时，已注册的任务应立即失效
	cfg, err := s.repo.GetCollectSchedule(ctx)
	if err != nil {
		s.logger.Warnw("读取自动采集任务配置失败", "err", err)
		return
	}
	if cfg.Enabled != 1 {
		return
	}

	redisSvc := redis.GetInstance()
	if redisSvc != nil {
		acquired, err := redisSvc.Client().SetNX(ctx, redisLockKey, time.Now().Unix(), redisLockTTL).Result()
		if err != nil {
			s.logger.Warnw("获取采集轮次锁失败", "err", err)
			return
		}
		if !acquired {
			s.logger.Infow("已有实例触发本轮回采集，跳过")
			return
		}
		defer func() {
			_ = redisSvc.Client().Del(context.Background(), redisLockKey).Err()
		}()
	}

	runID, err := s.StartCollectRound(ctx, "cron", nil)
	if err != nil {
		s.logger.Warnw("启动采集轮次失败", "err", err)
		return
	}
	s.logger.Infow("采集轮次已启动", "run_id", runID)
}

// StartCleanupSchedule 注册保留期清理任务（每日 03:30）。
func StartCleanupSchedule(svc Service) error {
	impl, ok := svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("招标情报站清理任务注册失败：非预期的服务实现")
	}
	_, err := schedule.Add("0 30 3 * * *", "tender_intel_cleanup", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		before := NowFunc().AddDate(0, 0, -retentionDays)
		rows, err := impl.repo.DeleteNoticesBefore(ctx, before)
		if err != nil {
			impl.logger.Warnw("清理过期招标情报失败", "err", err)
			return
		}
		if rows > 0 {
			impl.logger.Infow("清理过期招标情报完成", "rows", rows, "before", before.Format("2006-01-02"))
		}
	})
	if err != nil {
		return fmt.Errorf("注册招标情报站清理任务失败: %w", err)
	}
	return nil
}
