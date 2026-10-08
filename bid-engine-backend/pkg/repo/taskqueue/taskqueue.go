package taskqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"bid-engine/pkg/repo/redis"
)

// Repo 队列仓库（Redis 实现）
type Repo struct {
	redis  redis.Service
	wakeCh chan struct{}
}

// NewRepo 构造仓库
func NewRepo(redisSvc redis.Service) *Repo {
	return &Repo{
		redis:  redisSvc,
		wakeCh: make(chan struct{}, 1),
	}
}

// WakeChan 获取唤醒通道
func (r *Repo) WakeChan() <-chan struct{} {
	return r.wakeCh
}

// Enqueue 通用入队：LPUSH 任务到队列 + HSET 任务元数据
func (r *Repo) Enqueue(ctx context.Context, taskType string, payload any, opts EnqueueOpts) (string, error) {
	cfg, ok := QueueConfigs[taskType]
	if !ok {
		return "", fmt.Errorf("未知任务类型: %s", taskType)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("payload序列化失败: %w", err)
	}

	if err := validateTaskID(opts.TaskID); err != nil {
		return "", err
	}
	taskID := opts.TaskID
	if taskID == "" {
		taskID = genTaskID(taskType)
	}
	now := time.Now().Unix()

	if opts.Priority == "" {
		opts.Priority = TaskPriorityNormal
	}

	taskFields := map[string]any{
		"task_type":      taskType,
		"user_id":        opts.UserID,
		"project_id":     opts.ProjectID,
		"payload":        string(payloadJSON),
		"status":         string(TaskStatusPending),
		"priority":       string(opts.Priority),
		"attempts":       0,
		"max_attempts":   cfg.MaxAttempts,
		"visibility_sec": cfg.VisibilitySec,
		"scheduled_at":   now,
		"created_at":     now,
	}
	queueKey := cfg.QueueKey
	if opts.Priority == TaskPriorityHigh {
		queueKey = cfg.HighQueueKey
	}
	args := make([]any, 0, 1+len(taskFields)*2)
	args = append(args, taskID)
	for key, value := range taskFields {
		args = append(args, key, value)
	}
	if err := enqueueTaskScript.Run(ctx, r.redis.Client(), []string{taskKey(taskID), queueKey}, args...).Err(); err != nil {
		return "", fmt.Errorf("任务原子入队失败: %w", err)
	}

	// 非阻塞唤醒
	select {
	case r.wakeCh <- struct{}{}:
	default:
	}
	return taskID, nil
}

// GetTask 返回队列任务元数据，供业务状态对账使用。
func (r *Repo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return loadTask(ctx, r.redis, taskID)
}

// GetQueueStatus 查询任务队列位置
func (r *Repo) GetQueueStatus(ctx context.Context, taskID string) (*QueueStatus, error) {
	task, err := loadTask(ctx, r.redis, taskID)
	if err != nil {
		return nil, err
	}

	cfg, ok := QueueConfigs[task.TaskType]
	if !ok {
		return nil, fmt.Errorf("未知任务类型: %s", task.TaskType)
	}

	status := &QueueStatus{
		TaskID:    task.ID,
		TaskType:  task.TaskType,
		QueueName: cfg.Name,
		Status:    task.Status,
		Priority:  task.Priority,
		CreatedAt: task.CreatedAt,
	}

	if task.Status == TaskStatusPending {
		pos := r.redis.Client().LPos(ctx, cfg.QueueKey, taskID, goredis.LPosArgs{})
		status.QueuePosition = pos.Val()
		total := r.redis.Client().LLen(ctx, cfg.QueueKey)
		status.QueueTotal = total.Val()
		// 修正口径：入队用 LPUSH、消费用 BRPOP，LPos 的索引方向与执行顺序相反。
		// 距离队首（下一个被执行）的真实顺位见 QueueOrder。
		status.EstimatedWaitSec = (total.Val() - pos.Val() - 1) * 300
	}

	return status, nil
}

// QueueOrder 返回任务在“即将被执行”的顺序中的 1-based 顺位。
//
// worker 先排空高优队列再取普通队列，因此：
//   - 高优队列中的任务顺位 = 高优队列内距离队首的名次；
//   - 普通队列中的任务顺位 = 高优队列长度 + 普通队列内距离队首的名次。
//
// 处于延迟队列（失败退避重试）的 pending 任务没有确定顺位，返回 0。
func (r *Repo) QueueOrder(ctx context.Context, taskID string) (int64, error) {
	task, err := loadTask(ctx, r.redis, taskID)
	if err != nil {
		return 0, err
	}
	if task.Status != TaskStatusPending {
		return 0, nil
	}
	cfg, ok := QueueConfigs[task.TaskType]
	if !ok {
		return 0, fmt.Errorf("未知任务类型: %s", task.TaskType)
	}
	highTotal, err := r.redis.Client().LLen(ctx, cfg.HighQueueKey).Result()
	if err != nil {
		return 0, err
	}
	if pos, err := r.redis.Client().LPos(ctx, cfg.HighQueueKey, taskID, goredis.LPosArgs{}).Result(); err == nil {
		return highTotal - pos, nil
	}
	normalTotal, err := r.redis.Client().LLen(ctx, cfg.QueueKey).Result()
	if err != nil {
		return 0, err
	}
	if pos, err := r.redis.Client().LPos(ctx, cfg.QueueKey, taskID, goredis.LPosArgs{}).Result(); err == nil {
		return highTotal + normalTotal - pos, nil
	}
	// 既不在队列里又还是 pending：说明正处于延迟重试窗口
	return 0, nil
}

// ReprioritizeTask 调整 pending 任务的优先级。
//
// 队列只有普通/高优两条 List，且入队脚本对已存在的任务号直接跳过，
// 所以“改优先级”必须是一次原子搬迁：校验仍是 pending → 从延迟队列与两条
// List 上摘下来 → 按目标档位重新插入。非原子实现会和 worker 的 BRPOP 并发，
// 产生同一任务被消费两次。
func (r *Repo) ReprioritizeTask(ctx context.Context, taskID string, priority TaskPriority) error {
	if priority != TaskPriorityHigh && priority != TaskPriorityNormal {
		return fmt.Errorf("不支持的优先级: %s", priority)
	}
	task, err := loadTask(ctx, r.redis, taskID)
	if err != nil {
		return err
	}
	if task.Status != TaskStatusPending {
		return fmt.Errorf("仅可调整排队中任务的优先级，当前状态: %s", task.Status)
	}
	cfg, ok := QueueConfigs[task.TaskType]
	if !ok {
		return fmt.Errorf("未知任务类型: %s", task.TaskType)
	}
	result, err := reprioritizeScript.Run(ctx, r.redis.Client(),
		[]string{
			taskKey(taskID),
			cfg.QueueKey,
			cfg.HighQueueKey,
			DelayedQueueKey,
		},
		string(priority),
		time.Now().Unix(),
		taskID,
		fmt.Sprintf("%s|%s", task.TaskType, taskID),
	).Text()
	if err != nil {
		return fmt.Errorf("调整优先级失败: %w", err)
	}
	switch result {
	case "ok":
		// 提升后立即插到目标队列队首（下一个执行）；降级则排到普通队列队尾
		select {
		case r.wakeCh <- struct{}{}:
		default:
		}
		return nil
	case "missing":
		return fmt.Errorf("任务不存在: %s", taskID)
	case "not_pending":
		return fmt.Errorf("任务已不在排队中，无法调整优先级")
	default:
		return fmt.Errorf("调整优先级失败: %s", result)
	}
}

// DeleteTask 删除任务的队列元数据（业务侧必须先确认任务已处于终态）。
func (r *Repo) DeleteTask(ctx context.Context, taskID string) error {
	task, err := loadTask(ctx, r.redis, taskID)
	if err != nil {
		return err
	}
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
		return fmt.Errorf("任务仍在排队或执行中，无法删除")
	}
	cfg, ok := QueueConfigs[task.TaskType]
	if !ok {
		return fmt.Errorf("未知任务类型: %s", task.TaskType)
	}
	_ = r.redis.Client().LRem(ctx, cfg.QueueKey, 0, taskID).Err()
	_ = r.redis.Client().LRem(ctx, cfg.HighQueueKey, 0, taskID).Err()
	_ = r.redis.Client().ZRem(ctx, DelayedQueueKey, fmt.Sprintf("%s|%s", task.TaskType, taskID)).Err()
	return r.redis.Client().Del(ctx, taskKey(taskID)).Err()
}

// CancelTask 取消 pending 任务
func (r *Repo) CancelTask(ctx context.Context, taskID string) error {
	task, err := loadTask(ctx, r.redis, taskID)
	if err != nil {
		return err
	}
	if task.Status != TaskStatusPending {
		return fmt.Errorf("仅可取消 pending 状态的任务，当前状态: %s", task.Status)
	}
	cfg, ok := QueueConfigs[task.TaskType]
	if !ok {
		return fmt.Errorf("未知任务类型: %s", task.TaskType)
	}
	_ = r.redis.Client().LRem(ctx, cfg.QueueKey, 0, taskID).Err()
	_ = r.redis.Client().LRem(ctx, cfg.HighQueueKey, 0, taskID).Err()
	_ = r.redis.Client().ZRem(ctx, DelayedQueueKey, fmt.Sprintf("%s|%s", task.TaskType, taskID)).Err()
	r.redis.Client().HSet(ctx, taskKey(taskID),
		"status", string(TaskStatusCancelled), "last_error", "用户取消", "updated_at", time.Now().Unix())
	r.redis.Expire(ctx, taskKey(taskID), 72*time.Hour)
	return nil
}

// loadTask 从 Redis Hash 加载任务
func loadTask(ctx context.Context, redisSvc redis.Service, taskID string) (*Task, error) {
	vals, err := redisSvc.Client().HGetAll(ctx, taskKey(taskID)).Result()
	if err != nil {
		return nil, fmt.Errorf("HGetAll 失败: %w", err)
	}
	if len(vals) == 0 {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return parseTaskFromMap(taskID, vals), nil
}

// updateTaskFields 更新任务 Hash 字段
func updateTaskFields(ctx context.Context, r redis.Service, taskID string, fields map[string]any) {
	fields["updated_at"] = time.Now().Unix()
	r.Client().HSet(ctx, taskKey(taskID), fields)
}

// parseTaskFromMap 从 HGetAll 结果解析 Task
func parseTaskFromMap(taskID string, vals map[string]string) *Task {
	task := &Task{ID: taskID}
	task.TaskType = vals["task_type"]
	task.Payload = vals["payload"]
	task.Status = TaskStatus(vals["status"])
	task.Priority = TaskPriority(vals["priority"])
	task.LastError = vals["last_error"]
	if v, err := strconv.ParseInt(vals["user_id"], 10, 64); err == nil {
		task.UserID = v
	}
	if v, err := strconv.ParseInt(vals["project_id"], 10, 64); err == nil {
		task.ProjectID = v
	}
	if v, err := strconv.Atoi(vals["attempts"]); err == nil {
		task.Attempts = v
	}
	if v, err := strconv.Atoi(vals["max_attempts"]); err == nil {
		task.MaxAttempts = v
	}
	if v, err := strconv.Atoi(vals["visibility_sec"]); err == nil {
		task.VisibilitySec = v
	}
	if v, err := strconv.ParseInt(vals["scheduled_at"], 10, 64); err == nil {
		task.ScheduledAt = v
	}
	if v, err := strconv.ParseInt(vals["created_at"], 10, 64); err == nil {
		task.CreatedAt = v
	}
	if v, err := strconv.ParseInt(vals["claimed_at"], 10, 64); err == nil {
		task.ClaimedAt = v
	}
	if v, err := strconv.ParseInt(vals["lease_until"], 10, 64); err == nil {
		task.LeaseUntil = v
	}
	if v, err := strconv.ParseInt(vals["updated_at"], 10, 64); err == nil {
		task.UpdatedAt = v
	}
	return task
}

// taskKey 返回 Redis Hash key
func taskKey(taskID string) string {
	return "task:" + taskID
}

// genTaskID 生成唯一任务ID
func genTaskID(taskType string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%d_%s", taskType, time.Now().UnixNano(), hex.EncodeToString(b))
}

var enqueueTaskScript = goredis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 then
  return 0
end
for i = 2, #ARGV, 2 do
  redis.call("HSET", KEYS[1], ARGV[i], ARGV[i + 1])
end
redis.call("LPUSH", KEYS[2], ARGV[1])
return 1
`)

// reprioritizeScript 原子调整 pending 任务的优先级。
//
// KEYS[1]=任务 Hash  KEYS[2]=普通队列  KEYS[3]=高优队列  KEYS[4]=延迟队列
// ARGV[1]=目标优先级  ARGV[2]=当前时间戳  ARGV[3]=任务 ID  ARGV[4]=延迟队列成员
//
// 提升到高优：RPUSH 到高优队列尾部，成为下一个被执行的任务；
// 降回普通：LPUSH 到普通队列头部，排到所有普通任务之后执行。
var reprioritizeScript = goredis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 0 then
  return "missing"
end
if redis.call("HGET", KEYS[1], "status") ~= "pending" then
  return "not_pending"
end
redis.call("ZREM", KEYS[4], ARGV[4])
redis.call("LREM", KEYS[2], 0, ARGV[3])
redis.call("LREM", KEYS[3], 0, ARGV[3])
redis.call("HSET", KEYS[1], "priority", ARGV[1], "scheduled_at", ARGV[2], "updated_at", ARGV[2])
if ARGV[1] == "high" then
  redis.call("RPUSH", KEYS[3], ARGV[3])
else
  redis.call("LPUSH", KEYS[2], ARGV[3])
end
return "ok"
`)
