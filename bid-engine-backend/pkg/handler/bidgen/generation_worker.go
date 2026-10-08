package bidgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	bidgenRepo "bid-engine/pkg/repo/bidgen"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
	"bid-engine/pkg/service/biddoc"
)

// GenerateTaskPayload 是 Redis 队列与数据库业务任务之间的稳定关联。
type GenerateTaskPayload struct {
	BidTaskID int64 `json:"bid_task_id"`
}

// GenerationQuotaResolver 为后续会员额度接入保留稳定边界。
type GenerationQuotaResolver interface {
	Limit(ctx context.Context, userID int64) int
}

type configGenerationQuotaResolver struct{}

func (configGenerationQuotaResolver) Limit(_ context.Context, _ int64) int {
	return bidGenConfigInt("bid_generation.default_user_concurrency", 10, 1, 100)
}

type BidGenGenerateTaskHandler struct{ Svc Service }

type permanentGenerationError struct{ err error }

func (e *permanentGenerationError) Error() string      { return e.err.Error() }
func (e *permanentGenerationError) Unwrap() error      { return e.err }
func (e *permanentGenerationError) NonRetryable() bool { return true }

func permanentGeneration(err error) error {
	if err == nil {
		return nil
	}
	return &permanentGenerationError{err: err}
}

func (h *BidGenGenerateTaskHandler) Handle(ctx context.Context, queueTask *taskqueue.Task) error {
	var payload GenerateTaskPayload
	if err := json.Unmarshal([]byte(queueTask.Payload), &payload); err != nil || payload.BidTaskID <= 0 {
		return permanentGeneration(fmt.Errorf("解析标书生成任务失败"))
	}
	s, ok := h.Svc.(*svcImpl)
	if !ok {
		return permanentGeneration(fmt.Errorf("标书生成服务不可用"))
	}
	task, err := s.repo.GetTaskByID(ctx, payload.BidTaskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return permanentGeneration(fmt.Errorf("标书生成任务不存在"))
		}
		return err
	}
	if isGenerationTerminal(task.Status) {
		return nil
	}
	defer s.deleteGenerationKey(generationCancelKey(task.ProjectID, task.ID))
	if task.Status == "cancelling" || isCancelRequested(ctx, s, task.ProjectID, task.ID) {
		s.finishCancelledGeneration(task.ProjectID, task.ID, int(task.CompletedCount))
		_, _ = s.publishGenerationEvent(ctx, task.ID, "cancelled", map[string]any{"taskId": task.ID})
		return taskqueue.Cancelled("用户取消")
	}

	releaseSlot, acquired, err := s.acquireUserGenerationSlot(ctx, task.UserID, queueTask.ID)
	if err != nil {
		return err
	}
	if !acquired {
		_, _ = s.publishGenerationEvent(ctx, task.ID, "task_state", map[string]any{
			"taskId": task.ID, "status": "pending", "message": "等待可用生成槽位",
		})
		return taskqueue.Defer(2*time.Second, "等待用户生成槽位")
	}
	defer releaseSlot()
	releaseExecution, executionAcquired, err := s.acquireGenerationExecutionLock(ctx, task.ProjectID, queueTask.ID, queueTask.VisibilitySec)
	if err != nil {
		return err
	}
	if !executionAcquired {
		return taskqueue.Defer(2*time.Second, "等待同项目生成任务释放")
	}
	defer releaseExecution()

	task, runnable, err := s.repo.ClaimGenerationTask(ctx, task.ID)
	if err != nil {
		if errors.Is(err, bidgenRepo.ErrTaskNotRunnable) {
			return nil
		}
		return err
	}
	if !runnable {
		s.finishCancelledGeneration(task.ProjectID, task.ID, int(task.CompletedCount))
		_, _ = s.publishGenerationEvent(ctx, task.ID, "cancelled", map[string]any{"taskId": task.ID})
		return taskqueue.Cancelled("用户取消")
	}
	_, _ = s.publishGenerationEvent(ctx, task.ID, "task_state", taskResponse(task))

	execCtx, cancel := context.WithCancel(repoLLM.WithUserID(ctx, task.UserID))
	defer cancel()
	watchDone := make(chan struct{})
	go s.watchGenerationCancellation(execCtx, cancel, task.ProjectID, task.ID, watchDone)
	heartbeatDone := make(chan struct{})
	go s.heartbeatGenerationTask(execCtx, task.ID, task.UserID, task.ProjectID, queueTask.ID, queueTask.VisibilitySec, heartbeatDone)

	err = s.executeGenerationTask(execCtx, task)
	cancel()
	<-watchDone
	<-heartbeatDone
	if err == nil {
		return nil
	}
	fresh, _ := s.repo.GetTaskByID(context.Background(), task.ID)
	if (fresh != nil && fresh.Status == "cancelling") || isCancelRequested(context.Background(), s, task.ProjectID, task.ID) {
		completed := int(task.CompletedCount)
		if fresh != nil {
			completed = int(fresh.CompletedCount)
		}
		s.finishCancelledGeneration(task.ProjectID, task.ID, completed)
		_, _ = s.publishGenerationEvent(context.Background(), task.ID, "cancelled", map[string]any{"taskId": task.ID})
		return taskqueue.Cancelled("用户取消")
	}
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		err = context.DeadlineExceeded
	}
	msg := generationUserMessage(err)
	if msg == "" {
		msg = "标书生成失败"
	}
	_ = s.repo.UpdateTaskFields(context.Background(), task.ID, map[string]interface{}{"error_msg": msg})
	if isPermanentGenerationError(err) {
		return permanentGeneration(err)
	}

	// 可重试错误保持项目锁定态，当前未提交章节恢复为 pending。
	s.resetGeneratingOutlines(context.Background(), task.ProjectID)
	_ = s.repo.UpdateTaskFields(context.Background(), task.ID, map[string]interface{}{
		"status": "pending", "error_msg": msg, "heartbeat_at": nil,
	})
	_ = s.repo.UpdateProjectFields(context.Background(), task.ProjectID, map[string]interface{}{
		"status": ProjectStatusGenerating, "stage": "queued", "last_error": "",
	})
	_, _ = s.publishGenerationEvent(context.Background(), task.ID, "task_state", map[string]any{
		"taskId": task.ID, "status": "pending", "message": "生成服务暂时不可用，正在等待重试",
	})
	return err
}

func generationUserMessage(err error) string {
	if err == nil {
		return ""
	}
	if repoLLM.IsLLMError(err) || repoLLM.IsProviderRejection(err) {
		return repoLLM.FriendlyMessage(err)
	}
	var permanent interface{ NonRetryable() bool }
	if errors.As(err, &permanent) && permanent.NonRetryable() {
		message := err.Error()
		for _, safe := range []string{"章节内容为空", "正文结构异常", "生成任务没有有效章节", "标书项目不存在"} {
			if strings.Contains(message, safe) {
				return message
			}
		}
		return "生成内容未通过校验，请检查文档结构后重试"
	}
	return "生成服务暂时异常，正在自动重试"
}

var _ taskqueue.TaskHandler = (*BidGenGenerateTaskHandler)(nil)

func (s *svcImpl) executeGenerationTask(ctx context.Context, task *model.BidGenTask) error {
	proj, err := s.repo.GetProjectByID(ctx, task.ProjectID)
	if err != nil || proj == nil {
		return permanentGeneration(fmt.Errorf("标书项目不存在"))
	}
	outline, err := s.repo.GetOutlineByProjectID(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	ordered, err := biddoc.OrderOutlineTree(outline)
	if err != nil {
		return permanentGeneration(err)
	}
	ids := unmarshalIDs(task.OutlineIds)
	// 与入队时保持同一口径：write 模式的父章节连带子章节
	targets := s.selectTargets(ordered, ids, task.GenMode == GenModeWrite)
	if len(targets) == 0 {
		return permanentGeneration(fmt.Errorf("生成任务没有有效章节"))
	}
	if _, err := s.ensureDocumentAnchors(ctx, proj.ID, ordered); err != nil {
		return permanentGeneration(fmt.Errorf("正文结构异常: %w", err))
	}

	chapterContents, err := s.repo.GetChapterContentsByProjectID(ctx, proj.ID)
	if err != nil {
		return err
	}
	contentByOutline := make(map[int64]*model.BidGenChapterContent, len(chapterContents))
	for _, content := range chapterContents {
		contentByOutline[content.OutlineID] = content
	}
	completed := 0
	for _, node := range targets {
		content := contentByOutline[node.ID]
		if content != nil && content.GenTaskID == task.ID && node.GenStatus == OutlineGenSucceeded {
			completed++
		}
	}
	if completed != int(task.CompletedCount) {
		_ = s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{
			"completed_count": int32(completed), "progress": progressPct(completed, len(targets)),
		})
	}

	globalCtx := s.buildGlobalContext(ctx, proj)
	wordTargets := allocateChapterWords(ordered, task.LengthTier)
	failedCount := 0
	for _, node := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		content := contentByOutline[node.ID]
		if content != nil && content.GenTaskID == task.ID && node.GenStatus == OutlineGenSucceeded {
			continue
		}
		if err := s.repo.UpdateOutlineNode(ctx, node.ID, map[string]interface{}{"gen_status": OutlineGenGenerating}); err != nil {
			return err
		}
		if err := s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{
			"status": "running", "current_outline_id": node.ID,
			"completed_count": int32(completed), "progress": progressPct(completed, len(targets)), "heartbeat_at": time.Now(),
		}); err != nil {
			return err
		}
		s.clearGenerationPartial(ctx, task.ID)
		_, _ = s.publishGenerationEvent(ctx, task.ID, "chapter_start", sseChapterStart{OutlineID: node.ID, Title: node.Title})

		chapterJSON, chapterErr := s.generateChapter(ctx, proj, node, ordered, wordTargets, globalCtx, task.LengthTier, task.GenMode, task.Instruction, task.ID, completed+1, len(targets))
		if chapterErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Warnw("章节生成失败", "project_id", proj.ID, "task_id", task.ID, "outline_id", node.ID, "err", chapterErr)
			if isRetryableGenerationError(chapterErr) {
				_ = s.repo.UpdateOutlineNode(ctx, node.ID, map[string]interface{}{"gen_status": OutlineGenPending})
				return chapterErr
			}
			if repoLLM.IsLLMError(chapterErr) || repoLLM.IsProviderRejection(chapterErr) {
				return permanentGeneration(chapterErr)
			}
			if !isPermanentGenerationError(chapterErr) {
				_ = s.repo.UpdateOutlineNode(ctx, node.ID, map[string]interface{}{"gen_status": OutlineGenPending})
				return chapterErr
			}
			failedCount++
			completed++
			friendly := repoLLM.FriendlyMessage(chapterErr)
			_ = s.repo.UpdateOutlineNode(ctx, node.ID, map[string]interface{}{"gen_status": OutlineGenFailed})
			_ = s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{
				"current_outline_id": node.ID, "completed_count": int32(completed),
				"progress": progressPct(completed, len(targets)), "heartbeat_at": time.Now(),
			})
			_, _ = s.publishGenerationEvent(ctx, task.ID, "chapter_error", sseChapterError{OutlineID: node.ID, Msg: friendly})
		} else {
			completed++
			s.clearGenerationPartial(ctx, task.ID)
			_, _ = s.publishGenerationEvent(ctx, task.ID, "chapter_done", sseChapterDone{OutlineID: node.ID, JSON: chapterJSON})
		}
		_, _ = s.publishGenerationEvent(ctx, task.ID, "progress", sseProgress{
			Current: completed, Total: len(targets), Pct: progressPct(completed, len(targets)),
		})
	}
	if failedCount > 0 {
		msg := fmt.Sprintf("%d 个章节生成失败", failedCount)
		_ = s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{"error_msg": msg})
		return permanentGeneration(errors.New(msg))
	}

	// 篇幅达标：整篇撰写完成后若低于档位下限，做一轮有界的定向扩写补齐
	if task.GenMode == GenModeWrite {
		if err := s.topUpGenerationShortfall(ctx, proj, task, ordered, wordTargets); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Warnw("标书篇幅补齐未完成", "project_id", proj.ID, "task_id", task.ID, "err", err)
		}
	}

	freshOutline, err := s.repo.GetOutlineByProjectID(ctx, proj.ID)
	if err != nil {
		return err
	}
	overallCompleted, overallTotal, overallPct := generationCompletion(freshOutline)
	projectStatus := ProjectStatusDraft
	if overallTotal > 0 && overallCompleted == overallTotal {
		projectStatus = ProjectStatusSucceeded
	}
	if err := s.repo.FinishGeneration(ctx, proj.ID, map[string]interface{}{
		"status": projectStatus, "stage": "", "progress": overallPct, "last_error": "",
	}, task.ID, map[string]interface{}{
		"status": "succeeded", "progress": 100, "completed_count": int32(len(targets)),
		"heartbeat_at": time.Now(), "finished_at": time.Now(),
	}); err != nil {
		return err
	}
	_, _ = s.publishGenerationEvent(ctx, task.ID, "done", map[string]any{"taskId": task.ID})
	return nil
}

func (s *svcImpl) watchGenerationCancellation(ctx context.Context, cancel context.CancelFunc, projectID, taskID int64, done chan<- struct{}) {
	defer close(done)
	pubsub := s.redisSvc.Client().Subscribe(ctx, generationCancelChannel(taskID))
	defer pubsub.Close()
	messages := pubsub.Channel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-messages:
			cancel()
			return
		case <-ticker.C:
			if isCancelRequested(ctx, s, projectID, taskID) {
				cancel()
				return
			}
		}
	}
}

func (s *svcImpl) heartbeatGenerationTask(ctx context.Context, taskID, userID, projectID int64, queueTaskID string, visibilitySec int, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			_ = s.repo.UpdateTaskFields(ctx, taskID, map[string]interface{}{"heartbeat_at": now})
			s.renewUserGenerationSlot(ctx, userID, queueTaskID)
			s.renewGenerationExecutionLock(ctx, projectID, queueTaskID, visibilitySec)
		}
	}
}

func (s *svcImpl) acquireGenerationExecutionLock(ctx context.Context, projectID int64, owner string, visibilitySec int) (func(), bool, error) {
	if visibilitySec <= 0 {
		visibilitySec = 21600
	}
	key := fmt.Sprintf("lock:bid_gen_generate:%d", projectID)
	acquired, err := s.redisSvc.Client().SetNX(ctx, key, owner, time.Duration(visibilitySec)*time.Second).Result()
	if err != nil {
		return func() {}, false, err
	}
	release := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = releaseGenerationLockScript.Run(ctx, s.redisSvc.Client(), []string{key}, owner).Err()
	}
	return release, acquired, nil
}

func (s *svcImpl) renewGenerationExecutionLock(ctx context.Context, projectID int64, owner string, visibilitySec int) {
	if visibilitySec <= 0 {
		visibilitySec = 21600
	}
	key := fmt.Sprintf("lock:bid_gen_generate:%d", projectID)
	_ = renewGenerationLockScript.Run(ctx, s.redisSvc.Client(), []string{key}, owner, visibilitySec).Err()
}

var releaseGenerationLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

var renewGenerationLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

func (s *svcImpl) acquireUserGenerationSlot(ctx context.Context, userID int64, member string) (func(), bool, error) {
	limit := 10
	if s.quotaResolver != nil {
		limit = s.quotaResolver.Limit(ctx, userID)
	}
	lease := bidGenConfigInt("bid_generation.user_slot_lease_sec", 90, 60, 600)
	now := time.Now().Unix()
	key := fmt.Sprintf("bidgen:user-slots:%d", userID)
	result, err := acquireUserSlotScript.Run(ctx, s.redisSvc.Client(), []string{key}, now, now+int64(lease), limit, member).Int()
	if err != nil {
		return func() {}, false, err
	}
	release := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.redisSvc.Client().ZRem(ctx, key, member).Err()
	}
	return release, result == 1, nil
}

func (s *svcImpl) renewUserGenerationSlot(ctx context.Context, userID int64, member string) {
	lease := bidGenConfigInt("bid_generation.user_slot_lease_sec", 90, 60, 600)
	key := fmt.Sprintf("bidgen:user-slots:%d", userID)
	_ = s.redisSvc.Client().ZAddXX(ctx, key, goredisZ(time.Now().Add(time.Duration(lease)*time.Second), member)).Err()
}

func goredisZ(expiry time.Time, member string) goredis.Z {
	return goredis.Z{Score: float64(expiry.Unix()), Member: member}
}

var acquireUserSlotScript = goredis.NewScript(`
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
if redis.call("ZSCORE", KEYS[1], ARGV[4]) then
  redis.call("ZADD", KEYS[1], ARGV[2], ARGV[4])
  return 1
end
if redis.call("ZCARD", KEYS[1]) >= tonumber(ARGV[3]) then
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[2], ARGV[4])
return 1
`)

func isRetryableGenerationError(err error) bool {
	return errors.Is(err, repoLLM.ErrLLMTimeout) || errors.Is(err, repoLLM.ErrLLMRateLimit) ||
		errors.Is(err, repoLLM.ErrLLMUnavailable) || errors.Is(err, repoLLM.ErrLLMURLUnreachable) ||
		errors.Is(err, context.DeadlineExceeded)
}

func isPermanentGenerationError(err error) bool {
	var permanent interface{ NonRetryable() bool }
	if errors.As(err, &permanent) && permanent.NonRetryable() {
		return true
	}
	return errors.Is(err, repoLLM.ErrLLMNotConfigured) || errors.Is(err, repoLLM.ErrLLMKeyUnauthorized) ||
		errors.Is(err, repoLLM.ErrLLMModelNotFound) || repoLLM.IsProviderRejection(err)
}

func unmarshalIDs(raw string) []int64 {
	var ids []int64
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

func generationQueueTaskID(task *model.BidGenTask) string {
	if task != nil && task.QueueTaskID != nil && strings.TrimSpace(*task.QueueTaskID) != "" {
		return strings.TrimSpace(*task.QueueTaskID)
	}
	if task == nil {
		return ""
	}
	return "bid_gen_generate_" + strconv.FormatInt(task.ID, 10)
}

// HandleGenerationTaskFinal 将队列最终失败同步为数据库业务终态。
func (s *svcImpl) HandleGenerationTaskFinal(ctx context.Context, queueTask *taskqueue.Task, succeeded bool) {
	if succeeded {
		return
	}
	var payload GenerateTaskPayload
	if json.Unmarshal([]byte(queueTask.Payload), &payload) != nil || payload.BidTaskID <= 0 {
		return
	}
	task, err := s.repo.GetTaskByID(ctx, payload.BidTaskID)
	if err != nil || task == nil || isGenerationTerminal(task.Status) {
		return
	}
	if task.Status == "cancelling" || isCancelRequested(ctx, s, task.ProjectID, task.ID) {
		s.finishCancelledGeneration(task.ProjectID, task.ID, int(task.CompletedCount))
		_, _ = s.publishGenerationEvent(ctx, task.ID, "cancelled", map[string]any{"taskId": task.ID})
		return
	}
	msg := strings.TrimSpace(task.ErrorMsg)
	if msg == "" || strings.Contains(msg, "正在自动重试") {
		msg = "生成任务多次重试后失败"
	}
	s.failGenerationTask(ctx, task, msg)
	_, _ = s.publishGenerationEvent(ctx, task.ID, "error", sseError{Msg: msg})
}

// StartGenerationReconciler 修复数据库活动任务与 Redis 队列之间的短暂不一致。
func (s *svcImpl) StartGenerationReconciler(ctx context.Context) {
	reconcile := func() { s.reconcileGenerationTasks(ctx) }
	reconcile()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}

func (s *svcImpl) reconcileGenerationTasks(ctx context.Context) {
	tasks, err := s.repo.ListActiveGenerationTasks(ctx)
	if err != nil {
		s.logger.Warnw("扫描标书生成活动任务失败", "err", err)
		return
	}
	for _, task := range tasks {
		if task.Status == "cancelling" && (task.HeartbeatAt == nil || time.Since(*task.HeartbeatAt) > 2*time.Minute) {
			s.finishCancelledGeneration(task.ProjectID, task.ID, int(task.CompletedCount))
			_, _ = s.publishGenerationEvent(ctx, task.ID, "cancelled", map[string]any{"taskId": task.ID})
			continue
		}
		queueID := generationQueueTaskID(task)
		queueMeta, queueErr := s.taskqueueRepo.GetTask(ctx, queueID)
		if queueErr == nil && queueMeta != nil && (queueMeta.Status == taskqueue.TaskStatusPending || queueMeta.Status == taskqueue.TaskStatusProcessing) {
			continue
		}
		if task.Status == "running" && task.HeartbeatAt != nil && time.Since(*task.HeartbeatAt) <= 2*time.Minute {
			continue
		}
		if queueErr == nil && queueMeta != nil {
			queueID = fmt.Sprintf("bid_gen_generate_%d_recover_%d", task.ID, time.Now().Unix())
		}
		_ = s.repo.UpdateTaskFields(ctx, task.ID, map[string]interface{}{
			"queue_task_id": queueID, "status": "pending", "heartbeat_at": nil,
		})
		s.resetGeneratingOutlines(ctx, task.ProjectID)
		if _, err := s.taskqueueRepo.Enqueue(ctx, "bid_gen_generate", GenerateTaskPayload{BidTaskID: task.ID}, taskqueue.EnqueueOpts{
			TaskID: queueID, ProjectID: task.ProjectID, UserID: task.UserID,
		}); err != nil {
			s.logger.Warnw("恢复标书生成任务入队失败", "task_id", task.ID, "err", err)
		}
	}
}
