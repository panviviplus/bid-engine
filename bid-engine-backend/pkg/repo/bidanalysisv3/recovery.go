package bidanalysisv3

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

const PipelineCompleteStage = "pipeline_complete"

func StageIndex(stage string) int {
	for i, definition := range Stages {
		if definition.Name == stage {
			return i
		}
	}
	return -1
}

func NextStage(stage string) (string, bool) {
	index := StageIndex(stage)
	if index < 0 || index+1 >= len(Stages) {
		return PipelineCompleteStage, index >= 0
	}
	return Stages[index+1].Name, true
}

// CanSkipStage 仅允许跳过可降级的业务产出阶段。规范化、全文 Docling 索引和章节识别是后续处理的硬前提。
func CanSkipStage(stage string) bool {
	return StageIndex(stage) >= StageIndex("chapter_fact_extracting")
}

func (r *Repository) PrepareStageRetry(ctx context.Context, projectID, runID, operatorID int64, stage string) error {
	if StageIndex(stage) < 0 {
		return fmt.Errorf("未知解析阶段: %s", stage)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRetryableRun(tx, projectID, runID, stage); err != nil {
			return err
		}
		if err := cleanupStageAndDownstream(tx, projectID, runID, stage); err != nil {
			return err
		}
		// 重试必须重置目标阶段及其下游阶段（状态/attempts/计数），并清理这些阶段的旧任务行，
		// 否则 attempt 号不递增会导致 stage_task 的 (run_id, stage, unit_key) 唯一键冲突。
		if err := resetStageRunsFrom(tx, runID, stage); err != nil {
			return err
		}
		if err := tx.Where("run_id=? AND stage IN ?", runID, stageNamesFrom(stage)).Delete(&model.BidAnalysisV3StageTask{}).Error; err != nil {
			return err
		}
		// 目标阶段及下游阶段都会被重新执行，其旧告警视为作废。
		if err := tx.Model(&model.BidAnalysisV3Warning{}).
			Where("run_id=? AND stage IN ? AND resolved=0", runID, stageNamesFrom(stage)).
			Update("resolved", true).Error; err != nil {
			return err
		}
		warningCount, err := countUnresolvedAlerts(tx, runID)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Updates(map[string]any{
			"status": ProjectRunning, "stage": stage, "last_error": "", "completed_at": nil, "warning_count": warningCount,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3Project{}).Where("id=?", projectID).Updates(map[string]any{
			"status": ProjectRunning, "stage": stage, "last_error": "", "warning_count": warningCount,
		}).Error; err != nil {
			return err
		}
		return createRecoveryLog(tx, projectID, runID, operatorID, "retry_stage", stage, "")
	})
}

// PrepareStageRerun 幂等清理“运行中阶段”因进程中断遗留的部分产物与子任务，
// 使重新入队的解析任务可以从该阶段安全重跑（避免 stage_task 唯一键冲突与候选重复）。
// 仅当阶段状态为 running 且存在该阶段的子任务时执行清理；无遗留时直接通过。
// 与 PrepareStageRetry 的差别：不校验项目/运行状态、不修改 run/project 状态，
// 用于解析任务被僵尸回收后由 Handle 自动续跑的场景。
func (r *Repository) PrepareStageRerun(ctx context.Context, projectID, runID int64, stage string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stageRun model.BidAnalysisV3StageRun
		if err := tx.Where("run_id=? AND stage=?", runID, stage).First(&stageRun).Error; err != nil {
			return err
		}
		if stageRun.Status != StageRunning {
			return nil
		}
		var taskCount int64
		if err := tx.Model(&model.BidAnalysisV3StageTask{}).Where("run_id=? AND stage=?", runID, stage).Count(&taskCount).Error; err != nil {
			return err
		}
		if taskCount == 0 {
			return nil
		}
		if err := cleanupStageAndDownstream(tx, projectID, runID, stage); err != nil {
			return err
		}
		if err := resolveWarningsFromStage(tx, runID, stage); err != nil {
			return err
		}
		if err := resetStageRunsFrom(tx, runID, stage); err != nil {
			return err
		}
		return tx.Where("run_id=? AND stage IN ?", runID, stageNamesFrom(stage)).Delete(&model.BidAnalysisV3StageTask{}).Error
	})
}

func (r *Repository) SkipFailedStage(ctx context.Context, projectID, runID, operatorID int64, stage string) (string, error) {
	if !CanSkipStage(stage) {
		return "", fmt.Errorf("%s 是后续解析的基础阶段，不允许跳过", stage)
	}
	next, ok := NextStage(stage)
	if !ok {
		return "", fmt.Errorf("未知解析阶段: %s", stage)
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockFailedRun(tx, projectID, runID, stage); err != nil {
			return err
		}
		if err := cleanupStageAndDownstream(tx, projectID, runID, stage); err != nil {
			return err
		}
		if err := resolveWarningsFromStage(tx, runID, stage); err != nil {
			return err
		}
		if err := resetStageRunsFrom(tx, runID, stage); err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&model.BidAnalysisV3StageRun{}).Where("run_id=? AND stage=?", runID, stage).Updates(map[string]any{
			"status": StageSkipped, "last_error": "", "completed_at": now,
		}).Error; err != nil {
			return err
		}
		warning := &model.BidAnalysisV3Warning{
			ProjectID: projectID, RunID: runID, Stage: stage, Code: "stage_skipped_by_user",
			Severity: "warning", Message: "用户选择跳过失败阶段；后续结果可能不完整，系统已保留该操作记录。",
		}
		if err := tx.Create(warning).Error; err != nil {
			return err
		}
		progress, err := calculateProgress(tx, runID)
		if err != nil {
			return err
		}
		warningCount, err := countUnresolvedAlerts(tx, runID)
		if err != nil {
			return err
		}
		displayStage := next
		if next == PipelineCompleteStage {
			displayStage = stage
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Updates(map[string]any{
			"status": ProjectRunning, "stage": displayStage, "progress": progress, "last_error": "", "completed_at": nil, "warning_count": warningCount,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3Project{}).Where("id=?", projectID).Updates(map[string]any{
			"status": ProjectRunning, "stage": displayStage, "progress": progress, "last_error": "", "warning_count": warningCount,
		}).Error; err != nil {
			return err
		}
		return createRecoveryLog(tx, projectID, runID, operatorID, "skip_stage", stage, next)
	})
	return next, err
}

func lockFailedRun(tx *gorm.DB, projectID, runID int64, stage string) error {
	var project model.BidAnalysisV3Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", projectID).First(&project).Error; err != nil {
		return err
	}
	var latestRunID int64
	if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("project_id=?", projectID).Order("run_no DESC").Limit(1).Select("id").Scan(&latestRunID).Error; err != nil {
		return err
	}
	if latestRunID != runID || project.Status != ProjectFailed || project.Stage != stage {
		return fmt.Errorf("项目状态已变化，请刷新后重试")
	}
	var run model.BidAnalysisV3ParseRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=?", runID, projectID).First(&run).Error; err != nil {
		return err
	}
	if run.Status != ProjectFailed || run.Stage != stage {
		return fmt.Errorf("解析运行不在可恢复的失败阶段")
	}
	var stageRun model.BidAnalysisV3StageRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id=? AND stage=?", runID, stage).First(&stageRun).Error; err != nil {
		return err
	}
	if stageRun.Status != StageFailed {
		return fmt.Errorf("当前阶段不是失败状态")
	}
	return nil
}

// lockRetryableRun 供阶段级重试使用：允许失败项目（当前阶段失败）、
// “完成但有告警”项目（目标阶段 partial/failed/skipped），以及
// 纯完成项目（目标阶段为已跳过的阶段，支持补跑）重跑指定阶段。
func lockRetryableRun(tx *gorm.DB, projectID, runID int64, stage string) error {
	var project model.BidAnalysisV3Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", projectID).First(&project).Error; err != nil {
		return err
	}
	var latestRunID int64
	if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("project_id=?", projectID).Order("run_no DESC").Limit(1).Select("id").Scan(&latestRunID).Error; err != nil {
		return err
	}
	if latestRunID != runID {
		return fmt.Errorf("解析运行已不是当前运行，请刷新后重试")
	}
	if project.Status != ProjectFailed && project.Status != ProjectSucceededWithWarnings && project.Status != ProjectSucceeded {
		return fmt.Errorf("仅失败、完成或有告警的项目可以阶段级重试")
	}
	var run model.BidAnalysisV3ParseRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=?", runID, projectID).First(&run).Error; err != nil {
		return err
	}
	if run.Status != project.Status {
		return fmt.Errorf("解析运行状态已变化，请刷新后重试")
	}
	var stageRun model.BidAnalysisV3StageRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id=? AND stage=?", runID, stage).First(&stageRun).Error; err != nil {
		return err
	}
	if stageRun.Status != StageFailed && stageRun.Status != StagePartial && stageRun.Status != StageSkipped {
		return fmt.Errorf("目标阶段状态不是失败、部分完成或已跳过，无法重试")
	}
	return nil
}

func cleanupStageAndDownstream(tx *gorm.DB, projectID, runID int64, stage string) error {
	index := StageIndex(stage)
	if index <= StageIndex("document_summary") {
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3Summary{}).Error; err != nil {
			return err
		}
	}
	if index <= StageIndex("content_consolidating") {
		if err := cleanupRunAIValues(tx, projectID, runID); err != nil {
			return err
		}
	}
	if index <= StageIndex("chapter_fact_extracting") {
		clauseIDs := tx.Model(&model.BidAnalysisV3Clause{}).Select("id").Where("run_id=?", runID)
		if err := tx.Where("clause_id IN (?)", clauseIDs).Delete(&model.BidAnalysisV3ClauseEvidence{}).Error; err != nil {
			return fmt.Errorf("删除条款证据: %w", err)
		}
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3Clause{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3FactCandidate{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Update("extracted_chapters", 0).Error; err != nil {
			return err
		}
	}
	if index <= StageIndex("chapter_identifying") {
		chapterIDs := tx.Model(&model.BidAnalysisV3Chapter{}).Select("id").Where("run_id=?", runID)
		if err := tx.Where("chapter_id IN (?)", chapterIDs).Delete(&model.BidAnalysisV3ChapterBlock{}).Error; err != nil {
			return fmt.Errorf("删除章节文本块关联: %w", err)
		}
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3Chapter{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Update("total_chapters", 0).Error; err != nil {
			return err
		}
	}
	if index <= StageIndex("document_parsing") {
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3SourceTable{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3DocumentBlock{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3DocumentPage{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3DocumentChunk{}).Where("run_id=?", runID).Updates(map[string]any{
			"status": "pending", "attempts": 0, "docling_asset_id": 0, "text_chars": 0,
			"block_count": 0, "table_count": 0, "processing_ms": 0, "last_error": "",
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Updates(map[string]any{"parsed_pages": 0, "parsed_chunks": 0}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id=? AND asset_type='docling_json'", runID).Delete(&model.BidAnalysisV3DocumentAsset{}).Error; err != nil {
			return err
		}
	}
	if index <= StageIndex("document_preprocessing") {
		if err := tx.Where("run_id=?", runID).Delete(&model.BidAnalysisV3DocumentChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Updates(map[string]any{
			"total_pages": 0, "parsed_pages": 0, "total_chunks": 0, "parsed_chunks": 0,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id=? AND asset_type IN ?", runID, []string{"chunk_pdf", "docling_json"}).Delete(&model.BidAnalysisV3DocumentAsset{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func cleanupRunAIValues(tx *gorm.DB, projectID, runID int64) error {
	var values []*model.BidAnalysisV3FieldValue
	if err := tx.Where("project_id=? AND run_id=? AND origin='ai'", projectID, runID).Find(&values).Error; err != nil {
		return err
	}
	derivedTableIDs := tx.Model(&model.BidAnalysisV3DerivedTable{}).Select("id").Where("project_id=? AND run_id=?", projectID, runID)
	if err := tx.Where("derived_table_id IN (?)", derivedTableIDs).Delete(&model.BidAnalysisV3DerivedTableEvidence{}).Error; err != nil {
		return fmt.Errorf("删除归并表证据: %w", err)
	}
	if err := tx.Where("project_id=? AND run_id=?", projectID, runID).Delete(&model.BidAnalysisV3DerivedTable{}).Error; err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}
	valueIDs := make([]int64, 0, len(values))
	affectedFields := map[int64]bool{}
	for _, value := range values {
		valueIDs = append(valueIDs, value.ID)
		affectedFields[value.FieldID] = true
	}
	if err := tx.Where("field_value_id IN ?", valueIDs).Delete(&model.BidAnalysisV3FieldValueEvidence{}).Error; err != nil {
		return fmt.Errorf("删除字段值证据: %w", err)
	}
	if err := tx.Where("id IN ?", valueIDs).Delete(&model.BidAnalysisV3FieldValue{}).Error; err != nil {
		return err
	}
	for fieldID := range affectedFields {
		var field model.BidAnalysisV3Field
		if err := tx.First(&field, fieldID).Error; err != nil {
			return err
		}
		wasCurrent := false
		for _, valueID := range valueIDs {
			if field.CurrentValueID == valueID {
				wasCurrent = true
				break
			}
		}
		if !wasCurrent {
			continue
		}
		var manual model.BidAnalysisV3FieldValue
		err := tx.Where("field_id=? AND origin='user' AND value_status='active'", fieldID).Order("id DESC").First(&manual).Error
		if err == nil {
			if err := tx.Model(&field).Updates(map[string]any{"current_value_id": manual.ID, "extract_status": "found"}).Error; err != nil {
				return err
			}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Model(&field).Updates(map[string]any{"current_value_id": 0, "extract_status": "not_found"}).Error; err != nil {
			return err
		}
	}
	return nil
}

func createRecoveryLog(tx *gorm.DB, projectID, runID, operatorID int64, action, stage, next string) error {
	detail, err := json.Marshal(map[string]string{"stage": stage, "next_stage": next})
	if err != nil {
		return err
	}
	return tx.Create(&model.BidAnalysisV3OperationLog{
		ProjectID: projectID, RunID: runID, TargetType: "stage", Action: action,
		OperatorID: operatorID, DetailJSON: string(detail),
	}).Error
}
