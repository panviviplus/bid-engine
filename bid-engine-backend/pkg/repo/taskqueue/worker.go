package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"bid-engine/pkg/repo/redis"
)

// Worker 通用任务消费者（Redis BRPOP 实现）
type Worker struct {
	cfg         QueueConfig
	redis       redis.Service
	handler     TaskHandler
	logger      *zap.SugaredLogger
	sem         chan struct{}   // 并发控制令牌
	wakeCh      <-chan struct{} // 唤醒通道
	stopCh      chan struct{}
	onTaskFinal func(ctx context.Context, task *Task, succeeded bool) // 任务终态回调（succeeded / 失败入 DLQ）
}

// NewWorker 构造消费者
func NewWorker(cfg QueueConfig, redisSvc redis.Service, handler TaskHandler, logger *zap.SugaredLogger) *Worker {
	return &Worker{
		cfg:     cfg,
		redis:   redisSvc,
		handler: handler,
		logger:  logger,
		sem:     make(chan struct{}, cfg.Concurrency),
		wakeCh:  make(chan struct{}, 1),
		stopCh:  make(chan struct{}),
	}
}

// SetOnTaskFinal 注册任务终态回调（任务 succeeded 或失败入 DLQ 时触发，用于释放模块级资源）
func (w *Worker) SetOnTaskFinal(fn func(ctx context.Context, task *Task, succeeded bool)) {
	w.onTaskFinal = fn
}

// Wake 外部唤醒 Worker（非阻塞）
func (w *Worker) Wake() {}

// Stop 停止 Worker
func (w *Worker) Stop() {
	close(w.stopCh)
}

// Run 主消费循环（阻塞，应通过 context 控制生命周期）
func (w *Worker) Run(ctx context.Context) {
	w.logger.Infow("TaskQueue Worker 启动", "queue", w.cfg.QueueKey, "concurrency", w.cfg.Concurrency)
	defer w.logger.Infow("TaskQueue Worker 退出", "queue", w.cfg.QueueKey)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		// BRPOP 阻塞等待（支持高优先级队列优先）
		w.consumeOne(ctx)
	}
}

// consumeOne 领取一个任务并异步处理（并发度由 sem 令牌控制）
func (w *Worker) consumeOne(ctx context.Context) {
	// 获取并发令牌（阻塞直到有空位）
	select {
	case w.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}

	taskID, ok := w.popTask(ctx)
	if !ok {
		<-w.sem
		return
	}

	// 异步处理：释放令牌由处理 goroutine 负责，保证 Concurrency 配置真正生效
	go func() {
		defer func() { <-w.sem }()
		w.processTask(ctx, taskID)
	}()
}

// popTask 尝试从高优队列/普通队列领取一个任务ID（BRPOP 原子弹出）
func (w *Worker) popTask(ctx context.Context) (string, bool) {
	// 先检查高优队列，再检查普通队列
	if w.cfg.HighQueueKey != "" {
		taskID, err := w.redis.Client().BRPop(ctx, w.cfg.PollInterval, w.cfg.HighQueueKey).Result()
		if err == nil && len(taskID) > 1 {
			return taskID[1], true
		}
	}

	taskID, err := w.redis.Client().BRPop(ctx, w.cfg.PollInterval, w.cfg.QueueKey).Result()
	if err != nil || len(taskID) < 2 {
		return "", false
	}
	return taskID[1], true
}

// processTask 处理单个任务
func (w *Worker) processTask(ctx context.Context, taskID string) {
	task, err := loadTask(ctx, w.redis, taskID)
	if err != nil {
		w.logger.Warnw("加载任务失败", "taskID", taskID, "err", err)
		return
	}
	if task.Status == TaskStatusCancelled || task.Status == TaskStatusSucceeded || task.Status == TaskStatusFailed {
		return
	}

	// 设置 processing 状态 + 租约
	now := time.Now()
	leaseUntil := now.Add(time.Duration(task.VisibilitySec) * time.Second)
	updateTaskFields(ctx, w.redis, taskID, map[string]any{
		"status":       string(TaskStatusProcessing),
		"claimed_at":   now.Unix(),
		"lease_until":  leaseUntil.Unix(),
		"heartbeat_at": now.Unix(),
	})

	// 租约心跳：长任务定期续期，避免被僵尸扫描器误回收（handler 结束后停止）
	heartbeatCtx, heartbeatStop := context.WithCancel(ctx)
	defer heartbeatStop()
	go w.heartbeat(heartbeatCtx, taskID, task.VisibilitySec)

	// 执行任务处理器
	taskCtx, cancel := context.WithTimeout(context.Background(), time.Duration(task.VisibilitySec)*time.Second)
	defer cancel()

	if err := w.handler.Handle(taskCtx, task); err != nil {
		var cancelled *CancelledError
		if errors.As(err, &cancelled) {
			updateTaskFields(ctx, w.redis, taskID, map[string]any{
				"status": string(TaskStatusCancelled), "last_error": cancelled.Error(),
			})
			w.redis.Expire(ctx, taskKey(taskID), 72*time.Hour)
			if w.onTaskFinal != nil {
				w.onTaskFinal(ctx, task, false)
			}
			return
		}
		w.handleFailure(ctx, task, err)
		return
	}

	// 成功：标记 succeeded，3 天后过期
	updateTaskFields(ctx, w.redis, taskID, map[string]any{
		"status": string(TaskStatusSucceeded),
	})
	w.redis.Expire(ctx, taskKey(taskID), 72*time.Hour)

	if w.onTaskFinal != nil {
		w.onTaskFinal(ctx, task, true)
	}
}

// heartbeat 定期续期租约，保证长流水线不被僵尸扫描器回收
func (w *Worker) heartbeat(ctx context.Context, taskID string, visibilitySec int) {
	if visibilitySec <= 0 {
		visibilitySec = 3600
	}
	interval := time.Duration(visibilitySec/2) * time.Second
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateTaskFields(ctx, w.redis, taskID, map[string]any{
				"lease_until":  time.Now().Add(time.Duration(visibilitySec) * time.Second).Unix(),
				"heartbeat_at": time.Now().Unix(),
			})
		}
	}
}

// nonRetryableError 由任务处理器可选实现：返回 true 表示失败不应自动重试
// （例如内容校验失败、来源数据失效），直接进入终态，避免无意义地重复执行。
type nonRetryableError interface{ NonRetryable() bool }

// handleFailure 处理任务失败：按可重试性与最大次数决定退避重试或入 DLQ。
func (w *Worker) handleFailure(ctx context.Context, task *Task, err error) {
	var deferred interface{ RetryAfter() time.Duration }
	if errors.As(err, &deferred) {
		delay := deferred.RetryAfter()
		scheduledAt := time.Now().Add(delay).Unix()
		updateTaskFields(ctx, w.redis, task.ID, map[string]any{
			"status": string(TaskStatusPending), "last_error": "", "scheduled_at": scheduledAt,
		})
		w.redis.Client().ZAdd(ctx, DelayedQueueKey, goredis.Z{
			Score: float64(scheduledAt), Member: fmt.Sprintf("%s|%s", task.TaskType, task.ID),
		})
		return
	}
	attempts := task.Attempts + 1
	errMsg := err.Error()
	retryable := true
	var nr nonRetryableError
	if errors.As(err, &nr) && nr.NonRetryable() {
		retryable = false
	}

	if !retryable || attempts >= task.MaxAttempts {
		// 达到最大重试：标记 failed + 推入 DLQ
		updateTaskFields(ctx, w.redis, task.ID, map[string]any{
			"status":     string(TaskStatusFailed),
			"attempts":   attempts,
			"last_error": errMsg,
		})
		w.redis.Expire(ctx, taskKey(task.ID), 7*24*time.Hour) // 7 天后过期

		cfg, ok := QueueConfigs[task.TaskType]
		if ok {
			w.redis.Client().LPush(ctx, cfg.DLQKey, task.ID)
		}
		w.logger.Warnw("任务失败终止，已入DLQ", "taskID", task.ID, "attempts", attempts, "err", errMsg)

		// 任务终态回调（释放模块级资源，如活跃任务标记）
		if w.onTaskFinal != nil {
			w.onTaskFinal(ctx, task, false)
		}
		return
	}

	// 退避重试：delay = retryBackoff * attempts
	delay := w.cfg.RetryBackoff * time.Duration(attempts)
	scheduledAt := time.Now().Add(delay).Unix()

	updateTaskFields(ctx, w.redis, task.ID, map[string]any{
		"status":       string(TaskStatusPending),
		"attempts":     attempts,
		"last_error":   errMsg,
		"scheduled_at": scheduledAt,
	})

	// 加入延迟队列（全局 Sorted Set）
	w.redis.Client().ZAdd(ctx, DelayedQueueKey, goredis.Z{
		Score:  float64(scheduledAt),
		Member: fmt.Sprintf("%s|%s", task.TaskType, task.ID),
	})

	w.logger.Warnw("任务失败，退避重试", "taskID", task.ID, "attempts", attempts,
		"maxAttempts", task.MaxAttempts, "delay", delay, "err", errMsg)
}

// RunDelayedReclaimer 全局延迟任务回收器（独立 goroutine 运行）
func RunDelayedReclaimer(ctx context.Context, redisSvc redis.Service, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logger.Infow("延迟任务回收器 启动")
	defer logger.Infow("延迟任务回收器 退出")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := float64(time.Now().Unix())
			// ZRANGEBYSCORE task:delayed 0 now LIMIT 0 50
			members, err := redisSvc.Client().ZRangeArgs(ctx, goredis.ZRangeArgs{
				Key:     DelayedQueueKey,
				Start:   "0",
				Stop:    fmt.Sprintf("%.0f", now),
				ByScore: true,
				Count:   50,
			}).Result()
			if err != nil {
				continue
			}

			for _, member := range members {
				// member 格式: "{task_type}|{task_id}"
				parts := strings.SplitN(member, "|", 2)
				if len(parts) != 2 {
					continue
				}
				taskType, taskID := parts[0], parts[1]

				cfg, ok := QueueConfigs[taskType]
				if !ok {
					continue
				}

				// 重新入队 + 从延迟队列移除
				if err := redisSvc.Client().LPush(ctx, cfg.QueueKey, taskID).Err(); err == nil {
					redisSvc.Client().ZRem(ctx, DelayedQueueKey, member)
					logger.Debugw("延迟任务回收", "taskID", taskID, "taskType", taskType)
				}
			}
		}
	}
}

// zombieHeartbeatStaleSec 心跳超过该时长（秒）仍为 processing 的任务视为孤儿回收。
// 租约（visibility）是任务的最大执行时长（如 3h），进程被杀后要等租约到期才能被
// lease 判定回收；心跳判定可在几分钟内识别出进程已死的任务。
const zombieHeartbeatStaleSec = int64(5 * 60)

// RunZombieScanner 僵尸任务扫描器（独立 goroutine）：
// 回收租约过期或心跳超时仍为 processing 的任务，重置为 pending 并重新入队，
// 同时原子清除该任务持有的模块锁，避免残留锁阻塞后续任务。
func RunZombieScanner(ctx context.Context, redisSvc redis.Service, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logger.Infow("僵尸任务扫描器 启动")
	defer logger.Infow("僵尸任务扫描器 退出")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().Unix()
			var cursor uint64
			for {
				keys, nextCursor, err := redisSvc.Client().Scan(ctx, cursor, "task:*", 100).Result()
				if err != nil {
					break
				}
				for _, key := range keys {
					status, _ := redisSvc.Client().HGet(ctx, key, "status").Result()
					if status != string(TaskStatusProcessing) {
						continue
					}
					leaseStr, _ := redisSvc.Client().HGet(ctx, key, "lease_until").Result()
					leaseUntil, _ := parseInt64(leaseStr)
					heartbeatStr, _ := redisSvc.Client().HGet(ctx, key, "heartbeat_at").Result()
					heartbeatAt, _ := parseInt64(heartbeatStr)
					if heartbeatAt == 0 {
						// 旧任务无 heartbeat_at 时回退到 updated_at（每次心跳/领用都会刷新）。
						updatedStr, _ := redisSvc.Client().HGet(ctx, key, "updated_at").Result()
						heartbeatAt, _ = parseInt64(updatedStr)
					}
					if !zombieTaskStale(now, leaseUntil, heartbeatAt) {
						continue
					}
					// 僵尸任务：重置为 pending 并重新入队，同时清除其模块锁。
					taskID := strings.TrimPrefix(key, "task:")
					taskType, _ := redisSvc.Client().HGet(ctx, key, "task_type").Result()
					projectID, _ := redisSvc.Client().HGet(ctx, key, "project_id").Result()
					cfg, ok := QueueConfigs[taskType]
					if ok {
						redisSvc.Client().LPush(ctx, cfg.QueueKey, taskID)
					}
					redisSvc.Client().HSet(ctx, key, "status", string(TaskStatusPending))
					clearTaskLock(ctx, redisSvc, taskType, projectID, taskID, logger)
					logger.Warnw("僵尸任务回收", "taskID", taskID, "lease_until", leaseUntil, "heartbeat_at", heartbeatAt)
				}
				cursor = nextCursor
				if cursor == 0 {
					break
				}
			}
		}
	}
}

// zombieTaskStale 判定 processing 任务是否为僵尸：租约过期或心跳超时（无心跳记录时按租约判定）。
func zombieTaskStale(now, leaseUntil, heartbeatAt int64) bool {
	leaseExpired := leaseUntil > 0 && leaseUntil < now
	heartbeatStale := heartbeatAt > 0 && now-heartbeatAt > zombieHeartbeatStaleSec
	return leaseExpired || heartbeatStale
}

// clearTaskLock 原子清除模块级任务锁：仅当锁值等于被回收任务时才删除，
// 避免误删新任务持有的有效锁。
func clearTaskLock(ctx context.Context, redisSvc redis.Service, taskType, projectID, taskID string, logger *zap.SugaredLogger) {
	if taskType == "" || projectID == "" || taskID == "" {
		return
	}
	lockKey := fmt.Sprintf("lock:%s:%s", taskType, projectID)
	if err := clearTaskLockScript.Run(ctx, redisSvc.Client(), []string{lockKey}, taskID).Err(); err != nil {
		logger.Warnw("清除任务模块锁失败", "lock_key", lockKey, "taskID", taskID, "err", err)
	}
}

// clearTaskLockScript 原子“值匹配才删除”的 Lua 脚本。
var clearTaskLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
