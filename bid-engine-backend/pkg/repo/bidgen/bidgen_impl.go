package bidgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	"bid-engine/pkg/service/biddoc"
)

// ===== bid_gen_project =====

func (s *svcImpl) AddProject(ctx context.Context, p *model.BidGenProject) error {
	return query.Use(s.db).BidGenProject.WithContext(ctx).Create(p)
}

func (s *svcImpl) GetProjectByID(ctx context.Context, id int64) (*model.BidGenProject, error) {
	p := query.Use(s.db).BidGenProject
	return p.WithContext(ctx).Where(p.ID.Eq(id)).First()
}

func (s *svcImpl) GetProjectForUser(ctx context.Context, userID, id int64) (*model.BidGenProject, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	p := query.Use(s.db).BidGenProject
	return p.WithContext(ctx).Where(p.ID.Eq(id), p.UserID.Eq(userID)).First()
}

func (s *svcImpl) UpdateProjectFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	p := query.Use(s.db).BidGenProject
	_, err := p.WithContext(ctx).Where(p.ID.Eq(id)).Updates(fields)
	return err
}

func (s *svcImpl) UpdateProjectForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error {
	if userID <= 0 || id <= 0 {
		return gorm.ErrRecordNotFound
	}
	p := query.Use(s.db).BidGenProject
	result, err := p.WithContext(ctx).Where(p.ID.Eq(id), p.UserID.Eq(userID)).Updates(fields)
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *svcImpl) GetProjects(ctx context.Context, pageNum, pageSize int, status, name string) ([]*model.BidGenProject, int64, error) {
	p := query.Use(s.db).BidGenProject
	q := p.WithContext(ctx)
	if status != "" {
		q = q.Where(p.Status.Eq(status))
	}
	if name != "" {
		q = q.Where(p.Name.Like("%" + name + "%"))
	}
	total, err := q.Count()
	if err != nil {
		return nil, 0, err
	}
	records, err := q.Order(p.CreatedAt.Desc()).Offset((pageNum - 1) * pageSize).Limit(pageSize).Find()
	return records, total, err
}

func (s *svcImpl) GetProjectsForUser(ctx context.Context, userID int64, pageNum, pageSize int, status, name string) ([]*model.BidGenProject, int64, error) {
	if userID <= 0 {
		return nil, 0, gorm.ErrRecordNotFound
	}
	p := query.Use(s.db).BidGenProject
	q := p.WithContext(ctx).Where(p.UserID.Eq(userID))
	if status != "" {
		q = q.Where(p.Status.Eq(status))
	}
	if name != "" {
		q = q.Where(p.Name.Like("%" + name + "%"))
	}
	total, err := q.Count()
	if err != nil {
		return nil, 0, err
	}
	records, err := q.Order(p.CreatedAt.Desc()).Offset((pageNum - 1) * pageSize).Limit(pageSize).Find()
	return records, total, err
}

// CountProjectsByTime 按用户与创建时间范围统计 bid_gen_project 数量（dashboard“本月生成”）
func (s *svcImpl) CountProjectsByTime(ctx context.Context, userID int64, startTime, endTime int64) (int64, error) {
	if userID <= 0 {
		return 0, gorm.ErrRecordNotFound
	}
	p := query.Use(s.db).BidGenProject
	q := p.WithContext(ctx).Where(p.UserID.Eq(userID))
	if startTime > 0 {
		q = q.Where(p.CreatedAt.Gte(time.Unix(startTime, 0)))
	}
	if endTime > 0 {
		q = q.Where(p.CreatedAt.Lte(time.Unix(endTime, 0)))
	}
	return q.Count()
}

func (s *svcImpl) DeleteProjectByID(ctx context.Context, id int64) error {
	p := query.Use(s.db).BidGenProject
	_, err := p.WithContext(ctx).Where(p.ID.Eq(id)).Delete()
	return err
}

func (s *svcImpl) DeleteProjectForUser(ctx context.Context, userID, id int64) error {
	if userID <= 0 || id <= 0 {
		return gorm.ErrRecordNotFound
	}
	p := query.Use(s.db).BidGenProject
	result, err := p.WithContext(ctx).Where(p.ID.Eq(id), p.UserID.Eq(userID)).Delete()
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *svcImpl) DeleteProjectCascadeForUser(ctx context.Context, userID, id int64) (*model.BidGenProject, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var project model.BidGenProject
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=?", id, userID).First(&project).Error; err != nil {
			return err
		}
		if err := tx.Where("bid_project_id=?", id).Delete(&model.BidGenSourceSnapshot{}).Error; err != nil {
			return fmt.Errorf("删除标书来源快照: %w", err)
		}
		deletions := []struct {
			name  string
			model any
		}{
			{"章节正文", &model.BidGenChapterContent{}},
			{"完整文档", &model.BidGenDocContent{}},
			{"生成任务", &model.BidGenTask{}},
			{"素材引用", &model.BidGenMaterialRef{}},
			{"导出记录", &model.BidGenExportRecord{}},
			{"大纲", &model.BidGenOutline{}},
		}
		for _, deletion := range deletions {
			if err := tx.Where("project_id=?", id).Delete(deletion.model).Error; err != nil {
				return fmt.Errorf("删除标书%s: %w", deletion.name, err)
			}
		}
		if project.BlueprintGenerationID != nil && *project.BlueprintGenerationID > 0 {
			if err := tx.Model(&model.BidAnalysisV3BlueprintGeneration{}).
				Where("id=? AND associated_bid_project_id=?", *project.BlueprintGenerationID, id).
				Update("associated_bid_project_id", 0).Error; err != nil {
				return fmt.Errorf("清除标书蓝图关联: %w", err)
			}
			if err := tx.Model(&model.BidGenProject{}).Where("id=?", id).
				Updates(map[string]any{"blueprint_generation_id": nil, "tender_project_id": nil}).Error; err != nil {
				return fmt.Errorf("释放标书项目蓝图字段: %w", err)
			}
		}
		if project.TenderProjectID > 0 && project.CreateType == "tender_file" {
			if _, err := repov3.DeleteProjectTx(tx, project.TenderProjectID, project.UserID, project.UserID, "关联标书生成项目删除"); err != nil {
				return fmt.Errorf("删除内部招标解析项目: %w", err)
			}
		}
		if err := tx.Delete(&project).Error; err != nil {
			return fmt.Errorf("删除标书项目: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *svcImpl) GetSourceSnapshot(ctx context.Context, bidProjectID int64) (*model.BidGenSourceSnapshot, error) {
	var snapshot model.BidGenSourceSnapshot
	if err := s.db.WithContext(ctx).Where("bid_project_id=?", bidProjectID).First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// ===== bid_gen_outline =====

func (s *svcImpl) AddOutlineNode(ctx context.Context, n *model.BidGenOutline) error {
	normalizeOutlineJSON(n)
	return query.Use(s.db).BidGenOutline.WithContext(ctx).Create(n)
}

func (s *svcImpl) BatchCreateOutlineNodes(ctx context.Context, nodes []*model.BidGenOutline) error {
	if len(nodes) == 0 {
		return nil
	}
	for _, n := range nodes {
		normalizeOutlineJSON(n)
	}
	return query.Use(s.db).BidGenOutline.WithContext(ctx).Create(nodes...)
}

// normalizeOutlineJSON clause_ids/material_ids 为 MySQL JSON 列，空串非法，统一为 "[]"
func normalizeOutlineJSON(n *model.BidGenOutline) {
	if strings.TrimSpace(n.ClauseIds) == "" {
		n.ClauseIds = "[]"
	}
	if strings.TrimSpace(n.MaterialIds) == "" {
		n.MaterialIds = "[]"
	}
}

func (s *svcImpl) GetOutlineByProjectID(ctx context.Context, projectID int64) ([]*model.BidGenOutline, error) {
	o := query.Use(s.db).BidGenOutline
	nodes, err := o.WithContext(ctx).Where(o.ProjectID.Eq(projectID)).Order(o.SortOrder, o.ID).Find()
	if err != nil {
		return nil, err
	}
	return biddoc.OrderOutlineTree(nodes)
}

func (s *svcImpl) GetOutlineByID(ctx context.Context, id int64) (*model.BidGenOutline, error) {
	o := query.Use(s.db).BidGenOutline
	return o.WithContext(ctx).Where(o.ID.Eq(id)).First()
}

func (s *svcImpl) UpdateOutlineNode(ctx context.Context, id int64, fields map[string]interface{}) error {
	o := query.Use(s.db).BidGenOutline
	_, err := o.WithContext(ctx).Where(o.ID.Eq(id)).Updates(fields)
	return err
}

// SetOutlineSubtreeCompleted 单事务：批量置位章节（父章节 + 全部子章节）写作状态 + 重算项目完成度。
// 两处写入必须同时生效，避免出现“章节已标记完成但项目进度/状态未同步”的中间态。
func (s *svcImpl) SetOutlineSubtreeCompleted(ctx context.Context, projectID int64, outlineIDs []int64, genStatus string, projectFields map[string]interface{}) error {
	if projectID <= 0 || len(outlineIDs) == 0 {
		return gorm.ErrRecordNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		o := query.Use(tx).BidGenOutline
		// 不校验影响行数：目标状态与当前状态一致时 MySQL 同样返回 0 行，
		// 节点存在性已由调用方按项目归属校验。
		if _, err := o.WithContext(ctx).
			Where(o.ID.In(outlineIDs...), o.ProjectID.Eq(projectID)).
			Updates(map[string]interface{}{"gen_status": genStatus}); err != nil {
			return fmt.Errorf("更新章节写作状态: %w", err)
		}
		p := query.Use(tx).BidGenProject
		if _, err := p.WithContext(ctx).Where(p.ID.Eq(projectID)).Updates(projectFields); err != nil {
			return fmt.Errorf("更新项目完成度: %w", err)
		}
		return nil
	})
}

func (s *svcImpl) DeleteOutlineByID(ctx context.Context, id int64) error {
	o := query.Use(s.db).BidGenOutline
	_, err := o.WithContext(ctx).Where(o.ID.Eq(id)).Delete()
	return err
}

func (s *svcImpl) DeleteOutlineByProjectID(ctx context.Context, projectID int64) error {
	o := query.Use(s.db).BidGenOutline
	_, err := o.WithContext(ctx).Where(o.ProjectID.Eq(projectID)).Delete()
	return err
}

func (s *svcImpl) DeleteOutlineSubtree(ctx context.Context, projectID, rootID int64) ([]int64, error) {
	if projectID <= 0 || rootID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	deletedIDs := make([]int64, 0)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var root model.BidGenOutline
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND project_id=?", rootID, projectID).First(&root).Error; err != nil {
			return err
		}
		var nodes []*model.BidGenOutline
		if err := tx.Where("project_id=?", projectID).Find(&nodes).Error; err != nil {
			return err
		}
		children := make(map[int64][]int64)
		for _, node := range nodes {
			children[node.ParentID] = append(children[node.ParentID], node.ID)
		}
		var collect func(int64)
		collect = func(id int64) {
			deletedIDs = append(deletedIDs, id)
			for _, childID := range children[id] {
				collect(childID)
			}
		}
		collect(root.ID)
		if err := tx.Where("outline_id IN ?", deletedIDs).Delete(&model.BidGenChapterContent{}).Error; err != nil {
			return fmt.Errorf("删除大纲章节正文: %w", err)
		}
		if err := tx.Where("outline_id IN ?", deletedIDs).Delete(&model.BidGenMaterialRef{}).Error; err != nil {
			return fmt.Errorf("删除大纲素材引用: %w", err)
		}
		if err := tx.Where("project_id=? AND id IN ?", projectID, deletedIDs).Delete(&model.BidGenOutline{}).Error; err != nil {
			return fmt.Errorf("删除大纲子树: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deletedIDs, nil
}

// ===== bid_gen_chapter_content =====

func (s *svcImpl) UpsertChapterContent(ctx context.Context, cc *model.BidGenChapterContent) error {
	c := query.Use(s.db).BidGenChapterContent
	existing, err := c.WithContext(ctx).Where(c.ProjectID.Eq(cc.ProjectID), c.OutlineID.Eq(cc.OutlineID)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		_, err = c.WithContext(ctx).Where(c.ID.Eq(existing.ID)).Updates(map[string]interface{}{
			"content_json": cc.ContentJSON,
			"content_html": cc.ContentHTML,
			"word_count":   cc.WordCount,
			"source":       cc.Source,
			"gen_task_id":  cc.GenTaskID,
		})
		return err
	}
	return c.WithContext(ctx).Create(cc)
}

func (s *svcImpl) GetChapterContent(ctx context.Context, projectID, outlineID int64) (*model.BidGenChapterContent, error) {
	c := query.Use(s.db).BidGenChapterContent
	return c.WithContext(ctx).Where(c.ProjectID.Eq(projectID), c.OutlineID.Eq(outlineID)).First()
}

func (s *svcImpl) GetChapterContentsByProjectID(ctx context.Context, projectID int64) ([]*model.BidGenChapterContent, error) {
	c := query.Use(s.db).BidGenChapterContent
	return c.WithContext(ctx).Where(c.ProjectID.Eq(projectID)).Find()
}

func (s *svcImpl) SaveGeneratedChapter(ctx context.Context, cc *model.BidGenChapterContent, doc *model.BidGenDocContent, taskID int64, completedCount, progress int32) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		chapter := query.Use(tx).BidGenChapterContent
		existingChapter, err := chapter.WithContext(ctx).
			Where(chapter.ProjectID.Eq(cc.ProjectID), chapter.OutlineID.Eq(cc.OutlineID)).First()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询章节正文: %w", err)
		}
		if existingChapter != nil {
			if _, err := chapter.WithContext(ctx).Where(chapter.ID.Eq(existingChapter.ID)).Updates(map[string]interface{}{
				"content_json": cc.ContentJSON,
				"content_html": cc.ContentHTML,
				"word_count":   cc.WordCount,
				"source":       cc.Source,
				"gen_task_id":  cc.GenTaskID,
			}); err != nil {
				return fmt.Errorf("更新章节正文: %w", err)
			}
		} else if err := chapter.WithContext(ctx).Create(cc); err != nil {
			return fmt.Errorf("创建章节正文: %w", err)
		}

		document := query.Use(tx).BidGenDocContent
		existingDoc, err := document.WithContext(ctx).Where(document.ProjectID.Eq(doc.ProjectID)).First()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("查询主文档: %w", err)
		}
		if existingDoc != nil {
			if _, err := document.WithContext(ctx).Where(document.ID.Eq(existingDoc.ID)).Updates(map[string]interface{}{
				"doc_json": doc.DocJSON,
				"doc_html": doc.DocHTML,
				"version":  gorm.Expr("version + 1"),
			}); err != nil {
				return fmt.Errorf("更新主文档: %w", err)
			}
		} else if err := document.WithContext(ctx).Create(doc); err != nil {
			return fmt.Errorf("创建主文档: %w", err)
		}

		outline := query.Use(tx).BidGenOutline
		if _, err := outline.WithContext(ctx).Where(outline.ID.Eq(cc.OutlineID)).Updates(map[string]interface{}{"gen_status": "succeeded"}); err != nil {
			return fmt.Errorf("更新章节状态: %w", err)
		}
		task := query.Use(tx).BidGenTask
		result, err := task.WithContext(ctx).Where(task.ID.Eq(taskID), task.Status.Eq("running")).Updates(map[string]interface{}{
			"current_outline_id": cc.OutlineID,
			"completed_count":    completedCount,
			"progress":           progress,
			"heartbeat_at":       time.Now(),
		})
		if err != nil {
			return fmt.Errorf("更新生成任务进度: %w", err)
		}
		if result.RowsAffected == 0 {
			return ErrTaskNotRunnable
		}
		return nil
	})
}

func (s *svcImpl) DeleteChapterContentByProjectID(ctx context.Context, projectID int64) error {
	c := query.Use(s.db).BidGenChapterContent
	_, err := c.WithContext(ctx).Where(c.ProjectID.Eq(projectID)).Delete()
	return err
}

// ===== bid_gen_doc_content =====

func (s *svcImpl) UpsertDocContent(ctx context.Context, dc *model.BidGenDocContent) error {
	d := query.Use(s.db).BidGenDocContent
	existing, err := d.WithContext(ctx).Where(d.ProjectID.Eq(dc.ProjectID)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		_, err = d.WithContext(ctx).Where(d.ID.Eq(existing.ID)).Updates(map[string]interface{}{
			"doc_json": dc.DocJSON,
			"doc_html": dc.DocHTML,
			"version":  gorm.Expr("version + 1"),
		})
		return err
	}
	return d.WithContext(ctx).Create(dc)
}

func (s *svcImpl) GetDocContent(ctx context.Context, projectID int64) (*model.BidGenDocContent, error) {
	d := query.Use(s.db).BidGenDocContent
	return d.WithContext(ctx).Where(d.ProjectID.Eq(projectID)).First()
}

// ===== bid_gen_task =====

func (s *svcImpl) AddTask(ctx context.Context, t *model.BidGenTask) error {
	return query.Use(s.db).BidGenTask.WithContext(ctx).Create(t)
}

func (s *svcImpl) GetTaskByID(ctx context.Context, id int64) (*model.BidGenTask, error) {
	t := query.Use(s.db).BidGenTask
	return t.WithContext(ctx).Where(t.ID.Eq(id)).First()
}

func (s *svcImpl) GetTaskForUser(ctx context.Context, userID, id int64) (*model.BidGenTask, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var task model.BidGenTask
	err := s.db.WithContext(ctx).Table("bid_gen_task t").
		Select("t.*").Joins("JOIN bid_gen_project p ON p.id=t.project_id").
		Where("t.id=? AND t.user_id=? AND p.user_id=?", id, userID, userID).First(&task).Error
	return &task, err
}

func (s *svcImpl) GetRunningTask(ctx context.Context, projectID int64) (*model.BidGenTask, error) {
	t := query.Use(s.db).BidGenTask
	return t.WithContext(ctx).
		Where(t.ProjectID.Eq(projectID), t.Status.In("pending", "running", "cancelling")).
		Order(t.ID.Desc()).First()
}

func (s *svcImpl) CreateGenerationTask(ctx context.Context, userID int64, task *model.BidGenTask) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.BidGenProject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id=? AND user_id=?", task.ProjectID, userID).First(&project).Error; err != nil {
			return err
		}
		if project.Status == "generating" {
			return ErrGenerationActive
		}
		if project.Status == "parsing" || project.Status == "outline_review" {
			return ErrTaskNotRunnable
		}
		var active int64
		if err := tx.Model(&model.BidGenTask{}).
			Where("project_id=? AND status IN ?", task.ProjectID, []string{"pending", "running", "cancelling"}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrGenerationActive
		}
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		return tx.Model(&project).Updates(map[string]interface{}{
			"status": "generating", "stage": "queued", "last_error": "",
		}).Error
	})
}

func (s *svcImpl) ClaimGenerationTask(ctx context.Context, taskID int64) (*model.BidGenTask, bool, error) {
	var task model.BidGenTask
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, taskID).Error; err != nil {
			return err
		}
		if task.Status == "cancelling" || task.Status == "cancelled" {
			return nil
		}
		if task.Status != "pending" && task.Status != "running" {
			return ErrTaskNotRunnable
		}
		now := time.Now()
		updates := map[string]interface{}{
			"status": "running", "attempt_count": gorm.Expr("attempt_count + 1"),
			"heartbeat_at": now, "error_msg": "",
		}
		if task.StartedAt == nil {
			updates["started_at"] = now
		}
		if err := tx.Model(&model.BidGenTask{}).Where("id=?", task.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BidGenProject{}).Where("id=?", task.ProjectID).
			Updates(map[string]interface{}{"status": "generating", "stage": "generating", "last_error": ""}).Error; err != nil {
			return err
		}
		return tx.First(&task, task.ID).Error
	})
	if err != nil {
		return nil, false, err
	}
	return &task, task.Status == "running", nil
}

func (s *svcImpl) ListActiveGenerationTasks(ctx context.Context) ([]*model.BidGenTask, error) {
	t := query.Use(s.db).BidGenTask
	return t.WithContext(ctx).Where(t.Status.In("pending", "running", "cancelling")).Order(t.ID).Find()
}

func (s *svcImpl) UpdateTaskFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	t := query.Use(s.db).BidGenTask
	_, err := t.WithContext(ctx).Where(t.ID.Eq(id)).Updates(fields)
	return err
}

func (s *svcImpl) UpdateTaskFieldsIfStatus(ctx context.Context, id int64, statuses []string, fields map[string]interface{}) (bool, error) {
	if len(statuses) == 0 {
		return false, nil
	}
	result := s.db.WithContext(ctx).Model(&model.BidGenTask{}).Where("id=? AND status IN ?", id, statuses).Updates(fields)
	return result.RowsAffected > 0, result.Error
}

func (s *svcImpl) FinishGeneration(ctx context.Context, projectID int64, projectFields map[string]interface{}, taskID int64, taskFields map[string]interface{}) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.BidGenTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, taskID).Error; err != nil {
			return fmt.Errorf("锁定标书生成任务: %w", err)
		}
		if current.Status == "succeeded" || current.Status == "failed" || current.Status == "cancelled" {
			return nil
		}
		desiredStatus, _ := taskFields["status"].(string)
		if desiredStatus == "succeeded" && current.Status != "running" {
			return ErrTaskNotRunnable
		}
		if desiredStatus == "failed" && current.Status == "cancelling" {
			return ErrTaskNotRunnable
		}
		project := query.Use(tx).BidGenProject
		if _, err := project.WithContext(ctx).Where(project.ID.Eq(projectID)).Updates(projectFields); err != nil {
			return fmt.Errorf("更新标书项目终态: %w", err)
		}
		task := query.Use(tx).BidGenTask
		if _, err := task.WithContext(ctx).Where(task.ID.Eq(taskID)).Updates(taskFields); err != nil {
			return fmt.Errorf("更新标书生成任务终态: %w", err)
		}
		return nil
	})
}

// ===== bid_gen_material_ref =====

func (s *svcImpl) BatchCreateMaterialRefs(ctx context.Context, refs []*model.BidGenMaterialRef) error {
	if len(refs) == 0 {
		return nil
	}
	return query.Use(s.db).BidGenMaterialRef.WithContext(ctx).Create(refs...)
}

func (s *svcImpl) GetMaterialRefsByProject(ctx context.Context, projectID int64) ([]*model.BidGenMaterialRef, error) {
	m := query.Use(s.db).BidGenMaterialRef
	return m.WithContext(ctx).Where(m.ProjectID.Eq(projectID)).Find()
}

// ===== bid_gen_export_record =====

func (s *svcImpl) AddExportRecord(ctx context.Context, r *model.BidGenExportRecord) error {
	return query.Use(s.db).BidGenExportRecord.WithContext(ctx).Create(r)
}

func (s *svcImpl) GetExportRecordsByProject(ctx context.Context, projectID int64) ([]*model.BidGenExportRecord, error) {
	e := query.Use(s.db).BidGenExportRecord
	return e.WithContext(ctx).Where(e.ProjectID.Eq(projectID)).Order(e.ID.Desc()).Find()
}

// parseOutlineIDs 解析大纲ID列表
func parseOutlineIDs(s string) []int64 {
	if s == "" {
		return nil
	}
	var ids []int64
	_ = json.Unmarshal([]byte(s), &ids)
	return ids
}

// CountOutlineByProjectIDs 批量统计章节总数与已生成数
func (s *svcImpl) CountOutlineByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64][2]int64, error) {
	if len(projectIDs) == 0 {
		return map[int64][2]int64{}, nil
	}
	o := query.Use(s.db).BidGenOutline
	rows, err := o.WithContext(ctx).Where(o.ProjectID.In(projectIDs...)).
		Select(o.ProjectID, o.ParentID, o.Title, o.GenStatus).Find()
	if err != nil {
		return nil, err
	}
	result := make(map[int64][2]int64, len(projectIDs))
	for _, r := range rows {
		if biddoc.IsDocumentRoot(r) {
			continue
		}
		v := result[r.ProjectID]
		v[0]++ // total
		if r.GenStatus == "succeeded" {
			v[1]++
		}
		result[r.ProjectID] = v
	}
	return result, nil
}

func (s *svcImpl) DeleteChapterContentsByOutlineIDs(ctx context.Context, outlineIDs []int64) error {
	if len(outlineIDs) == 0 {
		return nil
	}
	c := query.Use(s.db).BidGenChapterContent
	_, err := c.WithContext(ctx).Where(c.OutlineID.In(outlineIDs...)).Delete()
	return err
}

func (s *svcImpl) DeleteDocContentByProjectID(ctx context.Context, projectID int64) error {
	d := query.Use(s.db).BidGenDocContent
	_, err := d.WithContext(ctx).Where(d.ProjectID.Eq(projectID)).Delete()
	return err
}

func (s *svcImpl) DeleteTasksByProjectID(ctx context.Context, projectID int64) error {
	t := query.Use(s.db).BidGenTask
	_, err := t.WithContext(ctx).Where(t.ProjectID.Eq(projectID)).Delete()
	return err
}

func (s *svcImpl) DeleteMaterialRefsByProject(ctx context.Context, projectID int64) error {
	m := query.Use(s.db).BidGenMaterialRef
	_, err := m.WithContext(ctx).Where(m.ProjectID.Eq(projectID)).Delete()
	return err
}

func (s *svcImpl) DeleteExportRecordsByProject(ctx context.Context, projectID int64) error {
	e := query.Use(s.db).BidGenExportRecord
	_, err := e.WithContext(ctx).Where(e.ProjectID.Eq(projectID)).Delete()
	return err
}

// ReconcileOutline 按文档标题结构对账大纲表（单事务）：
// - 无 id 标题 → 新建节点（source=user、gen_status=pending、clause_ids/material_ids="[]"）；
// - 已存在节点 → 仅更新 title/level/parent_id/sort_order（不覆盖 gen_status/source/is_required_file 等）；
// - 文档中已消失的节点 → 删除该节点及其章节内容（后代若仍在文档中会被推导结果重新挂载，不会误删）。
// 返回“请求数组下标 → 新 id”映射，供前端回写标题节点的 outlineId 属性。
func (s *svcImpl) ReconcileOutline(ctx context.Context, projectID int64, items []entity.BidGenOutlineSyncItem) (map[int64]int64, error) {
	mapping := make(map[int64]int64, len(items))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)
		o := q.BidGenOutline
		for _, it := range items {
			parentID := int64(0)
			if it.ParentIndex >= 0 && it.ParentIndex < int64(len(items)) {
				p := items[it.ParentIndex]
				if p.OutlineID > 0 {
					parentID = p.OutlineID
				} else {
					parentID = mapping[p.Index]
				}
			}
			if it.OutlineID <= 0 {
				node := &model.BidGenOutline{
					ProjectID:   projectID,
					ParentID:    parentID,
					Level:       it.Level,
					SortOrder:   it.SortOrder,
					Title:       it.Title,
					ClauseIds:   "[]",
					MaterialIds: "[]",
					GenStatus:   "pending",
					Source:      "user",
				}
				if err := o.WithContext(ctx).Create(node); err != nil {
					return err
				}
				mapping[it.Index] = node.ID
			} else {
				fields := map[string]interface{}{
					"title":      it.Title,
					"level":      it.Level,
					"parent_id":  parentID,
					"sort_order": it.SortOrder,
				}
				// 仅更新本项目内的节点，防止越权改动其他项目的大纲
				if _, err := o.WithContext(ctx).
					Where(o.ID.Eq(it.OutlineID), o.ProjectID.Eq(projectID)).
					Updates(fields); err != nil {
					return err
				}
			}
		}
		// 删除文档中已消失的节点及其章节内容
		present := make(map[int64]bool, len(items))
		for _, it := range items {
			if it.OutlineID > 0 {
				present[it.OutlineID] = true
			}
		}
		all, err := o.WithContext(ctx).Where(o.ProjectID.Eq(projectID)).Find()
		if err != nil {
			return err
		}
		toDelete := make([]int64, 0, len(all))
		for _, n := range all {
			if !present[n.ID] {
				toDelete = append(toDelete, n.ID)
			}
		}
		if len(toDelete) > 0 {
			cc := q.BidGenChapterContent
			if _, err := cc.WithContext(ctx).Where(cc.OutlineID.In(toDelete...)).Delete(); err != nil {
				return err
			}
			if _, err := o.WithContext(ctx).Where(o.ID.In(toDelete...)).Delete(); err != nil {
				return err
			}
		}
		return nil
	})
	return mapping, err
}

// ApplyOutlineStructure 单事务批量更新大纲结构（parent_id/level/sort_order）。
// - 仅接受本项目内节点，越权 id 直接报错；
// - parent_id 必须为 0（顶层）或本项目内节点；
// - level 夹取 1-4（与大纲表约定一致）；
// - 不做删除语义，删除走 DeleteOutlineNode 级联接口。
func (s *svcImpl) ApplyOutlineStructure(ctx context.Context, projectID int64, nodes []entity.BidGenApplyOutlineNode) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := query.Use(tx)
		o := q.BidGenOutline

		all, err := o.WithContext(ctx).Where(o.ProjectID.Eq(projectID)).Find()
		if err != nil {
			return err
		}
		valid := make(map[int64]bool, len(all))
		for _, n := range all {
			valid[n.ID] = true
		}
		normalized, err := normalizeApplyNodes(valid, nodes)
		if err != nil {
			return err
		}
		for _, it := range normalized {
			if _, err := o.WithContext(ctx).Where(o.ID.Eq(it.ID), o.ProjectID.Eq(projectID)).Updates(map[string]interface{}{
				"parent_id":  it.ParentID,
				"level":      it.Level,
				"sort_order": it.SortOrder,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// normalizeApplyNodes 校验并归一化结构快照节点：
// - id 必须存在且不重复；parent_id 必须为 0 或存在于 validIDs；
// - level 夹取 1-4（与大纲表约定一致）。
func normalizeApplyNodes(validIDs map[int64]bool, nodes []entity.BidGenApplyOutlineNode) ([]entity.BidGenApplyOutlineNode, error) {
	normalized := make([]entity.BidGenApplyOutlineNode, 0, len(nodes))
	seen := make(map[int64]bool, len(nodes))
	for _, it := range nodes {
		if it.ID <= 0 {
			return nil, fmt.Errorf("大纲节点 id 非法: %d", it.ID)
		}
		if seen[it.ID] {
			return nil, fmt.Errorf("大纲节点 id 重复: %d", it.ID)
		}
		seen[it.ID] = true
		if !validIDs[it.ID] {
			return nil, fmt.Errorf("大纲节点 %d 不属于该项目", it.ID)
		}
		if it.ParentID != 0 && !validIDs[it.ParentID] {
			return nil, fmt.Errorf("大纲节点父级 %d 不属于该项目", it.ParentID)
		}
		level := it.Level
		if level < 1 {
			level = 1
		}
		if level > 4 {
			level = 4
		}
		it.Level = level
		normalized = append(normalized, it)
	}
	return normalized, nil
}
