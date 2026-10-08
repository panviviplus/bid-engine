package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	repollm "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
)

type ParsePayload struct {
	ProjectID  int64  `json:"project_id"`
	RunID      int64  `json:"run_id"`
	UserID     int64  `json:"user_id"`
	StartStage string `json:"start_stage,omitempty"`
}
type ParseTaskHandler struct{ Svc *Service }

type runControlAppliedError struct {
	control *model.BidAnalysisV3RunControl
}

func (e *runControlAppliedError) Error() string {
	return fmt.Sprintf("运行控制已生效: %s/%s", e.control.Action, e.control.Mode)
}

func (h *ParseTaskHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload ParsePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("解析 tender_parse_v3 payload 失败: %w", err)
	}
	if cancelled, _ := h.Svc.redis.Client().Exists(ctx, fmt.Sprintf("cancel:tender_parse_v3:%d", payload.ProjectID)).Result(); cancelled > 0 {
		return nil
	}
	if _, err := h.Svc.repo.Project(ctx, payload.ProjectID, 0); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	latest, err := h.Svc.repo.CurrentRun(ctx, payload.ProjectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if latest.ID != payload.RunID {
		// 用户已创建更新的运行，旧 Redis 重试任务直接终止，禁止覆盖新结果。
		return nil
	}
	if latest.Status != repov3.ProjectRunning {
		// 失败任务会留在 DLQ 供排障。只有显式恢复流程将运行重新置为 running 后，
		// 队列任务才允许继续，避免旧任务被人工重放后覆盖已经完成或仍待处理的结果。
		return nil
	}
	startStage := payload.StartStage
	if startStage == "" {
		startStage = repov3.Stages[0].Name
	}
	if startStage != latest.Stage {
		// 同一运行的旧任务不得重新执行 payload 中的过期阶段；服务重启或
		// 跳过落库后统一以持久化阶段恢复，活动控制会在阶段入口幂等完成。
		startStage = latest.Stage
	}
	lockKey := fmt.Sprintf("lock:tender_parse_v3:%d", payload.ProjectID)
	ttl := time.Duration(task.VisibilitySec) * time.Second
	if ttl <= 0 {
		ttl = 3 * time.Hour
	}
	acquired, err := h.Svc.redis.Client().SetNX(ctx, lockKey, task.ID, ttl).Result()
	if err != nil {
		return err
	}
	if !acquired {
		// 锁可能属于已终止/失败/被僵尸回收的任务（进程被杀、租约过期后同任务重放等）。
		// 持有者已非活跃任务时视为过期锁，清除后重试，避免僵尸锁阻塞项目直到 TTL 到期。
		if h.Svc.isStaleParseLock(ctx, lockKey, task.ID) {
			if delErr := h.Svc.redis.Client().Del(ctx, lockKey).Err(); delErr == nil {
				acquired, err = h.Svc.redis.Client().SetNX(ctx, lockKey, task.ID, ttl).Result()
				if err != nil {
					return err
				}
			}
		}
		if !acquired {
			return fmt.Errorf("项目 %d 已有 V3 解析任务", payload.ProjectID)
		}
	}
	releaseLock := func() {
		if delErr := h.Svc.redis.Client().Del(context.WithoutCancel(ctx), lockKey).Err(); delErr != nil {
			h.Svc.logger.Warnw("释放V3解析运行锁失败", "project_id", payload.ProjectID, "run_id", payload.RunID, "err", delErr)
		}
	}
	// 僵尸回收/进程中断后重放的旧任务可能遗留当前阶段的部分子任务与产物：
	// 先幂等清理，避免 stage_task 唯一键冲突与候选重复，再进入流水线。
	if err := h.Svc.repo.PrepareStageRerun(ctx, payload.ProjectID, payload.RunID, startStage); err != nil {
		return err
	}
	for {
		err = h.Svc.RunPipelineFrom(ctx, payload.ProjectID, payload.RunID, payload.UserID, startStage)
		var controlled *runControlAppliedError
		if !errors.As(err, &controlled) {
			break
		}
		if controlled.control.Action == repov3.ControlActionPause {
			releaseLock()
			return nil
		}
		if controlled.control.ResumeStage == repov3.PipelineCompleteStage {
			releaseLock()
			return h.Svc.repo.CompleteRun(context.WithoutCancel(ctx), payload.ProjectID, payload.RunID)
		}
		startStage = controlled.control.ResumeStage
	}
	releaseLock()
	if errors.Is(err, repov3.ErrRunSuperseded) {
		return nil
	}
	if cancelled, _ := h.Svc.redis.Client().Exists(ctx, fmt.Sprintf("cancel:tender_parse_v3:%d", payload.ProjectID)).Result(); cancelled > 0 {
		return nil
	}
	return err
}

// isStaleParseLock 判断解析运行锁是否过期：持有者任务已不存在（redis.Nil）、
// 已进入终态（failed/cancelled），或锁持有者就是当前任务自身（僵尸回收后同任务重放）。
// Redis 其它异常保守返回 false，避免在 Redis 抖动时误删有效锁。
func (s *Service) isStaleParseLock(ctx context.Context, lockKey, taskID string) bool {
	holder, err := s.redis.Client().Get(ctx, lockKey).Result()
	if err != nil {
		return false
	}
	holderStatus, statusErr := s.redis.Client().HGet(ctx, "task:"+holder, "status").Result()
	return parseLockStale(holder, taskID, holderStatus, statusErr)
}

// parseLockStale 锁过期的纯判定（便于单测）：
//   - 锁值等于当前任务 ID：僵尸回收后同一任务重放，旧锁必然过期；
//   - 持有者任务不存在（redis.Nil）：进程已清理任务，锁过期；
//   - 持有者状态既非 processing 也非 pending：任务已进入终态，锁过期；
//   - 其它 Redis 错误：保守视为未过期。
func parseLockStale(holder, taskID, holderStatus string, holderErr error) bool {
	if holder == "" {
		// 没有锁值可清理；由调用方直接走 SetNX 重试。
		return false
	}
	if holder == taskID {
		return true
	}
	if holderErr != nil {
		return errors.Is(holderErr, goredis.Nil)
	}
	return holderStatus != "processing" && holderStatus != "pending"
}

func (s *Service) enqueue(ctx context.Context, projectID, runID, userID int64, high bool, startStage string) (string, error) {
	priority := taskqueue.TaskPriorityNormal
	if high {
		priority = taskqueue.TaskPriorityHigh
	}
	return s.queue.Enqueue(ctx, queueType, ParsePayload{ProjectID: projectID, RunID: runID, UserID: userID, StartStage: startStage}, taskqueue.EnqueueOpts{Priority: priority, ProjectID: projectID, UserID: userID})
}

func (s *Service) RunPipeline(ctx context.Context, projectID, runID, userID int64) error {
	return s.RunPipelineFrom(ctx, projectID, runID, userID, "document_preprocessing")
}

func (s *Service) RunPipelineFrom(ctx context.Context, projectID, runID, userID int64, startStage string) error {
	if startStage == "" {
		startStage = "document_preprocessing"
	}
	project, err := s.repo.Project(ctx, projectID, 0)
	if err != nil {
		return err
	}
	run, err := s.repo.Run(ctx, runID)
	if err != nil {
		return err
	}
	if run.ProjectID != projectID {
		return fmt.Errorf("解析运行不属于项目")
	}
	if startStage == repov3.PipelineCompleteStage {
		return s.repo.CompleteRun(ctx, projectID, runID)
	}
	startIndex := repov3.StageIndex(startStage)
	if startIndex < 0 {
		return fmt.Errorf("未知流水线起始阶段: %s", startStage)
	}
	workDir, err := os.MkdirTemp("", "bid-analysis-v3-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	fail := func(stage string, cause error) error {
		var controlled *runControlAppliedError
		if errors.As(cause, &controlled) {
			return cause
		}
		if dbErr := s.repo.FailRun(ctx, projectID, runID, stage, cause); dbErr != nil {
			if errors.Is(dbErr, repov3.ErrRunControlPending) {
				control, controlErr := s.applyPendingControlBeforeStage(ctx, runID)
				if controlErr != nil {
					return errors.Join(cause, controlErr)
				}
				if control != nil {
					return &runControlAppliedError{control: control}
				}
			}
			s.logger.Errorw("标记V3运行失败", "project_id", projectID, "run_id", runID, "err", dbErr)
			return errors.Join(cause, fmt.Errorf("保存失败运行状态: %w", dbErr))
		}
		return cause
	}
	var chunks []*model.BidAnalysisV3DocumentChunk
	if startIndex <= repov3.StageIndex("document_preprocessing") {
		err = s.runControlledStage(ctx, runID, "document_preprocessing", func(stageCtx context.Context) error {
			chunks, err = s.prepareDocument(stageCtx, project, run, workDir)
			return err
		})
		if err != nil {
			return fail("document_preprocessing", err)
		}
	} else if startIndex <= repov3.StageIndex("document_parsing") {
		chunks, err = s.repo.Chunks(ctx, runID)
		if err != nil || len(chunks) == 0 {
			if err == nil {
				err = fmt.Errorf("当前运行没有可复用的文档页块")
			}
			return fail("document_parsing", err)
		}
	}
	if startIndex <= repov3.StageIndex("document_parsing") {
		if err = s.runControlledStage(ctx, runID, "document_parsing", func(stageCtx context.Context) error {
			return s.parseAllChunks(stageCtx, projectID, runID, chunks, workDir)
		}); err != nil {
			return fail("document_parsing", err)
		}
	}
	if startIndex <= repov3.StageIndex("document_summary") {
		if err = s.runControlledStage(ctx, runID, "document_summary", func(stageCtx context.Context) error {
			return s.generateDocumentSummary(stageCtx, projectID, runID, userID)
		}); err != nil {
			// 内容类摘要失败在 generateDocumentSummary 内部已降级为告警并标记 partial；
			// 只有基础设施类失败（LLM 不可达/429/504）会冒泡到这里，按整阶段失败处理（告警已在阶段内落库）。
			return fail("document_summary", err)
		}
	}
	var chapters []*model.BidAnalysisV3Chapter
	if startIndex <= repov3.StageIndex("chapter_identifying") {
		err = s.runControlledStage(ctx, runID, "chapter_identifying", func(stageCtx context.Context) error {
			chapters, err = s.identifyChapters(stageCtx, projectID, runID, userID)
			return err
		})
		if err != nil {
			return fail("chapter_identifying", err)
		}
	} else if startIndex <= repov3.StageIndex("chapter_fact_extracting") {
		chapters, err = s.repo.Chapters(ctx, runID)
		if err != nil {
			return fail("chapter_fact_extracting", err)
		}
	}
	if startIndex <= repov3.StageIndex("chapter_fact_extracting") {
		tables, tableErr := s.repo.Tables(ctx, runID)
		if tableErr != nil {
			return fail("chapter_fact_extracting", tableErr)
		}
		if err = s.runControlledStage(ctx, runID, "chapter_fact_extracting", func(stageCtx context.Context) error {
			return s.extractAllChapters(stageCtx, projectID, runID, userID, chapters, tables)
		}); err != nil {
			return fail("chapter_fact_extracting", err)
		}
	}
	if startIndex <= repov3.StageIndex("content_consolidating") {
		if err = s.runControlledStage(ctx, runID, "content_consolidating", func(stageCtx context.Context) error {
			return s.finalizeConsolidation(stageCtx, projectID, runID)
		}); err != nil {
			return fail("content_consolidating", err)
		}
	}
	if err := s.repo.CompleteRun(ctx, projectID, runID); err != nil {
		if !errors.Is(err, repov3.ErrRunControlPending) {
			return err
		}
		control, controlErr := s.applyPendingControlBeforeStage(ctx, runID)
		if controlErr != nil {
			return controlErr
		}
		if control != nil {
			return &runControlAppliedError{control: control}
		}
		return err
	}
	return nil
}

func (s *Service) runControlledStage(ctx context.Context, runID int64, stage string, run func(context.Context) error) error {
	if controlled, err := s.applyPendingControlBeforeStage(ctx, runID); err != nil || controlled != nil {
		if err != nil {
			return err
		}
		return &runControlAppliedError{control: controlled}
	}

	stageCtx, cancelStage := context.WithCancel(ctx)
	defer cancelStage()
	watchDone := make(chan struct{})
	var watchWG sync.WaitGroup
	watchWG.Add(1)
	go func() {
		defer watchWG.Done()
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-watchDone:
				return
			case <-stageCtx.Done():
				return
			case <-ticker.C:
				control, err := s.repo.ActiveRunControl(stageCtx, runID)
				if err == nil {
					lateAfterStagePause := control.Action == repov3.ControlActionPause &&
						control.Mode == repov3.ControlModeAfterStage &&
						repov3.StageIndex(control.TargetStage) < repov3.StageIndex(stage)
					if control.Mode == repov3.ControlModeDiscardCurrent || lateAfterStagePause {
						cancelStage()
						return
					}
				}
			}
		}
	}()

	runErr := run(stageCtx)
	close(watchDone)
	watchWG.Wait()

	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cleanupCancel()
	control, controlErr := s.repo.ActiveRunControl(cleanupCtx, runID)
	if controlErr == nil {
		if control.Mode == repov3.ControlModeDiscardCurrent {
			applied, err := s.repo.ApplyDiscardControl(cleanupCtx, control.ID)
			if err != nil {
				return err
			}
			return &runControlAppliedError{control: applied}
		}
		lateAfterStagePause := control.Action == repov3.ControlActionPause &&
			control.Mode == repov3.ControlModeAfterStage &&
			repov3.StageIndex(control.TargetStage) < repov3.StageIndex(stage)
		if control.Action == repov3.ControlActionPause && control.Mode == repov3.ControlModeAfterStage &&
			((runErr == nil && control.TargetStage == stage) || lateAfterStagePause) {
			applied, err := s.repo.ApplyAfterStagePause(cleanupCtx, control.ID)
			if err != nil {
				return err
			}
			return &runControlAppliedError{control: applied}
		}
	}
	if controlErr != nil && !errors.Is(controlErr, repov3.ErrNoActiveControl) {
		return controlErr
	}
	return runErr
}

func (s *Service) applyPendingControlBeforeStage(ctx context.Context, runID int64) (*model.BidAnalysisV3RunControl, error) {
	control, err := s.repo.ActiveRunControl(ctx, runID)
	if errors.Is(err, repov3.ErrNoActiveControl) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if control.Mode == repov3.ControlModeDiscardCurrent {
		return s.repo.ApplyDiscardControl(cleanupCtx, control.ID)
	}
	if control.Action == repov3.ControlActionPause && control.Mode == repov3.ControlModeAfterStage {
		var stageRun model.BidAnalysisV3StageRun
		err := s.repo.DB().WithContext(cleanupCtx).Where("run_id=? AND stage=?", runID, control.TargetStage).First(&stageRun).Error
		if err != nil {
			return nil, err
		}
		if stageRun.Status == repov3.StageSucceeded || stageRun.Status == repov3.StagePartial || stageRun.Status == repov3.StageSkipped {
			return s.repo.ApplyAfterStagePause(cleanupCtx, control.ID)
		}
	}
	return nil, nil
}

func (s *Service) parseAllChunks(ctx context.Context, projectID, runID int64, chunks []*model.BidAnalysisV3DocumentChunk, workDir string) error {
	if err := s.repo.SetStage(ctx, projectID, runID, "document_parsing", repov3.StageRunning, int32(len(chunks)), 0, 0, ""); err != nil {
		return err
	}
	attempt, err := s.repo.StageAttempt(ctx, runID, "document_parsing")
	if err != nil {
		return err
	}
	sem := make(chan struct{}, s.doclingConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	completed := int32(0)
	for _, chunk := range chunks {
		chunk := chunk
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				failures = append(failures, ctx.Err())
				mu.Unlock()
				return
			}
			defer func() { <-sem }()
			startedAt := time.Now()
			task := &model.BidAnalysisV3StageTask{ProjectID: projectID, RunID: runID, Stage: "document_parsing", TaskType: "docling_chunk", UnitKey: fmt.Sprintf("chunk:%04d:attempt:%02d", chunk.ChunkNo, attempt), PageStart: chunk.PageStart, PageEnd: chunk.PageEnd, Status: "running", Attempts: attempt, PayloadHash: chunk.Checksum, StartedAt: &startedAt}
			if err := s.repo.CreateStageTask(ctx, task); err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				return
			}
			err := s.parseChunk(ctx, projectID, runID, chunk, workDir)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, fmt.Errorf("页 %d-%d: %w", chunk.PageStart, chunk.PageEnd, err))
				if updateErr := s.repo.UpdateChunkFailure(ctx, chunk.ID, 3, err); updateErr != nil {
					failures = append(failures, updateErr)
				}
				if updateErr := s.repo.UpdateStageTask(ctx, task.ID, map[string]any{"status": "failed", "attempts": 3, "last_error": err.Error(), "completed_at": time.Now()}); updateErr != nil {
					failures = append(failures, updateErr)
				}
			} else {
				if updateErr := s.repo.UpdateStageTask(ctx, task.ID, map[string]any{"status": "succeeded", "completed_at": time.Now()}); updateErr != nil {
					failures = append(failures, updateErr)
					return
				}
				completed++
				if updateErr := s.repo.SetStage(ctx, projectID, runID, "document_parsing", repov3.StageRunning, int32(len(chunks)), completed, int32(len(failures)), ""); updateErr != nil {
					failures = append(failures, updateErr)
				}
			}
		}()
	}
	wg.Wait()
	if len(failures) > 0 {
		message := fmt.Sprintf("%d/%d 个 Docling 页块最终失败，全文解析不完整：%v", len(failures), len(chunks), failures[0])
		if err := s.repo.SetStage(ctx, projectID, runID, "document_parsing", repov3.StageFailed, int32(len(chunks)), completed, int32(len(failures)), message); err != nil {
			return errors.Join(fmt.Errorf("%s", message), err)
		}
		return fmt.Errorf("%s", message)
	}
	return s.repo.SetStage(ctx, projectID, runID, "document_parsing", repov3.StageSucceeded, int32(len(chunks)), int32(len(chunks)), 0, "")
}

func (s *Service) extractAllChapters(ctx context.Context, projectID, runID, userID int64, chapters []*model.BidAnalysisV3Chapter, tables []*model.BidAnalysisV3SourceTable) error {
	if len(chapters) == 0 {
		return fmt.Errorf("没有可提取的章节")
	}
	if err := s.repo.SetStage(ctx, projectID, runID, "chapter_fact_extracting", repov3.StageRunning, int32(len(chapters)), 0, 0, ""); err != nil {
		return err
	}
	attempt, err := s.repo.StageAttempt(ctx, runID, "chapter_fact_extracting")
	if err != nil {
		return err
	}
	specs, err := s.loadExtractionSpecs(ctx)
	if err != nil {
		return err
	}
	allBlocks, err := s.repo.Blocks(ctx, runID)
	if err != nil {
		return err
	}
	sources := newEvidenceSourceIndex(allBlocks, tables)
	stageCtx, cancelStage := context.WithCancel(ctx)
	defer cancelStage()
	progressEvents := make(chan bool, len(chapters))
	progressDone := make(chan error, 1)
	go func() {
		progressDone <- runStageProgressReporter(ctx, progressEvents, 5, 2*time.Second, func(completed, failed int) error {
			return s.repo.SetStage(ctx, projectID, runID, "chapter_fact_extracting", repov3.StageRunning, int32(len(chapters)), int32(completed), int32(failed), "")
		})
	}()
	sem := make(chan struct{}, s.llmConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	usable, failed, partial := 0, 0, 0
	var persistenceErr error
	var infraErr error
	var infraChapters []warningChapterRef
	warmupDone := make(chan struct{})
	for chapterIndex, ch := range chapters {
		chapterIndex := chapterIndex
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			if chapterIndex == 0 {
				defer close(warmupDone)
			} else {
				select {
				case <-warmupDone:
				case <-stageCtx.Done():
					mu.Lock()
					failed++
					mu.Unlock()
					progressEvents <- false
					return
				}
			}
			unitSucceeded := false
			defer func() { progressEvents <- unitSucceeded }()
			select {
			case sem <- struct{}{}:
			case <-stageCtx.Done():
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			defer func() { <-sem }()
			err := s.extractChapter(stageCtx, projectID, runID, userID, ch, tables, sources, attempt, specs)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				var partialErr *partialExtractionError
				if errors.As(err, &partialErr) {
					usable++
					partial++
					unitSucceeded = true
					if warningErr := s.repo.AddWarning(ctx, &model.BidAnalysisV3Warning{ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting", Code: "chapter_extract_failed", GroupKey: "code:chapter_extract_failed", Severity: "warning", Message: "部分章节的字段/条款提取未完全成功，已保留可用结果", DetailJSON: buildChapterExtractWarningDetail([]warningChapterRef{{ID: ch.ID, Title: ch.ChapterTitle, PageStart: ch.PageStart, PageEnd: ch.PageEnd}}, fmt.Sprintf("章节“%s”：%s", ch.ChapterTitle, repollm.FriendlyMessage(err)))}); warningErr != nil && persistenceErr == nil {
						persistenceErr = warningErr
					}
					return
				}
				if errors.Is(err, context.Canceled) {
					failed++
					return
				}
				if isLLMStageFatal(err) {
					failed++
					if infraErr == nil {
						infraErr = err
					}
					infraChapters = append(infraChapters, warningChapterRef{ID: ch.ID, Title: ch.ChapterTitle, PageStart: ch.PageStart, PageEnd: ch.PageEnd})
					cancelStage()
					return
				}
				failed++
				if warningErr := s.repo.AddWarning(ctx, &model.BidAnalysisV3Warning{ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting", Code: "chapter_extract_failed", GroupKey: "code:chapter_extract_failed", Severity: "warning", Message: "部分章节提取失败，已保留其它章节的可用结果", DetailJSON: buildChapterExtractWarningDetail([]warningChapterRef{{ID: ch.ID, Title: ch.ChapterTitle, PageStart: ch.PageStart, PageEnd: ch.PageEnd}}, fmt.Sprintf("章节“%s”：%s", ch.ChapterTitle, repollm.FriendlyMessage(err)))}); warningErr != nil && persistenceErr == nil {
					persistenceErr = warningErr
				}
			} else {
				usable++
				unitSucceeded = true
			}
		}()
	}
	wg.Wait()
	close(progressEvents)
	if progressErr := <-progressDone; progressErr != nil && persistenceErr == nil {
		persistenceErr = progressErr
	}
	if infraErr != nil {
		// 配置/鉴权/基础设施失败：只记一条 critical 告警并停止整阶段，避免失败风暴。
		if warningErr := s.repo.AddWarning(ctx, &model.BidAnalysisV3Warning{ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting", Code: "chapter_extract_failed", GroupKey: llmInfraGroupKey(infraErr), Severity: "critical", Message: "章节提取因 LLM 配置或服务异常中断，可一键重试：" + repollm.FriendlyMessage(infraErr), DetailJSON: buildWarningDetail(infraChapters, nil)}); warningErr != nil {
			return errors.Join(infraErr, warningErr)
		}
		return infraErr
	}
	if persistenceErr != nil {
		return persistenceErr
	}
	if usable == 0 {
		return fmt.Errorf("全部 %d 个章节提取失败", len(chapters))
	}
	fixedFieldsPartial, err := s.extractFixedFields(ctx, projectID, runID, userID, allBlocks, tables, sources, attempt, specs)
	if err != nil {
		return err
	}
	if fixedFieldsPartial {
		partial++
	}
	if err := s.persistConsolidatedFacts(ctx, projectID, runID, userID); err != nil {
		return err
	}
	if err := s.generateAiInterpretations(ctx, projectID, runID, userID); err != nil {
		partial++
	}
	status := repov3.StageSucceeded
	if failed > 0 || partial > 0 {
		status = repov3.StagePartial
	}
	return s.repo.SetStage(ctx, projectID, runID, "chapter_fact_extracting", status, int32(len(chapters)), int32(usable), int32(failed), "")
}

var _ taskqueue.TaskHandler = (*ParseTaskHandler)(nil)
