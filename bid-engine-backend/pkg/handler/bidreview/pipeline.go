package bidreview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
)

// ReviewTaskPayload 审核任务负载
type ReviewTaskPayload struct {
	ProjectID       int64  `json:"project_id"`
	UserID          int64  `json:"user_id"`
	ResumeFromStage string `json:"resume_from_stage,omitempty"`
}

// startAsyncPipeline 队列驱动的 8 阶段审核流水线
//
// 状态机：
//   - resumeFromStage 非空（阶段重试）→ 从指定阶段开始执行
//   - 为空（创建 / Worker 失败重试）→ 自动定位首个 failed 阶段；无 failed 从头开始
//   - 任一阶段失败 → 落 failed 阶段 + 项目 failed + 返回 error 触发 Worker 重试
//   - 用户取消（redis cancel 标记）→ 阶段边界停止，项目置 cancelled，任务进 cancelled 终态
//   - 全部成功 → succeed / progress=100 / stage=completed
func (s *svcImpl) startAsyncPipeline(ctx context.Context, projID, userID int64, resumeFromStage string) error {
	flowCtx := repoLLM.WithUserID(ctx, userID)

	proj, err := s.repo.GetProjectForUser(flowCtx, userID, projID)
	if err != nil {
		return fmt.Errorf("加载项目失败: %w", err)
	}
	if proj == nil {
		return fmt.Errorf("项目不存在: %d", projID)
	}

	stageStatus := parseStageStatusJSON(proj.StageStatus)
	if len(stageStatus) == 0 {
		stageStatus = bidreviewRepo.NewStageStatusMap()
	}
	startStage := bidreviewRepo.ResolveStartStage(stageStatus, resumeFromStage)
	s.logger.Infow("审核流水线启动", "project_id", projID, "start_stage", startStage, "resume_from_stage", resumeFromStage)

	// 生命周期：标记本次执行开始（run_count 由入队侧累加，这里补齐时间戳）
	now := time.Now()
	lifecycleFields := map[string]interface{}{
		"status":      bidreviewRepo.ProjectStatusRunning,
		"started_at":  now,
		"finished_at": nil,
	}
	if err := s.repo.UpdateProjectFields(flowCtx, projID, lifecycleFields); err != nil {
		s.logger.Warnw("更新执行开始时间失败", "project_id", projID, "err", err)
	}

	started := false
	for _, stage := range bidreviewRepo.StageOrder {
		if stage == bidreviewRepo.StageCompleted {
			break
		}
		// 阶段边界检查取消标记：支持运行中取消（任务进入 cancelled 终态，不再重试）
		if s.isCancelled(flowCtx, projID) {
			cancelAt := time.Now()
			cancelledJSON, _ := bidreviewRepo.MarshalStageStatus(stageStatus)
			_ = s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{
				"status":       bidreviewRepo.ProjectStatusCancelled,
				"stage_status": cancelledJSON,
				"finished_at":  cancelAt,
				"cancelled_at": cancelAt,
				"last_error":   "",
			})
			s.logger.Infow("审核任务已取消", "project_id", projID, "stage", stage)
			return taskqueue.Cancelled("用户取消审核任务")
		}
		if !started {
			if startStage != "" && stage != startStage {
				continue
			}
			started = true
		}

		status := stageStatus[stage]
		if status == bidreviewRepo.StageStatusSucceeded || status == bidreviewRepo.StageStatusSkipped {
			continue
		}

		// 重跑/重试前清理该阶段产物，保证幂等
		if err := s.clearStageData(flowCtx, proj, stage); err != nil {
			s.logger.Warnw("阶段数据清理失败（继续执行）", "project_id", projID, "stage", stage, "err", err)
		}

		// 标记 running：同步把项目状态拉回 running，避免出现“failed + running”自相矛盾态
		stageStatus[stage] = bidreviewRepo.StageStatusRunning
		progress := bidreviewRepo.CalculateProgress(stageStatus)
		if err := s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{
			"status":     bidreviewRepo.ProjectStatusRunning,
			"stage":      stage,
			"progress":   progress,
			"last_error": "",
		}); err != nil {
			s.logger.Warnw("更新项目运行状态失败", "project_id", projID, "stage", stage, "err", err)
		}
		stageJSON, _ := bidreviewRepo.MarshalStageStatus(stageStatus)
		_ = s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{"stage_status": stageJSON})
		s.beginStageRun(flowCtx, projID, stage)

		startedAt := time.Now()
		stageErr := s.executeStage(flowCtx, proj, stage)
		if stageErr != nil {
			s.logger.Infow("阶段执行失败", "project_id", projID, "stage", stage, "err", stageErr, "cost_ms", time.Since(startedAt).Milliseconds())
			stageStatus[stage] = bidreviewRepo.StageStatusFailed
			failedJSON, _ := bidreviewRepo.MarshalStageStatus(stageStatus)
			_ = s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{
				"stage_status": failedJSON,
				"status":       bidreviewRepo.ProjectStatusFailed,
				"last_error":   stage + " 失败: " + repoLLM.FriendlyMessage(stageErr),
				"progress":     progress,
			})
			s.finishStageRun(flowCtx, projID, stage, bidreviewRepo.StageStatusFailed, stageErr)
			return fmt.Errorf("阶段 %s 失败: %w", stage, stageErr)
		}

		stageStatus[stage] = bidreviewRepo.StageStatusSucceeded
		progress = bidreviewRepo.CalculateProgress(stageStatus)
		if err := s.repo.UpdateProjectStageStatus(flowCtx, projID, stageStatus, stage, progress); err != nil {
			s.logger.Warnw("更新阶段状态失败", "project_id", projID, "stage", stage, "err", err)
		}
		s.finishStageRun(flowCtx, projID, stage, bidreviewRepo.StageStatusSucceeded, nil)
		s.logger.Infow("阶段完成", "project_id", projID, "stage", stage, "cost_ms", time.Since(startedAt).Milliseconds())
	}

	if bidreviewRepo.HasFailedStage(stageStatus) {
		failedJSON, _ := bidreviewRepo.MarshalStageStatus(stageStatus)
		_ = s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{
			"status":       bidreviewRepo.ProjectStatusFailed,
			"progress":     bidreviewRepo.CalculateProgress(stageStatus),
			"stage_status": failedJSON,
			"last_error":   "存在未成功的审核阶段",
		})
		return fmt.Errorf("存在失败阶段，流水线未完成")
	}

	stats, scoreTotal, scoreMax, statErr := s.buildScorecard(flowCtx, projID)
	if statErr != nil {
		s.logger.Warnw("汇总维度结论失败", "project_id", projID, "err", statErr)
	}
	todoItems, _ := s.repo.CountTodoRemediations(flowCtx, projID)
	total, passed, warning, errCount := int64(0), int64(0), int64(0), int64(0)
	for _, st := range stats {
		total += int64(st.Total)
		passed += int64(st.Passed)
		warning += int64(st.Warning)
		errCount += int64(st.Error + st.NotFound)
	}

	_ = s.repo.UpdateProjectFields(flowCtx, projID, map[string]interface{}{
		"status":        bidreviewRepo.ProjectStatusSucceed,
		"progress":      100,
		"stage":         bidreviewRepo.StageCompleted,
		"finished_at":   time.Now(),
		"last_error":    "",
		"total_items":   total,
		"passed_items":  passed,
		"warning_items": warning,
		"error_items":   errCount,
		"todo_items":    todoItems,
		"scoring_total": scoreTotal,
		"scoring_max":   scoreMax,
	})
	s.logger.Infow("审核流水线完成", "project_id", projID, "items", total, "score", scoreTotal, "score_max", scoreMax)
	return nil
}

// executeStage 按阶段分发执行
func (s *svcImpl) executeStage(ctx context.Context, proj *model.BidReviewV2Project, stage string) error {
	switch stage {
	case bidreviewRepo.StageTenderParse:
		return s.stageTenderParse(ctx, proj)
	case bidreviewRepo.StageBidParse:
		return s.stageBidParse(ctx, proj)
	case bidreviewRepo.StageChecklistBuild:
		return s.stageChecklistBuild(ctx, proj)
	case bidreviewRepo.StageEvidenceMatch:
		return s.stageEvidenceMatch(ctx, proj)
	case bidreviewRepo.StageVerdict:
		return s.stageVerdict(ctx, proj)
	case bidreviewRepo.StageFormatScan:
		return s.stageFormatScan(ctx, proj)
	case bidreviewRepo.StageScoring:
		return s.stageScoring(ctx, proj)
	case bidreviewRepo.StageFinalize:
		return s.stageFinalize(ctx, proj)
	default:
		return fmt.Errorf("未知阶段: %s", stage)
	}
}

// ── 阶段实现 ────────────────────────────────────────────────────

// stageTenderParse 招标文件解析：绑定/触发解析 → 冻结审核依据 → 推断暗标
func (s *svcImpl) stageTenderParse(ctx context.Context, proj *model.BidReviewV2Project) error {
	if usesDirectTenderExtraction(proj) {
		return s.runDirectTenderParse(ctx, proj)
	}
	analysisProjectID, runID, err := s.ensureAnalysisBinding(ctx, proj)
	if err != nil {
		if proj.CreateType == "gen" {
			s.logger.Warnw("来源解析结果不可用，改用审核专用解析", "project_id", proj.ID, "err", err)
			return s.runDirectTenderParse(ctx, proj)
		}
		return err
	}
	proj.AnalysisProjectID = analysisProjectID
	proj.AnalysisRunID = runID
	snap, err := s.freezeAnalysisSnapshot(ctx, proj)
	if err != nil {
		return err
	}
	if !proj.IsAnonymous && snapshotMentionsAnonymous(snap) {
		if err := s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{"is_anonymous": true}); err != nil {
			return err
		}
		proj.IsAnonymous = true
		s.logger.Infow("依据招标文件推断为暗标评审", "project_id", proj.ID)
	}
	s.logger.Infow("招标文件解析与审核依据冻结完成", "project_id", proj.ID,
		"analysis_project_id", proj.AnalysisProjectID, "analysis_run_id", proj.AnalysisRunID,
		"fields", len(snap.Fields), "scoring_rows", len(snap.ScoringRows), "clauses", len(snap.Clauses))
	// 记录本阶段覆盖的招标文件数量，供前端描述工作量
	if files, fErr := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "tender"); fErr == nil {
		_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageTenderParse, map[string]interface{}{
			"total": len(files), "completed": len(files),
		})
	}
	return nil
}

func snapshotMentionsAnonymous(snap *analysisSnapshot) bool {
	probe := func(text string) bool {
		return containsAny(text, []string{"暗标", "匿名评审", "不得出现投标人名称", "技术标匿名"})
	}
	for _, f := range snap.Fields {
		if probe(f.DisplayName + f.Value) {
			return true
		}
	}
	for _, c := range snap.Clauses {
		if probe(c.Title + c.Content) {
			return true
		}
	}
	return false
}

// stageBidParse 投标文件解析入库（转 PDF + 逐页/页块/分块/术语索引）
func (s *svcImpl) stageBidParse(ctx context.Context, proj *model.BidReviewV2Project) error {
	files, err := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "bid")
	if err != nil {
		return fmt.Errorf("查询投标文件失败: %w", err)
	}
	if len(files) == 0 {
		return asNonRetryable(fmt.Errorf("审核项目缺少投标文件"))
	}
	total := int32(len(files))
	done := int32(0)
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageBidParse, map[string]interface{}{
		"total": total, "completed": done, "progress": 0,
	})
	for fileIndex, f := range files {
		target := &parseTarget{FileID: f.ID, FileName: f.FileName, FileObject: f.FileObject, PdfObject: f.PdfObject}
		if err := s.setParsingProgress(ctx, proj.ID, bidreviewRepo.StageBidParse, int32(fileIndex*100/len(files)), "转换投标文件"); err != nil {
			return err
		}

		// 非 PDF 先转 PDF，保证预览页码与解析页码一致
		if !strings.HasSuffix(strings.ToLower(f.FileName), ".pdf") {
			if objectKey, url := s.convertAndUploadPdf(ctx, proj.ID, f); objectKey != "" {
				target.PdfObject = objectKey
				_ = s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{"pdf_object": objectKey, "pdf_url": url})
			}
		} else if f.PdfObject == "" {
			_ = s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{"pdf_object": f.FileObject, "pdf_url": f.FileURL})
			target.PdfObject = f.FileObject
		}
		if err := s.setParsingProgress(ctx, proj.ID, bidreviewRepo.StageBidParse, int32((fileIndex*100+25)/len(files)), "解析投标文件"); err != nil {
			return err
		}

		doc, perr := s.parseTargetDoc(ctx, target)
		if perr != nil {
			_ = s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{"parse_status": "failed", "parse_error": perr.Error()})
			return fmt.Errorf("投标文件解析失败(%s): %w", f.FileName, perr)
		}
		if err := s.setParsingProgress(ctx, proj.ID, bidreviewRepo.StageBidParse, int32((fileIndex*100+85)/len(files)), "保存投标文件索引"); err != nil {
			return err
		}
		payload := buildDocumentPayload(proj.ID, f.ID, doc, s.chunkSize, s.chunkOverlap)
		if err := s.repo.ReplaceDocumentContent(ctx, proj.ID, f.ID, payload); err != nil {
			_ = s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{"parse_status": "failed", "parse_error": err.Error()})
			return fmt.Errorf("投标文件入库失败(%s): %w", f.FileName, err)
		}
		if err := s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{
			"parse_status": "parsed",
			"page_count":   len(doc.Pages),
			"char_count":   totalChars(doc),
			"parse_error":  "",
		}); err != nil {
			return fmt.Errorf("更新投标文件解析状态失败: %w", err)
		}
		s.logger.Infow("投标文件解析完成", "project_id", proj.ID, "file_id", f.ID, "pages", len(doc.Pages),
			"chunks", len(payload.Chunks), "terms", len(payload.Terms))
		done++
		percent := int32(0)
		if total > 0 {
			percent = done * 100 / total
		}
		_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageBidParse, map[string]interface{}{
			"completed": done, "progress": percent,
		})
		if err := s.setParsingProgress(ctx, proj.ID, bidreviewRepo.StageBidParse, percent, "投标文件已入库"); err != nil {
			return err
		}
	}
	return nil
}

func totalChars(doc *parsedDoc) int {
	n := 0
	for _, p := range doc.Pages {
		n += len([]rune(p.Content))
	}
	return n
}

// stageChecklistBuild 生成审核清单
func (s *svcImpl) stageChecklistBuild(ctx context.Context, proj *model.BidReviewV2Project) error {
	count, err := s.buildChecklist(ctx, proj)
	if err != nil {
		return err
	}
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageChecklistBuild, map[string]interface{}{
		"total": count, "completed": count,
	})
	return nil
}

// stageEvidenceMatch 证据召回覆盖度评估（判定阶段按同一召回策略取候选页）
func (s *svcImpl) stageEvidenceMatch(ctx context.Context, proj *model.BidReviewV2Project) error {
	items, err := s.repo.GetChecklistItems(ctx, proj.ID)
	if err != nil {
		return err
	}
	files, err := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "bid")
	if err != nil {
		return err
	}
	fileMap := make(map[int64]*model.BidReviewV2File, len(files))
	for _, f := range files {
		fileMap[f.ID] = f
	}

	matched, missed := int32(0), int32(0)
	for _, item := range items {
		hits, rErr := s.recallForItem(ctx, proj.ID, item, fileMap)
		if rErr != nil {
			return rErr
		}
		if len(hits) > 0 {
			matched++
		} else {
			missed++
		}
	}
	detail := fmt.Sprintf(`{"matched":%d,"missed":%d,"total":%d}`, matched, missed, matched+missed)
	// 语义说明：本阶段"是否跑完"与"召回命中率"是两件事。
	// 命中率（matched/missed）记入 detail_json；阶段本身跑完即 completed == total，
	// 避免出现"已完成却显示 37/43"这类自相矛盾的进度读数。
	if err := s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageEvidenceMatch, map[string]interface{}{
		"total": matched + missed, "completed": matched + missed, "failed": 0, "detail_json": detail,
	}); err != nil {
		s.logger.Warnw("更新证据召回统计失败", "project_id", proj.ID, "err", err)
	}
	s.logger.Infow("证据召回完成", "project_id", proj.ID, "matched", matched, "missed", missed)
	if matched == 0 {
		return fmt.Errorf("投标文件中未召回任何候选证据，请确认投标文件内容与解析结果")
	}
	return nil
}

// stageVerdict 分层判定
func (s *svcImpl) stageVerdict(ctx context.Context, proj *model.BidReviewV2Project) error {
	done, err := s.runVerdictStage(ctx, proj)
	if err != nil {
		_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageVerdict, map[string]interface{}{
			"completed": done,
		})
		return err
	}
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageVerdict, map[string]interface{}{
		"total": done, "completed": done,
	})
	return nil
}

// stageFormatScan 暗标/版式确定性扫描
func (s *svcImpl) stageFormatScan(ctx context.Context, proj *model.BidReviewV2Project) error {
	done, err := s.runFormatScanStage(ctx, proj)
	if err != nil {
		return err
	}
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageFormatScan, map[string]interface{}{
		"total": done, "completed": done,
	})
	return nil
}

// stageScoring 竞争力评分对标汇总
func (s *svcImpl) stageScoring(ctx context.Context, proj *model.BidReviewV2Project) error {
	stats, total, maxScore, err := s.buildScorecard(ctx, proj.ID)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{
		"scoring_total": total,
		"scoring_max":   maxScore,
	}); err != nil {
		return err
	}
	detail := "{}"
	if raw, mErr := jsonMarshal(stats); mErr == nil {
		detail = raw
	}
	// total 用"本轮对标的评分项数量"，让前端能描述工作量（而不是一个没有意义的 1）
	scoringItems := int32(0)
	for _, st := range stats {
		if st.Dimension == bidreviewRepo.DimensionCompetitiveness {
			scoringItems = st.Total
			break
		}
	}
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageScoring, map[string]interface{}{
		"total": scoringItems, "completed": scoringItems, "detail_json": detail,
	})
	s.logger.Infow("竞争力评分对标完成", "project_id", proj.ID, "score", total, "score_max", maxScore)
	return nil
}

// stageFinalize 汇总项目统计
func (s *svcImpl) stageFinalize(ctx context.Context, proj *model.BidReviewV2Project) error {
	stats, _, _, err := s.buildScorecard(ctx, proj.ID)
	if err != nil {
		return err
	}
	total, passed, warning, errCount := int64(0), int64(0), int64(0), int64(0)
	for _, st := range stats {
		total += int64(st.Total)
		passed += int64(st.Passed)
		warning += int64(st.Warning)
		errCount += int64(st.Error + st.NotFound)
	}
	todo, _ := s.repo.CountTodoRemediations(ctx, proj.ID)
	return s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{
		"total_items": total, "passed_items": passed, "warning_items": warning,
		"error_items": errCount, "todo_items": todo,
	})
}

// ── 阶段数据清理（重跑幂等）──────────────────────────────────────

func (s *svcImpl) clearStageData(ctx context.Context, proj *model.BidReviewV2Project, stage string) error {
	switch stage {
	case bidreviewRepo.StageTenderParse:
		if err := s.repo.DeleteSnapshot(ctx, proj.ID); err != nil {
			return err
		}
		// 招标解析重跑：解绑当前 run（若其失败/被清理），下次绑定会新建 run 重新解析
		if err := s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{
			"analysis_run_id": 0,
		}); err != nil {
			return err
		}
		proj.AnalysisRunID = 0
	case bidreviewRepo.StageBidParse:
		if err := s.repo.DeleteDocumentContentByType(ctx, proj.ID, "bid"); err != nil {
			return err
		}
		files, err := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "bid")
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := s.repo.UpdateFile(ctx, f.ID, map[string]interface{}{
				"parse_status": "pending", "page_count": 0, "char_count": 0, "parse_error": "",
			}); err != nil {
				return err
			}
		}
	case bidreviewRepo.StageChecklistBuild:
		return s.repo.DeleteChecklistItemsBySource(ctx, proj.ID,
			[]string{SourcePreset, SourceAnalysis, SourceRule, SourceFormat})
	case bidreviewRepo.StageVerdict:
		return s.repo.DeleteFindingsByProject(ctx, proj.ID)
	case bidreviewRepo.StageFormatScan:
		return s.repo.DeleteFindingsByDimension(ctx, proj.ID, bidreviewRepo.DimensionFormat)
	}
	return nil
}

// isCancelled 读取取消标记
func (s *svcImpl) isCancelled(ctx context.Context, projectID int64) bool {
	n, err := s.redisSvc.Client().Exists(ctx, cancelKey(projectID)).Result()
	return err == nil && n > 0
}

// cancelKey 取消标记键（与招标解析模块保持一致的命名风格）
func cancelKey(projectID int64) string {
	return fmt.Sprintf("cancel:bid_review:%d", projectID)
}

// clearCancelFlag 清除取消标记（新的一次运行开始前调用）
func (s *svcImpl) clearCancelFlag(ctx context.Context, projectID int64) {
	if err := s.redisSvc.Client().Del(ctx, cancelKey(projectID)).Err(); err != nil {
		s.logger.Warnw("清除取消标记失败", "project_id", projectID, "err", err)
	}
}

// ── 阶段运行记录 ─────────────────────────────────────────────────

func (s *svcImpl) beginStageRun(ctx context.Context, projectID int64, stage string) {
	now := time.Now()
	run := &model.BidReviewV2StageRun{
		ProjectID: projectID, Stage: stage, Status: bidreviewRepo.StageStatusRunning,
		Attempts: 1, StartedAt: &now,
		// detail_json 走 text 列，但仍统一写合法 JSON 文本，避免后续变更列类型时踩空
		DetailJSON: "{}",
	}
	if err := s.repo.UpsertStageRun(ctx, run); err != nil {
		s.logger.Warnw("写入阶段运行记录失败", "project_id", projectID, "stage", stage, "err", err)
		return
	}
	_ = s.repo.UpdateStageRun(ctx, projectID, stage, map[string]interface{}{
		"status":      bidreviewRepo.StageStatusRunning,
		"attempts":    gormExprIncr("attempts", 1),
		"total":       0,
		"completed":   0,
		"failed":      0,
		"progress":    0,
		"detail_json": "{}",
		"started_at":  now,
		"last_error":  "",
		"finished_at": nil,
	})
}

func (s *svcImpl) finishStageRun(ctx context.Context, projectID int64, stage string, status string, stageErr error) {
	now := time.Now()
	fields := map[string]interface{}{
		"status":      status,
		"finished_at": now,
	}
	if stageErr != nil {
		fields["last_error"] = repoLLM.FriendlyMessage(stageErr)
	} else {
		fields["last_error"] = ""
	}
	if err := s.repo.UpdateStageRun(ctx, projectID, stage, fields); err != nil {
		s.logger.Warnw("更新阶段运行记录失败", "project_id", projectID, "stage", stage, "err", err)
	}
}

// parseStageStatusJSON 解析 stage_status JSON
func parseStageStatusJSON(raw string) map[string]string {
	m := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return m
	}
	_ = jsonUnmarshal(raw, &m)
	// 兼容早期阶段命名（source_bind/bid_ingest），避免历史项目无法重跑
	return bidreviewRepo.NormalizeStageStatus(m)
}
