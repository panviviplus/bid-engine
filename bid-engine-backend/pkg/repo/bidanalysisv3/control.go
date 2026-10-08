package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

const (
	ControlActionPause = "pause"
	ControlActionSkip  = "skip"

	ControlModeAfterStage     = "after_stage"
	ControlModeDiscardCurrent = "discard_current"

	ControlRequested = "requested"
	ControlApplying  = "applying"
	ControlApplied   = "applied"
	ControlCancelled = "cancelled"
	ControlFailed    = "failed"
)

var (
	ErrControlConflict   = errors.New("项目状态已变化，请刷新后重试")
	ErrNoActiveControl   = errors.New("没有待处理的运行控制请求")
	ErrRunControlPending = errors.New("运行控制请求尚未应用")
	ErrControlRejected   = errors.New("运行控制请求不允许执行")
)

type controlRejectedError struct{ message string }

func (e *controlRejectedError) Error() string        { return e.message }
func (e *controlRejectedError) Is(target error) bool { return target == ErrControlRejected }
func rejectControl(message string) error             { return &controlRejectedError{message: message} }

type RequestRunControlInput struct {
	ProjectID     int64
	RunID         int64
	OperatorID    int64
	Action        string
	Mode          string
	ExpectedStage string
}

// RequestRunControl 在锁定项目与当前运行后创建控制请求，确保同一运行最多只有一个活动请求。
func (r *Repository) RequestRunControl(ctx context.Context, in RequestRunControlInput) (*model.BidAnalysisV3RunControl, error) {
	if in.Action != ControlActionPause && in.Action != ControlActionSkip {
		return nil, rejectControl("不支持的运行控制操作")
	}
	if in.Mode != ControlModeAfterStage && in.Mode != ControlModeDiscardCurrent {
		return nil, rejectControl("不支持的运行控制方式")
	}
	if in.Action == ControlActionSkip && in.Mode != ControlModeDiscardCurrent {
		return nil, rejectControl("跳过阶段仅支持立即停止当前阶段")
	}
	if StageIndex(in.ExpectedStage) < 0 {
		return nil, rejectControl(fmt.Sprintf("未知解析阶段: %s", in.ExpectedStage))
	}
	if in.Action == ControlActionSkip && !CanSkipStage(in.ExpectedStage) {
		return nil, rejectControl(fmt.Sprintf("%s 是后续解析的基础阶段，不允许跳过", in.ExpectedStage))
	}
	nextStage, ok := NextStage(in.ExpectedStage)
	if !ok {
		return nil, rejectControl(fmt.Sprintf("未知解析阶段: %s", in.ExpectedStage))
	}
	if in.Action == ControlActionPause && in.Mode == ControlModeAfterStage && nextStage == PipelineCompleteStage {
		return nil, rejectControl("这是最后阶段，完成后解析将直接完成")
	}
	resumeStage := in.ExpectedStage
	if in.Action == ControlActionSkip || in.Mode == ControlModeAfterStage {
		resumeStage = nextStage
	}

	control := &model.BidAnalysisV3RunControl{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, run, err := lockCurrentRun(tx, in.ProjectID, in.RunID)
		if err != nil {
			return err
		}
		if project.Status != ProjectRunning || run.Status != ProjectRunning || project.Stage != in.ExpectedStage || run.Stage != in.ExpectedStage {
			return ErrControlConflict
		}
		var active int64
		if err := tx.Model(&model.BidAnalysisV3RunControl{}).
			Where("run_id=? AND status IN ?", in.RunID, []string{ControlRequested, ControlApplying}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return rejectControl("当前解析已有暂停或跳过请求，请等待处理完成")
		}
		now := time.Now()
		control = &model.BidAnalysisV3RunControl{
			ProjectID: in.ProjectID, RunID: in.RunID, Action: in.Action, Mode: in.Mode,
			TargetStage: in.ExpectedStage, ResumeStage: resumeStage, Status: ControlRequested,
			OperatorID: in.OperatorID, RequestedAt: now,
		}
		if err := tx.Create(control).Error; err != nil {
			return err
		}
		return createControlLog(tx, control, "request_"+in.Action, "")
	})
	return control, err
}

// FailStaleControls 将超过 staleBefore 仍处于 requested/applying 的运行控制标记为失败，
// 用于解析任务已中断（孤儿任务/未运行）时避免暂停/跳过请求无限挂起。
// 返回本次标记失败的条数。
func (r *Repository) FailStaleControls(ctx context.Context, staleBefore time.Time) (int64, error) {
	var controls []*model.BidAnalysisV3RunControl
	if err := r.db.WithContext(ctx).
		Where("status IN ? AND requested_at < ?", []string{ControlRequested, ControlApplying}, staleBefore).
		Find(&controls).Error; err != nil {
		return 0, err
	}
	var affected int64
	for _, control := range controls {
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var current model.BidAnalysisV3RunControl
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, control.ID).Error; err != nil {
				return err
			}
			if current.Status != ControlRequested && current.Status != ControlApplying {
				return nil
			}
			msg := "解析任务未在运行，暂停/跳过未能生效，请刷新后重试"
			if err := tx.Model(&current).Updates(map[string]any{"status": ControlFailed, "last_error": msg}).Error; err != nil {
				return err
			}
			current.Status = ControlFailed
			current.LastError = msg
			return createControlLog(tx, &current, "fail_stale_control", msg)
		}); err != nil {
			return affected, err
		}
		affected++
	}
	return affected, nil
}

func lockCurrentRun(tx *gorm.DB, projectID, runID int64) (*model.BidAnalysisV3Project, *model.BidAnalysisV3ParseRun, error) {
	var project model.BidAnalysisV3Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", projectID).First(&project).Error; err != nil {
		return nil, nil, err
	}
	var latest model.BidAnalysisV3ParseRun
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id=?", projectID).Order("run_no DESC").First(&latest).Error; err != nil {
		return nil, nil, err
	}
	if latest.ID != runID {
		return nil, nil, ErrControlConflict
	}
	return &project, &latest, nil
}

func (r *Repository) ActiveRunControl(ctx context.Context, runID int64) (*model.BidAnalysisV3RunControl, error) {
	var control model.BidAnalysisV3RunControl
	err := r.db.WithContext(ctx).Where("run_id=? AND status IN ?", runID, []string{ControlRequested, ControlApplying}).Order("id DESC").First(&control).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoActiveControl
	}
	return &control, err
}

func (r *Repository) LatestRunControls(ctx context.Context, runIDs []int64) (map[int64]*model.BidAnalysisV3RunControl, error) {
	out := make(map[int64]*model.BidAnalysisV3RunControl)
	if len(runIDs) == 0 {
		return out, nil
	}
	var controls []*model.BidAnalysisV3RunControl
	if err := r.db.WithContext(ctx).Where("run_id IN ?", runIDs).Order("id DESC").Find(&controls).Error; err != nil {
		return nil, err
	}
	for _, control := range controls {
		if _, exists := out[control.RunID]; !exists {
			out[control.RunID] = control
		}
	}
	return out, nil
}

func (r *Repository) CurrentRunsByProjects(ctx context.Context, projectIDs []int64) (map[int64]*model.BidAnalysisV3ParseRun, error) {
	out := make(map[int64]*model.BidAnalysisV3ParseRun)
	if len(projectIDs) == 0 {
		return out, nil
	}
	var runs []*model.BidAnalysisV3ParseRun
	if err := r.db.WithContext(ctx).Where("project_id IN ?", projectIDs).Order("project_id, run_no DESC").Find(&runs).Error; err != nil {
		return nil, err
	}
	for _, run := range runs {
		if _, exists := out[run.ProjectID]; !exists {
			out[run.ProjectID] = run
		}
	}
	return out, nil
}

// ApplyDiscardControl 在所有阶段 goroutine 退出后调用。它会清理当前及下游结果，再应用立即暂停或跳过。
func (r *Repository) ApplyDiscardControl(ctx context.Context, controlID int64) (*model.BidAnalysisV3RunControl, error) {
	var applied *model.BidAnalysisV3RunControl
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		control, project, run, err := lockControlRun(tx, controlID)
		if err != nil {
			return err
		}
		if control.Status == ControlApplied {
			applied = control
			return nil
		}
		if control.Status != ControlRequested && control.Status != ControlApplying {
			return ErrNoActiveControl
		}
		if control.Mode != ControlModeDiscardCurrent {
			return fmt.Errorf("控制请求不是立即停止模式")
		}
		if project.Status != ProjectRunning || run.Status != ProjectRunning {
			return ErrControlConflict
		}
		if err := tx.Model(control).Updates(map[string]any{"status": ControlApplying, "last_error": ""}).Error; err != nil {
			return err
		}
		if err := cleanupStageAndDownstream(tx, control.ProjectID, control.RunID, control.TargetStage); err != nil {
			return err
		}
		stageNames := stageNamesFrom(control.TargetStage)
		now := time.Now()
		if err := cancelStageDiagnostics(tx, control.RunID, stageNames, "运行控制已取消该任务", now); err != nil {
			return err
		}
		if err := resolveWarningsFromStage(tx, control.RunID, control.TargetStage); err != nil {
			return err
		}
		if err := resetStageRunsFrom(tx, control.RunID, control.TargetStage); err != nil {
			return err
		}

		if control.Action == ControlActionPause {
			if err := updatePausedState(tx, control.ProjectID, control.RunID, control.TargetStage); err != nil {
				return err
			}
		} else if control.Action == ControlActionSkip {
			if !CanSkipStage(control.TargetStage) {
				return fmt.Errorf("%s 是后续解析的基础阶段，不允许跳过", control.TargetStage)
			}
			if err := markStageSkipped(tx, control); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("未知控制操作: %s", control.Action)
		}
		if err := tx.Model(control).Updates(map[string]any{"status": ControlApplied, "applied_at": now, "last_error": ""}).Error; err != nil {
			return err
		}
		control.Status = ControlApplied
		control.AppliedAt = &now
		if err := createControlLog(tx, control, "apply_"+control.Action, ""); err != nil {
			return err
		}
		applied = control
		return nil
	})
	if err != nil {
		_ = r.markControlFailed(context.WithoutCancel(ctx), controlID, err)
	}
	return applied, err
}

// ApplyAfterStagePause 在目标阶段成功或部分成功结束、进入下一阶段之前应用暂停。
func (r *Repository) ApplyAfterStagePause(ctx context.Context, controlID int64) (*model.BidAnalysisV3RunControl, error) {
	var applied *model.BidAnalysisV3RunControl
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		control, project, run, err := lockControlRun(tx, controlID)
		if err != nil {
			return err
		}
		if control.Status == ControlApplied {
			applied = control
			return nil
		}
		if control.Action != ControlActionPause || control.Mode != ControlModeAfterStage {
			return fmt.Errorf("控制请求不是完成本阶段后暂停")
		}
		if project.Status != ProjectRunning || run.Status != ProjectRunning {
			return ErrControlConflict
		}
		var stageRun model.BidAnalysisV3StageRun
		if err := tx.Where("run_id=? AND stage=?", control.RunID, control.TargetStage).First(&stageRun).Error; err != nil {
			return err
		}
		if stageRun.Status != StageSucceeded && stageRun.Status != StagePartial && stageRun.Status != StageSkipped {
			return fmt.Errorf("当前阶段尚未完成")
		}
		if control.ResumeStage == PipelineCompleteStage {
			return fmt.Errorf("最后阶段完成后不能暂停")
		}
		now := time.Now()
		// 极小的阶段切换竞态下，下一阶段可能刚刚开始。此时先等调用方收敛该阶段，
		// 再清除下一阶段的部分写入，仍停在原请求约定的下一阶段起点。
		if run.Stage != control.TargetStage {
			if StageIndex(run.Stage) < StageIndex(control.ResumeStage) {
				return ErrControlConflict
			}
			if err := cleanupStageAndDownstream(tx, control.ProjectID, control.RunID, control.ResumeStage); err != nil {
				return err
			}
			stageNames := stageNamesFrom(control.ResumeStage)
			if err := cancelStageDiagnostics(tx, control.RunID, stageNames, "阶段切换时应用暂停请求", now); err != nil {
				return err
			}
			if err := resolveWarningsFromStage(tx, control.RunID, control.ResumeStage); err != nil {
				return err
			}
			if err := resetStageRunsFrom(tx, control.RunID, control.ResumeStage); err != nil {
				return err
			}
		}
		if err := updatePausedState(tx, control.ProjectID, control.RunID, control.ResumeStage); err != nil {
			return err
		}
		if err := tx.Model(control).Updates(map[string]any{"status": ControlApplied, "applied_at": now, "last_error": ""}).Error; err != nil {
			return err
		}
		control.Status = ControlApplied
		control.AppliedAt = &now
		if err := createControlLog(tx, control, "apply_pause", ""); err != nil {
			return err
		}
		applied = control
		return nil
	})
	if err != nil {
		_ = r.markControlFailed(context.WithoutCancel(ctx), controlID, err)
	}
	return applied, err
}

func lockControlRun(tx *gorm.DB, controlID int64) (*model.BidAnalysisV3RunControl, *model.BidAnalysisV3Project, *model.BidAnalysisV3ParseRun, error) {
	var snapshot model.BidAnalysisV3RunControl
	if err := tx.First(&snapshot, controlID).Error; err != nil {
		return nil, nil, nil, err
	}
	project, run, err := lockCurrentRun(tx, snapshot.ProjectID, snapshot.RunID)
	if err != nil {
		return nil, nil, nil, err
	}
	var control model.BidAnalysisV3RunControl
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&control, controlID).Error; err != nil {
		return nil, nil, nil, err
	}
	if control.ProjectID != project.ID || control.RunID != run.ID {
		return nil, nil, nil, ErrControlConflict
	}
	return &control, project, run, err
}

func updatePausedState(tx *gorm.DB, projectID, runID int64, stage string) error {
	progress, err := calculateProgress(tx, runID)
	if err != nil {
		return err
	}
	warningCount, err := countUnresolvedAlerts(tx, runID)
	if err != nil {
		return err
	}
	if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", runID).Updates(map[string]any{
		"status": ProjectPaused, "stage": stage, "progress": progress, "warning_count": warningCount, "last_error": "", "completed_at": nil,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&model.BidAnalysisV3Project{}).Where("id=?", projectID).Updates(map[string]any{
		"status": ProjectPaused, "stage": stage, "progress": progress, "warning_count": warningCount, "last_error": "",
	}).Error
}

func markStageSkipped(tx *gorm.DB, control *model.BidAnalysisV3RunControl) error {
	now := time.Now()
	if err := tx.Model(&model.BidAnalysisV3StageRun{}).Where("run_id=? AND stage=?", control.RunID, control.TargetStage).Updates(map[string]any{
		"status": StageSkipped, "last_error": "", "completed_at": now,
	}).Error; err != nil {
		return err
	}
	warning := &model.BidAnalysisV3Warning{
		ProjectID: control.ProjectID, RunID: control.RunID, Stage: control.TargetStage,
		Code: "stage_skipped_by_user", Severity: "warning",
		Message: "用户在运行中跳过本阶段；该阶段结果已清除，后续结果可能不完整。",
	}
	if err := tx.Create(warning).Error; err != nil {
		return err
	}
	progress, err := calculateProgress(tx, control.RunID)
	if err != nil {
		return err
	}
	warningCount, err := countUnresolvedAlerts(tx, control.RunID)
	if err != nil {
		return err
	}
	displayStage := control.ResumeStage
	if displayStage == PipelineCompleteStage {
		displayStage = control.TargetStage
	}
	updates := map[string]any{"status": ProjectRunning, "stage": displayStage, "progress": progress, "last_error": "", "warning_count": warningCount, "completed_at": nil}
	if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id=?", control.RunID).Updates(updates).Error; err != nil {
		return err
	}
	delete(updates, "completed_at")
	return tx.Model(&model.BidAnalysisV3Project{}).Where("id=?", control.ProjectID).Updates(updates).Error
}

func stageNamesFrom(stage string) []string {
	index := StageIndex(stage)
	if index < 0 {
		return nil
	}
	names := make([]string, 0, len(Stages)-index)
	for _, definition := range Stages[index:] {
		names = append(names, definition.Name)
	}
	return names
}

func resolveWarningsFromStage(tx *gorm.DB, runID int64, stage string) error {
	return tx.Model(&model.BidAnalysisV3Warning{}).
		Where("run_id=? AND stage IN ? AND resolved=0", runID, stageNamesFrom(stage)).
		Update("resolved", true).Error
}

func resetStageRunsFrom(tx *gorm.DB, runID int64, stage string) error {
	return tx.Model(&model.BidAnalysisV3StageRun{}).Where("run_id=? AND stage IN ?", runID, stageNamesFrom(stage)).Updates(map[string]any{
		"status": StagePending, "total_units": 0, "completed_units": 0, "failed_units": 0,
		"warning_count": 0, "last_error": "", "started_at": nil, "completed_at": nil,
	}).Error
}

func cancelStageDiagnostics(tx *gorm.DB, runID int64, stages []string, message string, completedAt time.Time) error {
	taskIDs := tx.Model(&model.BidAnalysisV3StageTask{}).Select("id").Where("run_id=? AND stage IN ?", runID, stages)
	if err := tx.Model(&model.BidAnalysisV3LlmCall{}).
		Where("run_id=? AND task_id IN (?) AND status <> 'cancelled'", runID, taskIDs).
		Updates(map[string]any{"status": "cancelled", "error_code": "run_control_cancelled"}).Error; err != nil {
		return err
	}
	return tx.Model(&model.BidAnalysisV3StageTask{}).
		Where("run_id=? AND stage IN ? AND status <> 'cancelled'", runID, stages).
		Updates(map[string]any{"status": "cancelled", "last_error": message, "completed_at": completedAt}).Error
}

func (r *Repository) markControlFailed(ctx context.Context, controlID int64, cause error) error {
	if cause == nil || errors.Is(cause, ErrNoActiveControl) {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var control model.BidAnalysisV3RunControl
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&control, controlID).Error; err != nil {
			return err
		}
		if control.Status != ControlRequested && control.Status != ControlApplying {
			return nil
		}
		if err := tx.Model(&control).Updates(map[string]any{"status": ControlFailed, "last_error": cause.Error()}).Error; err != nil {
			return err
		}
		control.Status = ControlFailed
		control.LastError = cause.Error()
		return createControlLog(tx, &control, "fail_"+control.Action, cause.Error())
	})
}

// ResumePausedRun 先把暂停运行置回 running；调用方入队失败时必须调用 RestorePausedRun。
func (r *Repository) ResumePausedRun(ctx context.Context, projectID, runID, operatorID int64, expectedStage string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, run, err := lockCurrentRun(tx, projectID, runID)
		if err != nil {
			return err
		}
		if project.Status != ProjectPaused || run.Status != ProjectPaused || project.Stage != expectedStage || run.Stage != expectedStage {
			return ErrControlConflict
		}
		var active int64
		if err := tx.Model(&model.BidAnalysisV3RunControl{}).Where("run_id=? AND status IN ?", runID, []string{ControlRequested, ControlApplying}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return rejectControl("暂停请求仍在处理中，请稍后再继续")
		}
		if err := tx.Model(run).Updates(map[string]any{"status": ProjectRunning, "last_error": "", "completed_at": nil}).Error; err != nil {
			return err
		}
		if err := tx.Model(project).Updates(map[string]any{"status": ProjectRunning, "last_error": ""}).Error; err != nil {
			return err
		}
		return createSimpleControlLog(tx, projectID, runID, operatorID, "resume_run", expectedStage, "")
	})
}

func (r *Repository) RestorePausedRun(ctx context.Context, projectID, runID, operatorID int64, stage string, cause error) error {
	message := "继续解析任务入队失败"
	if cause != nil {
		message = cause.Error()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, run, err := lockCurrentRun(tx, projectID, runID)
		if err != nil {
			return err
		}
		if project.Status != ProjectRunning || run.Status != ProjectRunning || run.Stage != stage {
			return ErrControlConflict
		}
		if err := tx.Model(run).Updates(map[string]any{"status": ProjectPaused, "last_error": ""}).Error; err != nil {
			return err
		}
		if err := tx.Model(project).Updates(map[string]any{"status": ProjectPaused, "last_error": ""}).Error; err != nil {
			return err
		}
		return createSimpleControlLog(tx, projectID, runID, operatorID, "resume_enqueue_failed", stage, message)
	})
}

func createControlLog(tx *gorm.DB, control *model.BidAnalysisV3RunControl, action, message string) error {
	detail, err := json.Marshal(map[string]any{
		"control_id": control.ID, "action": control.Action, "mode": control.Mode,
		"target_stage": control.TargetStage, "resume_stage": control.ResumeStage, "message": message,
	})
	if err != nil {
		return err
	}
	return tx.Create(&model.BidAnalysisV3OperationLog{
		ProjectID: control.ProjectID, RunID: control.RunID, TargetType: "run_control", TargetID: control.ID,
		Action: action, OperatorID: control.OperatorID, DetailJSON: string(detail),
	}).Error
}

func createSimpleControlLog(tx *gorm.DB, projectID, runID, operatorID int64, action, stage, message string) error {
	detail, err := json.Marshal(map[string]string{"stage": stage, "message": message})
	if err != nil {
		return err
	}
	return tx.Create(&model.BidAnalysisV3OperationLog{
		ProjectID: projectID, RunID: runID, TargetType: "run", TargetID: runID,
		Action: action, OperatorID: operatorID, DetailJSON: string(detail),
	}).Error
}
