package bidreview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/repo/taskqueue"
)

// ReviewTaskHandler 审核任务 Handler，实现 taskqueue.TaskHandler
type ReviewTaskHandler struct {
	Svc Service
}

// Handle 实现 taskqueue.TaskHandler 接口
func (h *ReviewTaskHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload ReviewTaskPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("解析 bid_review payload 失败: %w", err)
	}

	svc, ok := h.Svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("ReviewTaskHandler: Svc 类型断言失败")
	}

	// 创建最小化 gin.Context（供内部复用 gin 上下文的方法使用）
	w := httptest.NewRecorder()
	ac, _ := gin.CreateTestContext(w)
	ac.Request, _ = http.NewRequestWithContext(ctx, "POST", "/internal/bid-review", nil)
	if payload.UserID > 0 {
		ac.Set("user_id", payload.UserID)
	}

	// Redis 分布式锁（TTL 与队列 visibility 一致，兜底 2h）
	lockKey := fmt.Sprintf("lock:bid_review:%d", payload.ProjectID)
	lockTTL := time.Duration(task.VisibilitySec) * time.Second
	if lockTTL <= 0 {
		lockTTL = 2 * time.Hour
	}
	acquired, err := svc.redisSvc.Client().SetNX(ctx, lockKey, task.ID, lockTTL).Result()
	if err != nil {
		return fmt.Errorf("获取项目锁失败: %w", err)
	}
	if !acquired {
		return fmt.Errorf("项目 %d 已有审核任务正在执行", payload.ProjectID)
	}
	defer svc.redisSvc.Client().Del(ctx, lockKey)

	// resume 定位：首次执行按 payload 指定阶段；Worker 失败重试自动定位首个 failed 阶段
	resume := payload.ResumeFromStage
	if task.Attempts > 0 {
		resume = ""
	}

	return svc.startAsyncPipeline(ctx, payload.ProjectID, payload.UserID, resume)
}

var _ taskqueue.TaskHandler = (*ReviewTaskHandler)(nil)
