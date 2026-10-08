package taskqueue

import (
	"context"
	"fmt"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSucceeded  TaskStatus = "succeeded"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityHigh   TaskPriority = "high"
)

// Task 任务元数据，存储在 Redis Hash 中
type Task struct {
	ID        string       `json:"id"`         // 任务唯一ID（格式: {task_type}_{unix_nano}_{rand}）
	TaskType  string       `json:"task_type"`  // 任务类型
	UserID    int64        `json:"user_id"`    // 提交者
	ProjectID int64        `json:"project_id"` // 关联项目ID
	Payload   string       `json:"payload"`    // JSON 负载
	Status    TaskStatus   `json:"status"`     // 任务状态
	Priority  TaskPriority `json:"priority"`   // 优先级
	Attempts  int          `json:"attempts"`   // 已重试次数

	// 以下字段在入队时由 Producer 填充
	MaxAttempts   int   `json:"max_attempts"`   // 最大重试次数
	VisibilitySec int   `json:"visibility_sec"` // 租约超时秒数
	ScheduledAt   int64 `json:"scheduled_at"`   // 计划执行时间（unix timestamp）
	CreatedAt     int64 `json:"created_at"`     // 创建时间

	// 以下字段在消费时由 Worker 填充
	ClaimedAt  int64  `json:"claimed_at,omitempty"`  // 领取时间
	LeaseUntil int64  `json:"lease_until,omitempty"` // 租约到期时间
	LastError  string `json:"last_error,omitempty"`  // 最近错误
	UpdatedAt  int64  `json:"updated_at,omitempty"`  // 更新时间
}

// TaskHandler 任务处理器接口，每种 task_type 实现一个
type TaskHandler interface {
	// Handle 处理任务，返回 error 视为失败，触发重试逻辑
	Handle(ctx context.Context, task *Task) error
}

// EnqueueOpts 入队选项
type EnqueueOpts struct {
	TaskID    string       // 可选确定性任务ID；用于数据库业务任务与Redis任务幂等关联
	Priority  TaskPriority // 优先级，默认 normal
	ProjectID int64        // 关联项目ID
	UserID    int64        // 任务提交者
	DelaySec  int          // 延迟执行秒数，0=立即
}

// DeferError 表示任务因配额等非故障原因需要稍后重试，不消耗失败重试次数。
type DeferError struct {
	After  time.Duration
	Reason string
}

func (e *DeferError) Error() string {
	if e == nil || e.Reason == "" {
		return "任务等待可用执行槽位"
	}
	return e.Reason
}

func (e *DeferError) RetryAfter() time.Duration {
	if e == nil || e.After <= 0 {
		return 2 * time.Second
	}
	return e.After
}

// Defer 构造不计入 attempts 的延期信号。
func Defer(after time.Duration, reason string) error {
	return &DeferError{After: after, Reason: reason}
}

// CancelledError 表示业务处理器已完成取消收尾，队列任务应进入 cancelled 终态。
type CancelledError struct{ Reason string }

func (e *CancelledError) Error() string {
	if e == nil || e.Reason == "" {
		return "用户取消"
	}
	return e.Reason
}

func Cancelled(reason string) error { return &CancelledError{Reason: reason} }

func validateTaskID(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 128 {
		return fmt.Errorf("任务ID过长")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("任务ID包含非法字符")
	}
	return nil
}

// QueueStatus 队列位置信息
type QueueStatus struct {
	TaskID           string       `json:"task_id"`
	TaskType         string       `json:"task_type"`
	QueueName        string       `json:"queue_name"`
	Status           TaskStatus   `json:"status"`
	QueuePosition    int64        `json:"queue_position"`     // 当前队列位置（0=正在处理或不在队列中）
	QueueTotal       int64        `json:"queue_total"`        // 队列中 pending 任务总数
	EstimatedWaitSec int64        `json:"estimated_wait_sec"` // 预估等待时间
	Priority         TaskPriority `json:"priority"`
	CreatedAt        int64        `json:"created_at"`
}
