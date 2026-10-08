package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
)

const (
	ProjectRunning               = "running"
	ProjectPaused                = "paused"
	ProjectSucceeded             = "succeeded"
	ProjectSucceededWithWarnings = "succeeded_with_warnings"
	ProjectFailed                = "failed"

	StagePending   = "pending"
	StageRunning   = "running"
	StageSucceeded = "succeeded"
	StagePartial   = "partial"
	StageFailed    = "failed"
	StageSkipped   = "skipped"
)

var ErrRunSuperseded = errors.New("解析运行已被更新的运行取代")

type StageDefinition struct {
	Name   string
	Weight int32
}

var Stages = []StageDefinition{
	{Name: "document_preprocessing", Weight: 5},
	{Name: "document_parsing", Weight: 35},
	{Name: "document_summary", Weight: 5},
	{Name: "chapter_identifying", Weight: 10},
	{Name: "chapter_fact_extracting", Weight: 35},
	{Name: "content_consolidating", Weight: 10},
}

const (
	FactSubtaskDynamicClauses = "dynamic_clauses"
	FactSubtaskFixedFields    = "fixed_fields"
	FactSubtaskEvidence       = "evidence"
	FactSubtaskAIInterpret    = "ai_interpret"
)

var factSubtaskWeights = map[string]int32{
	FactSubtaskDynamicClauses: 10,
	FactSubtaskFixedFields:    10,
	FactSubtaskEvidence:       10,
	FactSubtaskAIInterpret:    5,
}

var factSubtaskTaskTypes = map[string]string{
	"chapter_extraction":           FactSubtaskDynamicClauses,
	"fixed_fields":                 FactSubtaskFixedFields,
	"fixed_fields_merge":           FactSubtaskEvidence,
	"chapter_evidence_persist":     FactSubtaskEvidence,
	"field_interpretation":         FactSubtaskAIInterpret,
	"clause_interpretation":        FactSubtaskAIInterpret,
	"clause_interpretation_reduce": FactSubtaskAIInterpret,
}

type Repository struct{ db *gorm.DB }

func New() *Repository { return &Repository{db: storage.GetDB()} }

// NewWithDB 用指定连接构造仓储，供测试注入内存库使用。
func NewWithDB(db *gorm.DB) *Repository { return &Repository{db: db} }

type CreateProjectInput struct {
	Name, SourceFileName, SourceBucket, SourceObject, SourceSHA256 string
	UserID                                                         int64
	TeamID, CompanyID                                              int32
	Internal                                                       bool
	ModelConfigJSON                                                string
}

func (r *Repository) CreateProjectAndRun(ctx context.Context, in CreateProjectInput) (*model.BidAnalysisV3Project, *model.BidAnalysisV3ParseRun, error) {
	var project *model.BidAnalysisV3Project
	var run *model.BidAnalysisV3ParseRun
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project = &model.BidAnalysisV3Project{
			Name: in.Name, SourceFileName: in.SourceFileName, SourceBucket: in.SourceBucket,
			SourceObject: in.SourceObject, SourceSha256: in.SourceSHA256, Status: ProjectRunning,
			Stage: Stages[0].Name, UserID: in.UserID, UserTeamID: in.TeamID,
			UserCompanyID: in.CompanyID, IsInternal: in.Internal,
		}
		if err := tx.Create(project).Error; err != nil {
			return err
		}
		run = &model.BidAnalysisV3ParseRun{
			ProjectID: project.ID, RunNo: 1, TriggerType: "create", Status: ProjectRunning,
			Stage: Stages[0].Name, ModelConfigJSON: in.ModelConfigJSON,
		}
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		if err := tx.Model(project).Updates(map[string]any{"current_run_id": run.ID}).Error; err != nil {
			return err
		}
		for _, def := range Stages {
			stage := &model.BidAnalysisV3StageRun{ProjectID: project.ID, RunID: run.ID, Stage: def.Name, Status: StagePending, Weight: def.Weight}
			if err := tx.Create(stage).Error; err != nil {
				return err
			}
		}
		var catalogs []*model.BidAnalysisV3FieldCatalog
		if err := tx.Order("sort_order").Find(&catalogs).Error; err != nil {
			return err
		}
		for _, c := range catalogs {
			field := &model.BidAnalysisV3Field{ProjectID: project.ID, FieldKey: c.FieldKey, DisplayName: c.DisplayName, CategoryKey: c.CategoryKey, Origin: "system", ValueType: c.ValueType, ExtractStatus: "not_found", SortOrder: c.SortOrder}
			if err := tx.Create(field).Error; err != nil {
				return err
			}
		}
		asset := &model.BidAnalysisV3DocumentAsset{ProjectID: project.ID, RunID: run.ID, AssetType: "source", Bucket: in.SourceBucket, ObjectKey: in.SourceObject, FileName: in.SourceFileName, Sha256: in.SourceSHA256}
		return tx.Create(asset).Error
	})
	return project, run, err
}

func (r *Repository) CreateRun(ctx context.Context, projectID, userID int64, trigger, modelConfig string) (*model.BidAnalysisV3ParseRun, error) {
	var run *model.BidAnalysisV3ParseRun
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.BidAnalysisV3Project
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", projectID)
		if userID > 0 {
			q = q.Where("user_id = ?", userID)
		}
		if err := q.First(&p).Error; err != nil {
			return err
		}
		if p.Status == ProjectRunning || p.Status == ProjectPaused {
			return fmt.Errorf("项目已有尚未结束的解析运行")
		}
		// 重新解析一经确认就使旧蓝图失效。已创建投标书依靠来源快照继续编辑，
		// 不再实时依赖招标解析项目或旧蓝图。
		generationIDs := tx.Model(&model.BidAnalysisV3BlueprintGeneration{}).
			Select("id").Where("project_id = ? AND status <> 'invalidated'", projectID)
		if err := tx.Model(&model.BidGenProject{}).
			Where("blueprint_generation_id IN (?)", generationIDs).
			Updates(map[string]any{"tender_project_id": nil, "blueprint_generation_id": nil}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3BlueprintGeneration{}).
			Where("project_id = ? AND status <> 'invalidated'", projectID).
			Updates(map[string]any{"status": "invalidated", "associated_bid_project_id": 0, "completed_at": time.Now()}).Error; err != nil {
			return err
		}
		var maxRun int32
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("project_id = ?", projectID).Select("COALESCE(MAX(run_no), 0)").Scan(&maxRun).Error; err != nil {
			return err
		}
		run = &model.BidAnalysisV3ParseRun{ProjectID: projectID, RunNo: maxRun + 1, TriggerType: trigger, Status: ProjectRunning, Stage: Stages[0].Name, ModelConfigJSON: modelConfig}
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		for _, def := range Stages {
			if err := tx.Create(&model.BidAnalysisV3StageRun{ProjectID: projectID, RunID: run.ID, Stage: def.Name, Status: StagePending, Weight: def.Weight}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&p).Updates(map[string]any{"status": ProjectRunning, "stage": Stages[0].Name, "progress": 0, "warning_count": 0, "last_error": ""}).Error
	})
	return run, err
}

func (r *Repository) Project(ctx context.Context, projectID, userID int64) (*model.BidAnalysisV3Project, error) {
	var p model.BidAnalysisV3Project
	q := r.db.WithContext(ctx).Where("id = ?", projectID)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Run(ctx context.Context, runID int64) (*model.BidAnalysisV3ParseRun, error) {
	var run model.BidAnalysisV3ParseRun
	if err := r.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) CurrentRun(ctx context.Context, projectID int64) (*model.BidAnalysisV3ParseRun, error) {
	var run model.BidAnalysisV3ParseRun
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("run_no DESC").First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) ListProjects(ctx context.Context, userID int64, page, size int, status, keyword string) ([]*model.BidAnalysisV3Project, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.BidAnalysisV3Project{}).Where("user_id = ? AND is_internal = 0", userID)
	if status == "completed" {
		q = q.Where("status IN ?", []string{ProjectSucceeded, ProjectSucceededWithWarnings})
	} else if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.BidAnalysisV3Project
	err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

func (r *Repository) ProjectStatusCounts(ctx context.Context, userID int64) (map[string]int64, error) {
	type countRow struct {
		Status string
		Total  int64
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).Model(&model.BidAnalysisV3Project{}).
		Select("status, COUNT(*) AS total").Where("user_id = ? AND is_internal = 0", userID).
		Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int64{"all": 0, ProjectRunning: 0, ProjectPaused: 0, ProjectSucceeded: 0, ProjectSucceededWithWarnings: 0, ProjectFailed: 0, "completed": 0}
	for _, row := range rows {
		counts[row.Status] = row.Total
		counts["all"] += row.Total
	}
	counts["completed"] = counts[ProjectSucceeded] + counts[ProjectSucceededWithWarnings]
	return counts, nil
}

func (r *Repository) SetStage(ctx context.Context, projectID, runID int64, stage, status string, total, completed, failed int32, lastErr string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.BidAnalysisV3StageRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id = ? AND stage = ?", runID, stage).First(&current).Error; err != nil {
			return err
		}
		updates := map[string]any{"status": status, "total_units": total, "completed_units": completed, "failed_units": failed, "last_error": lastErr}
		if status == StageRunning {
			updates["completed_at"] = nil
			if current.Status != StageRunning {
				updates["started_at"] = now
				updates["attempts"] = gorm.Expr("attempts + 1")
			}
		}
		if status == StageSucceeded || status == StagePartial || status == StageFailed || status == StageSkipped {
			updates["completed_at"] = now
		}
		res := tx.Model(&model.BidAnalysisV3StageRun{}).Where("run_id = ? AND stage = ?", runID, stage).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		progress, err := calculateProgress(tx, runID)
		if err != nil {
			return err
		}
		runUpdates := map[string]any{"stage": stage, "progress": progress}
		projectUpdates := map[string]any{"stage": stage, "progress": progress}
		if status == StageFailed {
			runUpdates["last_error"] = lastErr
			projectUpdates["last_error"] = lastErr
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(runUpdates).Error; err != nil {
			return err
		}
		return tx.Model(&model.BidAnalysisV3Project{}).
			Where("id = ? AND ? = (SELECT latest.id FROM (SELECT id FROM bid_analysis_v3_parse_run WHERE project_id = ? ORDER BY run_no DESC LIMIT 1) latest)", projectID, runID, projectID).
			Updates(projectUpdates).Error
	})
}

func calculateProgress(tx *gorm.DB, runID int64) (int32, error) {
	var stages []*model.BidAnalysisV3StageRun
	if err := tx.Where("run_id = ?", runID).Find(&stages).Error; err != nil {
		return 0, err
	}
	var progress float64
	for _, s := range stages {
		if s.Stage == "chapter_fact_extracting" {
			if factContribution, ok, err := calculateFactStageContribution(tx, runID); err != nil {
				return 0, err
			} else if ok {
				progress += factContribution
				continue
			}
		}
		ratio := float64(0)
		switch s.Status {
		case StageSucceeded, StagePartial, StageSkipped:
			ratio = 1
		case StageRunning:
			if s.TotalUnits > 0 {
				ratio = float64(s.CompletedUnits+s.FailedUnits) / float64(s.TotalUnits)
			}
		}
		progress += float64(s.Weight) * ratio
	}
	if progress > 100 {
		progress = 100
	}
	return int32(progress), nil
}

func calculateFactStageContribution(tx *gorm.DB, runID int64) (float64, bool, error) {
	var rows []struct {
		TaskType string
		Status   string
		Count    int64
	}
	if err := tx.Model(&model.BidAnalysisV3StageTask{}).
		Select("task_type, status, COUNT(*) AS count").
		Where("run_id=? AND stage='chapter_fact_extracting'", runID).
		Group("task_type, status").
		Scan(&rows).Error; err != nil {
		return 0, false, err
	}
	type subtaskCount struct {
		Total     int64
		Completed int64
	}
	counts := map[string]*subtaskCount{
		FactSubtaskDynamicClauses: {},
		FactSubtaskFixedFields:    {},
		FactSubtaskEvidence:       {},
		FactSubtaskAIInterpret:    {},
	}
	hasMappedTasks := false
	for _, row := range rows {
		subtask, ok := factSubtaskTaskTypes[row.TaskType]
		if !ok {
			continue
		}
		hasMappedTasks = true
		item := counts[subtask]
		item.Total += row.Count
		if row.Status == StageSucceeded || row.Status == StageFailed || row.Status == StageSkipped {
			item.Completed += row.Count
		}
	}
	if !hasMappedTasks {
		return 0, false, nil
	}
	progress := float64(0)
	for subtask, weight := range factSubtaskWeights {
		item := counts[subtask]
		if item.Total == 0 {
			continue
		}
		progress += float64(weight) * float64(item.Completed) / float64(item.Total)
	}
	if progress > float64(Stages[4].Weight) {
		progress = float64(Stages[4].Weight)
	}
	return progress, true, nil
}

func (r *Repository) CompleteRun(ctx context.Context, projectID, runID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.BidAnalysisV3Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", projectID).First(&project).Error; err != nil {
			return err
		}
		var latestRunID int64
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("project_id=?", projectID).Order("run_no DESC").Limit(1).Select("id").Scan(&latestRunID).Error; err != nil {
			return err
		}
		if latestRunID != runID {
			return ErrRunSuperseded
		}
		var activeControls int64
		if err := tx.Model(&model.BidAnalysisV3RunControl{}).
			Where("run_id=? AND status IN ?", runID, []string{ControlRequested, ControlApplying}).
			Count(&activeControls).Error; err != nil {
			return err
		}
		if activeControls > 0 {
			return ErrRunControlPending
		}
		warningCount, err := countUnresolvedAlerts(tx, runID)
		if err != nil {
			return err
		}
		status := ProjectSucceeded
		if warningCount > 0 {
			status = ProjectSucceededWithWarnings
		}
		now := time.Now()
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(map[string]any{"status": status, "progress": 100, "warning_count": warningCount, "completed_at": now, "activated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&project).Updates(map[string]any{"current_run_id": runID, "status": status, "progress": 100, "warning_count": warningCount, "last_error": ""}).Error
	})
}

func (r *Repository) FailRun(ctx context.Context, projectID, runID int64, stage string, cause error) error {
	msg := cause.Error()
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, _, err := lockCurrentRun(tx, projectID, runID); err != nil {
			return err
		}
		var controls []*model.BidAnalysisV3RunControl
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id=? AND status IN ?", runID, []string{ControlRequested, ControlApplying}).Find(&controls).Error; err != nil {
			return err
		}
		for _, control := range controls {
			if control.Mode == ControlModeDiscardCurrent {
				return ErrRunControlPending
			}
			if err := tx.Model(control).Updates(map[string]any{"status": ControlCancelled, "last_error": msg}).Error; err != nil {
				return err
			}
			control.Status = ControlCancelled
			control.LastError = msg
			if err := createControlLog(tx, control, "cancel_pause", msg); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.BidAnalysisV3StageRun{}).Where("run_id = ? AND stage = ?", runID, stage).Updates(map[string]any{"status": StageFailed, "last_error": msg, "completed_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(map[string]any{"status": ProjectFailed, "stage": stage, "last_error": msg, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.BidAnalysisV3Project{}).
			Where("id = ? AND ? = (SELECT latest.id FROM (SELECT id FROM bid_analysis_v3_parse_run WHERE project_id = ? ORDER BY run_no DESC LIMIT 1) latest)", projectID, runID, projectID).
			Updates(map[string]any{"status": ProjectFailed, "stage": stage, "last_error": msg}).Error
	})
}

func (r *Repository) AddWarning(ctx context.Context, warning *model.BidAnalysisV3Warning) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return AddWarningTx(tx, warning)
	})
}

// AddWarningTx 在给定事务内幂等写入告警：同一 run+group_key 合并详情并刷新样例消息，
// 新建非提示级告警时同步累计计数。流水线在既有事务内直接复用，保证同根因只落一行。
func AddWarningTx(tx *gorm.DB, warning *model.BidAnalysisV3Warning) error {
	if strings.TrimSpace(warning.GroupKey) == "" {
		warning.GroupKey = "code:" + warning.Code
	}
	var existing model.BidAnalysisV3Warning
	err := tx.Where("run_id = ? AND group_key = ?", warning.RunID, warning.GroupKey).First(&existing).Error
	if err == nil {
		// 同一根因告警幂等合并：不新增行，只累计 occurrences 并刷新样例消息/详情。
		oldCounted := existing.Severity != "info"
		newSeverity := strings.TrimSpace(warning.Severity)
		if newSeverity == "" {
			newSeverity = existing.Severity
		}
		if newSeverity == "" {
			newSeverity = "warning"
		}
		newCounted := newSeverity != "info"
		merged := mergeWarningDetail(existing.DetailJSON, warning.DetailJSON, warning.Message)
		updates := map[string]any{
			"message":     warning.Message,
			"severity":    warning.Severity,
			"stage":       warning.Stage,
			"detail_json": merged,
			"resolved":    false,
		}
		if existing.Severity != "" && warning.Severity == "" {
			delete(updates, "severity")
		}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		delta := 0
		if newCounted && !oldCounted {
			delta = 1
		}
		if oldCounted && !newCounted {
			delta = -1
		}
		if delta != 0 {
			return bumpWarningCounts(tx, warning.ProjectID, warning.RunID, delta)
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if warning.Severity == "" {
		warning.Severity = "warning"
	}
	if err := tx.Create(warning).Error; err != nil {
		return err
	}
	if warning.Severity != "info" {
		return bumpWarningCounts(tx, warning.ProjectID, warning.RunID, 1)
	}
	return nil
}

// bumpWarningCounts 同步加减运行与当前项目的未解决告警计数（delta 可为负数，下限 0）。
func bumpWarningCounts(tx *gorm.DB, projectID, runID int64, delta int) error {
	// CASE WHEN 在 MySQL 与 SQLite 均可用（MySQL 不支持 MAX(x,0) 标量形式）。
	expr := gorm.Expr("CASE WHEN warning_count + ? < 0 THEN 0 ELSE warning_count + ? END", delta, delta)
	if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).UpdateColumn("warning_count", expr).Error; err != nil {
		return err
	}
	return tx.Model(&model.BidAnalysisV3Project{}).
		Where("id = ? AND ? = (SELECT latest.id FROM (SELECT id FROM bid_analysis_v3_parse_run WHERE project_id = ? ORDER BY run_no DESC LIMIT 1) latest)", projectID, runID, projectID).
		UpdateColumn("warning_count", expr).Error
}

// countUnresolvedAlerts 统计运行内未解决的非提示级告警数量。
// 提示级（info）告警不计入“未解决告警”，避免把系统提示当成用户待办。
func countUnresolvedAlerts(tx *gorm.DB, runID int64) (int64, error) {
	var count int64
	err := tx.Model(&model.BidAnalysisV3Warning{}).
		Where("run_id = ? AND resolved = 0 AND severity <> 'info'", runID).
		Count(&count).Error
	return count, err
}

// mergeWarningDetail 合并告警结构化详情：occurrences 相加、追加受影响章节/任务并去重、
// 刷新样例消息；旧数据 detail_json 为空时按全新详情处理。
func mergeWarningDetail(existingDetailJSON *string, newDetailJSON *string, latestMessage string) *string {
	chapters := []map[string]any{}
	tasks := []int64{}
	reasons := map[string]int{}
	seenChapter := map[int64]bool{}
	seenTask := map[int64]bool{}
	totalOcc := 0
	load := func(raw *string, defaultOcc int) {
		if raw == nil || strings.TrimSpace(*raw) == "" {
			totalOcc += defaultOcc
			return
		}
		var parsed map[string]any
		if json.Unmarshal([]byte(*raw), &parsed) != nil {
			totalOcc += defaultOcc
			return
		}
		if n, ok := parsed["occurrences"].(float64); ok && int(n) > 0 {
			totalOcc += int(n)
		} else {
			totalOcc += defaultOcc
		}
		if items, ok := parsed["chapters"].([]any); ok {
			for _, item := range items {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, ok := obj["id"].(float64)
				if !ok || seenChapter[int64(id)] {
					continue
				}
				seenChapter[int64(id)] = true
				chapters = append(chapters, map[string]any{"id": int64(id), "title": strAny(obj["title"]), "page_start": numAny(obj["page_start"]), "page_end": numAny(obj["page_end"])})
			}
		}
		if items, ok := parsed["tasks"].([]any); ok {
			for _, item := range items {
				id, ok := item.(float64)
				if !ok || seenTask[int64(id)] {
					continue
				}
				seenTask[int64(id)] = true
				tasks = append(tasks, int64(id))
			}
		}
		if items, ok := parsed["reasons"].(map[string]any); ok {
			for reason, raw := range items {
				switch n := raw.(type) {
				case float64:
					reasons[reason] += int(n)
				case int64:
					reasons[reason] += int(n)
				}
			}
		}
	}
	load(existingDetailJSON, 0)
	load(newDetailJSON, 1)
	merged := map[string]any{"occurrences": totalOcc, "sample_message": latestMessage}
	if len(chapters) > 0 {
		merged["chapters"] = chapters
	}
	if len(tasks) > 0 {
		merged["tasks"] = tasks
	}
	if len(reasons) > 0 {
		merged["reasons"] = reasons
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return newDetailJSON
	}
	mergedStr := string(raw)
	return &mergedStr
}

func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func numAny(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

func (r *Repository) CreateAsset(ctx context.Context, asset *model.BidAnalysisV3DocumentAsset) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.BidAnalysisV3DocumentAsset
		err := tx.Where("bucket=? AND object_key=?", asset.Bucket, asset.ObjectKey).First(&existing).Error
		if err == nil {
			asset.ID = existing.ID
			return tx.Model(&existing).Updates(map[string]any{
				"project_id": asset.ProjectID, "run_id": asset.RunID, "asset_type": asset.AssetType,
				"file_name": asset.FileName, "mime_type": asset.MimeType, "size_bytes": asset.SizeBytes, "sha256": asset.Sha256,
			}).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(asset).Error
	})
}

func (r *Repository) UpdateNormalizedPDF(ctx context.Context, projectID int64, bucket, object string, pages int) error {
	return r.db.WithContext(ctx).Model(&model.BidAnalysisV3Project{}).Where("id = ?", projectID).Updates(map[string]any{"normalized_pdf_bucket": bucket, "normalized_pdf_object": object, "page_count": pages}).Error
}

func (r *Repository) SetRunDocumentTotals(ctx context.Context, runID int64, pages, chunks int) error {
	return r.db.WithContext(ctx).Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(map[string]any{"total_pages": pages, "total_chunks": chunks}).Error
}

func (r *Repository) CreateChunks(ctx context.Context, chunks []*model.BidAnalysisV3DocumentChunk) error {
	if len(chunks) == 0 {
		return errors.New("没有可创建的文档页块")
	}
	return r.db.WithContext(ctx).Create(&chunks).Error
}

// SaveChunkDefinitions 原子保存页块 PDF 资产及页块运行态，避免只写入其中一侧。
func (r *Repository) SaveChunkDefinitions(ctx context.Context, assets []*model.BidAnalysisV3DocumentAsset, chunks []*model.BidAnalysisV3DocumentChunk) error {
	if len(assets) == 0 || len(assets) != len(chunks) {
		return errors.New("页块资产与页块数量不一致")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range assets {
			var existing model.BidAnalysisV3DocumentAsset
			err := tx.Where("bucket=? AND object_key=?", assets[i].Bucket, assets[i].ObjectKey).First(&existing).Error
			if err == nil {
				assets[i].ID = existing.ID
				if err := tx.Model(&existing).Updates(map[string]any{
					"project_id": assets[i].ProjectID, "run_id": assets[i].RunID, "asset_type": assets[i].AssetType,
					"file_name": assets[i].FileName, "mime_type": assets[i].MimeType, "size_bytes": assets[i].SizeBytes, "sha256": assets[i].Sha256,
				}).Error; err != nil {
					return err
				}
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(assets[i]).Error; err != nil {
					return err
				}
			} else {
				return err
			}
			chunks[i].PdfAssetID = assets[i].ID
			if err := tx.Create(chunks[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type ChunkIndex struct {
	Chunk  *model.BidAnalysisV3DocumentChunk
	Pages  []*model.BidAnalysisV3DocumentPage
	Blocks []*model.BidAnalysisV3DocumentBlock
	Tables []*model.BidAnalysisV3SourceTable
}

func (r *Repository) SaveChunkIndex(ctx context.Context, index ChunkIndex) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(index.Pages) > 0 {
			if err := tx.Create(&index.Pages).Error; err != nil {
				return err
			}
		}
		if len(index.Blocks) > 0 {
			if err := tx.CreateInBatches(index.Blocks, 250).Error; err != nil {
				return err
			}
		}
		if len(index.Tables) > 0 {
			if err := tx.CreateInBatches(index.Tables, 100).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.BidAnalysisV3DocumentChunk{}).Where("id = ? AND status <> 'succeeded'", index.Chunk.ID).Updates(map[string]any{
			"status": "succeeded", "attempts": index.Chunk.Attempts, "docling_asset_id": index.Chunk.DoclingAssetID,
			"text_chars": index.Chunk.TextChars, "block_count": len(index.Blocks), "table_count": len(index.Tables),
			"checksum": index.Chunk.Checksum, "processing_ms": index.Chunk.ProcessingMs, "last_error": "",
		}).Error
	})
}

func (r *Repository) UpdateChunkFailure(ctx context.Context, chunkID int64, attempts int32, err error) error {
	return r.db.WithContext(ctx).Model(&model.BidAnalysisV3DocumentChunk{}).Where("id = ?", chunkID).Updates(map[string]any{"status": "failed", "attempts": attempts, "last_error": err.Error()}).Error
}

func (r *Repository) Chunks(ctx context.Context, runID int64) ([]*model.BidAnalysisV3DocumentChunk, error) {
	var chunks []*model.BidAnalysisV3DocumentChunk
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("chunk_no").Find(&chunks).Error
	return chunks, err
}

func (r *Repository) Blocks(ctx context.Context, runID int64) ([]*model.BidAnalysisV3DocumentBlock, error) {
	var blocks []*model.BidAnalysisV3DocumentBlock
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("page_no, sort_order").Find(&blocks).Error
	return blocks, err
}

func (r *Repository) Tables(ctx context.Context, runID int64) ([]*model.BidAnalysisV3SourceTable, error) {
	var tables []*model.BidAnalysisV3SourceTable
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("page_start, sort_order").Find(&tables).Error
	return tables, err
}

func (r *Repository) SaveChapters(ctx context.Context, runID int64, chapters []*model.BidAnalysisV3Chapter, linksByIndex map[int][]int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		oldChapterIDs := tx.Model(&model.BidAnalysisV3Chapter{}).Select("id").Where("run_id = ?", runID)
		if err := tx.Where("chapter_id IN (?)", oldChapterIDs).Delete(&model.BidAnalysisV3ChapterBlock{}).Error; err != nil {
			return fmt.Errorf("删除旧章节文本块关联: %w", err)
		}
		if err := tx.Where("run_id = ?", runID).Delete(&model.BidAnalysisV3Chapter{}).Error; err != nil {
			return err
		}
		for i, chapter := range chapters {
			if err := tx.Create(chapter).Error; err != nil {
				return err
			}
			ids := linksByIndex[i]
			links := make([]*model.BidAnalysisV3ChapterBlock, 0, len(ids))
			for order, blockID := range ids {
				links = append(links, &model.BidAnalysisV3ChapterBlock{ChapterID: chapter.ID, BlockID: blockID, SortOrder: int32(order)})
			}
			if len(links) > 0 {
				if err := tx.CreateInBatches(links, 250).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Update("total_chapters", len(chapters)).Error
	})
}

func (r *Repository) Chapters(ctx context.Context, runID int64) ([]*model.BidAnalysisV3Chapter, error) {
	var chapters []*model.BidAnalysisV3Chapter
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("sort_order").Find(&chapters).Error
	return chapters, err
}

func (r *Repository) ChapterBlocks(ctx context.Context, chapterID int64) ([]*model.BidAnalysisV3DocumentBlock, error) {
	var blocks []*model.BidAnalysisV3DocumentBlock
	err := r.db.WithContext(ctx).Table(model.TableNameBidAnalysisV3DocumentBlock+" b").Select("b.*").Joins("JOIN "+model.TableNameBidAnalysisV3ChapterBlock+" cb ON cb.block_id = b.id").Where("cb.chapter_id = ?", chapterID).Order("cb.sort_order").Scan(&blocks).Error
	return blocks, err
}

func (r *Repository) StageRuns(ctx context.Context, runID int64) ([]*model.BidAnalysisV3StageRun, error) {
	var stages []*model.BidAnalysisV3StageRun
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("id").Find(&stages).Error
	return stages, err
}

func (r *Repository) StageTasks(ctx context.Context, runID int64) ([]*model.BidAnalysisV3StageTask, error) {
	var tasks []*model.BidAnalysisV3StageTask
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("id").Find(&tasks).Error
	return tasks, err
}

type FactSubtaskStatus struct {
	SubTask   string
	Completed int64
	Total     int64
}

func (r *Repository) FactSubtaskStatuses(ctx context.Context, runID int64) ([]FactSubtaskStatus, error) {
	var rows []struct {
		TaskType string
		Status   string
		Count    int64
	}
	if err := r.db.WithContext(ctx).Model(&model.BidAnalysisV3StageTask{}).
		Select("task_type, status, COUNT(*) AS count").
		Where("run_id=? AND stage='chapter_fact_extracting'", runID).
		Group("task_type, status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	type subtaskCount struct {
		Total     int64
		Completed int64
	}
	counts := map[string]*subtaskCount{
		FactSubtaskDynamicClauses: {},
		FactSubtaskFixedFields:    {},
		FactSubtaskEvidence:       {},
		FactSubtaskAIInterpret:    {},
	}
	for _, row := range rows {
		subtask, ok := factSubtaskTaskTypes[row.TaskType]
		if !ok {
			continue
		}
		item := counts[subtask]
		item.Total += row.Count
		if row.Status == StageSucceeded || row.Status == StageFailed || row.Status == StageSkipped {
			item.Completed += row.Count
		}
	}
	order := []string{FactSubtaskDynamicClauses, FactSubtaskFixedFields, FactSubtaskEvidence, FactSubtaskAIInterpret}
	out := make([]FactSubtaskStatus, 0, len(order))
	for _, key := range order {
		item := counts[key]
		out = append(out, FactSubtaskStatus{SubTask: key, Completed: item.Completed, Total: item.Total})
	}
	return out, nil
}

func (r *Repository) StageAttempt(ctx context.Context, runID int64, stage string) (int32, error) {
	var attempt int32
	err := r.db.WithContext(ctx).Model(&model.BidAnalysisV3StageRun{}).
		Where("run_id=? AND stage=?", runID, stage).Select("attempts").Scan(&attempt).Error
	return attempt, err
}

func (r *Repository) Warnings(ctx context.Context, runID int64) ([]*model.BidAnalysisV3Warning, error) {
	var warnings []*model.BidAnalysisV3Warning
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("id DESC").Find(&warnings).Error
	return warnings, err
}

// ResolveWarnings 将匹配的未解决告警标记为已解决；支持按告警 id、分组键或级别批量处理。
// 仅非提示级（非 info）告警参与计数扣减；info 提示不产生告警数字，因此不扣减。
// 返回本次解决的告警数以及项目当前未解决告警数。
func (r *Repository) ResolveWarnings(ctx context.Context, projectID, runID int64, ids []int64, groupKeys []string, severity string) (int64, int64, error) {
	var resolvedCount int64
	var finalCount int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		build := func() *gorm.DB {
			q := tx.Model(&model.BidAnalysisV3Warning{}).Where("run_id = ? AND resolved = 0", runID)
			if len(ids) > 0 {
				q = q.Where("id IN ?", ids)
			}
			if len(groupKeys) > 0 {
				q = q.Where("group_key IN ?", groupKeys)
			}
			if strings.TrimSpace(severity) != "" {
				q = q.Where("severity = ?", strings.TrimSpace(severity))
			}
			return q
		}
		if err := build().Count(&resolvedCount).Error; err != nil {
			return err
		}
		if resolvedCount > 0 {
			var nonInfo int64
			if err := build().Where("severity <> 'info'").Count(&nonInfo).Error; err != nil {
				return err
			}
			if err := build().Update("resolved", true).Error; err != nil {
				return err
			}
			if nonInfo > 0 {
				expr := gorm.Expr("CASE WHEN warning_count - ? < 0 THEN 0 ELSE warning_count - ? END", nonInfo, nonInfo)
				if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).UpdateColumn("warning_count", expr).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.BidAnalysisV3Project{}).
					Where("id = ? AND current_run_id = ?", projectID, runID).
					UpdateColumn("warning_count", expr).Error; err != nil {
					return err
				}
			}
		}
		// 状态调和：无论本次是否真的解决了告警，只要项目是“完成但有告警”且已无未解决告警，
		// 就降级为“完成”，保证列表页卡片状态与告警数字一致。
		remaining, err := countUnresolvedAlerts(tx, runID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			var project model.BidAnalysisV3Project
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", projectID).First(&project).Error; err != nil {
				return err
			}
			if project.Status == ProjectSucceededWithWarnings && project.CurrentRunID == runID {
				if err := tx.Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(map[string]any{
					"status": ProjectSucceeded, "warning_count": 0,
				}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.BidAnalysisV3Project{}).Where("id = ?", projectID).Updates(map[string]any{
					"status": ProjectSucceeded, "warning_count": 0,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	if err := r.db.WithContext(ctx).Model(&model.BidAnalysisV3Project{}).
		Where("id = ?", projectID).Select("warning_count").Scan(&finalCount).Error; err != nil {
		return 0, 0, err
	}
	return resolvedCount, finalCount, nil
}

func (r *Repository) CreateStageTask(ctx context.Context, task *model.BidAnalysisV3StageTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) UpdateStageTask(ctx context.Context, id int64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.BidAnalysisV3StageTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repository) MarkParsedProgress(ctx context.Context, runID int64, pages int32) error {
	return r.db.WithContext(ctx).Model(&model.BidAnalysisV3ParseRun{}).Where("id = ?", runID).Updates(map[string]any{"parsed_pages": gorm.Expr("parsed_pages + ?", pages), "parsed_chunks": gorm.Expr("parsed_chunks + 1")}).Error
}

func (r *Repository) DB() *gorm.DB { return r.db }

func (r *Repository) CountProjectsByTime(ctx context.Context, userID int64, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.BidAnalysisV3Project{}).
		Where("user_id=? AND is_internal=0 AND created_at>=? AND created_at<=?", userID, start, end).
		Count(&count).Error
	return count, err
}
