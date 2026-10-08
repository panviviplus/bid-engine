package bidanalysisv3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

func (r *Repository) DeleteProject(ctx context.Context, projectID, userID, deletedBy int64, reason string) (int64, error) {
	var jobID int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var innerErr error
		jobID, innerErr = DeleteProjectTx(tx, projectID, userID, deletedBy, reason)
		return innerErr
	})
	return jobID, err
}

func DeleteProjectTx(tx *gorm.DB, projectID, userID, deletedBy int64, reason string) (int64, error) {
	var p model.BidAnalysisV3Project
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=?", projectID, userID).First(&p).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("锁定待删除项目: %w", err)
		}
		var tomb model.BidAnalysisV3ProjectDeleted
		if err := tx.Unscoped().Where("original_project_id=? AND user_id=?", projectID, userID).First(&tomb).Error; err != nil {
			return 0, err
		}
		var job model.BidAnalysisV3DeletionJob
		if err := tx.Where("deleted_project_id=? AND original_project_id=?", tomb.ID, projectID).Order("id DESC").First(&job).Error; err != nil {
			return 0, fmt.Errorf("查找已有删除任务: %w", err)
		}
		return job.ID, nil
	}
	var fieldCount, evidenceCount int64
	if err := tx.Model(&model.BidAnalysisV3Field{}).Where("project_id=?", projectID).Count(&fieldCount).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&model.BidAnalysisV3FieldValueEvidence{}).Where("project_id=?", projectID).Count(&evidenceCount).Error; err != nil {
		return 0, err
	}
	tomb := &model.BidAnalysisV3ProjectDeleted{OriginalProjectID: p.ID, Name: p.Name, SourceFileName: p.SourceFileName, SourceSha256: p.SourceSha256, FinalStatus: p.Status, PageCount: p.PageCount, FieldCount: int32(fieldCount), EvidenceCount: int32(evidenceCount), WarningCount: p.WarningCount, UserID: p.UserID, UserTeamID: p.UserTeamID, UserCompanyID: p.UserCompanyID, CreatedAt: p.CreatedAt, DeletedBy: deletedBy, DeleteReason: reason}
	if err := tx.Create(tomb).Error; err != nil {
		return 0, err
	}
	var assets []*model.BidAnalysisV3DocumentAsset
	if err := tx.Where("project_id=?", projectID).Find(&assets).Error; err != nil {
		return 0, err
	}
	job := &model.BidAnalysisV3DeletionJob{DeletedProjectID: tomb.ID, OriginalProjectID: p.ID, Status: "pending", TotalObjects: int32(len(assets))}
	if err := tx.Create(job).Error; err != nil {
		return 0, err
	}
	for _, asset := range assets {
		object := &model.BidAnalysisV3DeletionObject{JobID: job.ID, Bucket: asset.Bucket, ObjectKey: asset.ObjectKey, Status: "pending"}
		if err := tx.Create(object).Error; err != nil {
			return 0, err
		}
	}
	if err := releaseBidProjectAssociations(tx, projectID); err != nil {
		return 0, err
	}
	if err := deleteProjectDataTx(tx, projectID); err != nil {
		return 0, err
	}
	if err := tx.Delete(&p).Error; err != nil {
		return 0, err
	}
	return job.ID, nil
}

func deleteProjectDataTx(tx *gorm.DB, projectID int64) error {
	chapterIDs := tx.Model(&model.BidAnalysisV3Chapter{}).Select("id").Where("project_id=?", projectID)
	if err := tx.Where("chapter_id IN (?)", chapterIDs).Delete(&model.BidAnalysisV3ChapterBlock{}).Error; err != nil {
		return fmt.Errorf("删除章节文本块关联: %w", err)
	}
	derivedTableIDs := tx.Model(&model.BidAnalysisV3DerivedTable{}).Select("id").Where("project_id=?", projectID)
	if err := tx.Where("derived_table_id IN (?)", derivedTableIDs).Delete(&model.BidAnalysisV3DerivedTableEvidence{}).Error; err != nil {
		return fmt.Errorf("删除归并表证据: %w", err)
	}

	deletions := []struct {
		name  string
		model any
	}{
		{"条款证据", &model.BidAnalysisV3ClauseEvidence{}},
		{"字段值证据", &model.BidAnalysisV3FieldValueEvidence{}},
		{"标书蓝图节点", &model.BidAnalysisV3Blueprint{}},
		{"标书蓝图生成记录", &model.BidAnalysisV3BlueprintGeneration{}},
		{"解析摘要", &model.BidAnalysisV3Summary{}},
		{"摘要风险解决状态", &model.BidAnalysisV3SummaryRiskResolution{}},
		{"关注项", &model.BidAnalysisV3Follow{}},
		{"条款", &model.BidAnalysisV3Clause{}},
		{"事实候选", &model.BidAnalysisV3FactCandidate{}},
		{"归并表", &model.BidAnalysisV3DerivedTable{}},
		{"字段值", &model.BidAnalysisV3FieldValue{}},
		{"项目字段", &model.BidAnalysisV3Field{}},
		{"原文表格", &model.BidAnalysisV3SourceTable{}},
		{"文档文本块", &model.BidAnalysisV3DocumentBlock{}},
		{"文档页", &model.BidAnalysisV3DocumentPage{}},
		{"文档页块", &model.BidAnalysisV3DocumentChunk{}},
		{"文档资产", &model.BidAnalysisV3DocumentAsset{}},
		{"运行控制", &model.BidAnalysisV3RunControl{}},
		{"阶段任务", &model.BidAnalysisV3StageTask{}},
		{"阶段运行", &model.BidAnalysisV3StageRun{}},
		{"告警", &model.BidAnalysisV3Warning{}},
		{"操作日志", &model.BidAnalysisV3OperationLog{}},
		{"模型调用记录", &model.BidAnalysisV3LlmCall{}},
		{"章节", &model.BidAnalysisV3Chapter{}},
		{"解析运行", &model.BidAnalysisV3ParseRun{}},
	}
	for _, deletion := range deletions {
		if err := tx.Where("project_id=?", projectID).Delete(deletion.model).Error; err != nil {
			return fmt.Errorf("删除%s: %w", deletion.name, err)
		}
	}
	return nil
}

// releaseBidProjectAssociations supports both the original V3 schema and the
// later on-demand-blueprint schema. Some deployed databases intentionally do
// not have the generation table/column yet; deleting an analysis project must
// not depend on that optional migration having run.
func releaseBidProjectAssociations(tx *gorm.DB, projectID int64) error {
	migrator := tx.Migrator()
	if !migrator.HasTable(model.TableNameBidGenProject) {
		return nil
	}

	hasTenderProjectID := migrator.HasColumn(&model.BidGenProject{}, "tender_project_id")
	if hasTenderProjectID {
		if err := tx.Unscoped().Table(model.TableNameBidGenProject).
			Where("tender_project_id=?", projectID).
			Update("tender_project_id", nil).Error; err != nil {
			return fmt.Errorf("解除投标书与招标解析项目关联: %w", err)
		}
	}

	if !migrator.HasTable(model.TableNameBidAnalysisV3BlueprintGeneration) ||
		!migrator.HasColumn(&model.BidGenProject{}, "blueprint_generation_id") {
		return nil
	}

	generationIDs := tx.Table(model.TableNameBidAnalysisV3BlueprintGeneration).
		Select("id").Where("project_id=?", projectID)
	updates := map[string]any{"blueprint_generation_id": nil}
	if hasTenderProjectID {
		updates["tender_project_id"] = nil
	}
	if err := tx.Unscoped().Table(model.TableNameBidGenProject).
		Where("blueprint_generation_id IN (?)", generationIDs).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("解除投标书与标书蓝图关联: %w", err)
	}
	return nil
}

func (r *Repository) PendingDeletionJobs(ctx context.Context, limit int) ([]*model.BidAnalysisV3DeletionJob, error) {
	var jobs []*model.BidAnalysisV3DeletionJob
	err := r.db.WithContext(ctx).Where("attempts < 10 AND (status IN ('pending','failed') OR (status='processing' AND updated_at<?))", time.Now().Add(-5*time.Minute)).Order("created_at").Limit(limit).Find(&jobs).Error
	return jobs, err
}

func (r *Repository) DeletionJob(ctx context.Context, jobID int64) (*model.BidAnalysisV3DeletionJob, error) {
	var job model.BidAnalysisV3DeletionJob
	if err := r.db.WithContext(ctx).Where("id=?", jobID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *Repository) ClaimDeletionJob(ctx context.Context, jobID int64) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.BidAnalysisV3DeletionJob{}).
		Where("id=? AND attempts < 10 AND (status IN ('pending','failed') OR (status='processing' AND updated_at<?))", jobID, time.Now().Add(-5*time.Minute)).
		Updates(map[string]any{"status": "processing", "attempts": gorm.Expr("attempts+1"), "last_error": ""})
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) DeletionObjects(ctx context.Context, jobID int64) ([]*model.BidAnalysisV3DeletionObject, error) {
	var objects []*model.BidAnalysisV3DeletionObject
	// Unscoped is required because deleted_at is business completion time but GORM
	// generated it as gorm.DeletedAt.
	err := r.db.WithContext(ctx).Unscoped().Where("job_id=? AND status<>'succeeded'", jobID).Order("id").Find(&objects).Error
	return objects, err
}

func (r *Repository) MarkDeletionObject(ctx context.Context, id int64, succeeded bool, cause string) error {
	fields := map[string]any{"attempts": gorm.Expr("attempts+1"), "last_error": cause, "status": "failed"}
	if succeeded {
		fields["status"] = "succeeded"
		fields["deleted_at"] = time.Now()
		fields["last_error"] = ""
	}
	return r.db.WithContext(ctx).Unscoped().Model(&model.BidAnalysisV3DeletionObject{}).Where("id=?", id).Updates(fields).Error
}

func (r *Repository) FinishDeletionJob(ctx context.Context, jobID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var total, success int64
		if err := tx.Unscoped().Model(&model.BidAnalysisV3DeletionObject{}).Where("job_id=?", jobID).Count(&total).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&model.BidAnalysisV3DeletionObject{}).Where("job_id=? AND status='succeeded'", jobID).Count(&success).Error; err != nil {
			return err
		}
		remaining := total - success
		status := "succeeded"
		fields := map[string]any{"status": status, "deleted_objects": success, "failed_objects": remaining, "completed_at": time.Now(), "last_error": ""}
		if remaining > 0 {
			fields["status"] = "failed"
			fields["completed_at"] = nil
			fields["last_error"] = "仍有对象未确认删除，将自动重试"
		}
		return tx.Model(&model.BidAnalysisV3DeletionJob{}).Where("id=?", jobID).Updates(fields).Error
	})
}
