package bidreview

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

// ================================================================
// 并发写入保护
//
// 判定阶段会为每个清单项并发落库，而每项写入都是“UPDATE 历史判定 → INSERT 判定
// → DELETE/INSERT 证据”这类多语句事务；REPEATABLE READ 下这些短事务会在
// idx_item_latest / uniq_item 等索引上互相持有间隙锁，出现循环等待后 MySQL 抛
// 1213 Deadlock。这里用“进程内串行化小事务 + 死锁/锁等待重试”双保险：
// 慢的是 LLM 调用（仍并发），落库本身是毫秒级，串行不会成为瓶颈。
// ================================================================

var txWriteMu sync.Mutex

// isRetryableTxError 是否属于可安全重试的事务冲突（1213 死锁 / 1205 锁等待超时）
func isRetryableTxError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1213 || mysqlErr.Number == 1205
	}
	return false
}

// withTxRetry 串行执行小事务，并在死锁/锁等待时退避重试
func (s *svcImpl) withTxRetry(ctx context.Context, fn func() error) error {
	txWriteMu.Lock()
	defer txWriteMu.Unlock()

	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); err == nil || !isRetryableTxError(err) {
			return err
		}
		s.logger.Warnw("事务冲突，退避重试", "attempt", attempt+1, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(40*(attempt+1)) * time.Millisecond):
		}
	}
	return err
}

// ================================================================
// 项目
// ================================================================

func (s *svcImpl) GetProjectByID(ctx context.Context, id int64) (*model.BidReviewV2Project, error) {
	var p model.BidReviewV2Project
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *svcImpl) GetProjectForUser(ctx context.Context, userID, id int64) (*model.BidReviewV2Project, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var p model.BidReviewV2Project
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *svcImpl) AddProject(ctx context.Context, p *model.BidReviewV2Project) error {
	return s.db.WithContext(ctx).Create(p).Error
}

func (s *svcImpl) UpdateProjectFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.BidReviewV2Project{}).Where("id = ?", id).Updates(fields).Error
}

func (s *svcImpl) UpdateProjectStageStatus(ctx context.Context, id int64, stageStatus map[string]string, stage string, progress int32) error {
	stageJSON, err := MarshalStageStatus(stageStatus)
	if err != nil {
		return err
	}
	return s.UpdateProjectFields(ctx, id, map[string]interface{}{
		"stage_status": stageJSON,
		"stage":        stage,
		"progress":     progress,
	})
}

func (s *svcImpl) GetProjectsForUser(ctx context.Context, userID int64, pageNum, pageSize int, status, keyword string) ([]*model.BidReviewV2Project, int64, error) {
	if userID <= 0 {
		return nil, 0, gorm.ErrRecordNotFound
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	q := s.db.WithContext(ctx).Model(&model.BidReviewV2Project{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]*model.BidReviewV2Project, 0, pageSize)
	if err := q.Order("created_at DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DeleteProjectCascadeForUser 级联删除审核项目及其全部子数据（事务内完成）
func (s *svcImpl) DeleteProjectCascadeForUser(ctx context.Context, userID, id int64) (*model.BidReviewV2Project, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var project model.BidReviewV2Project
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).First(&project).Error; err != nil {
			return err
		}
		tables := []struct {
			name  string
			model any
		}{
			{"文件", &model.BidReviewV2File{}},
			{"逐页文本", &model.BidReviewV2DocumentPage{}},
			{"页块", &model.BidReviewV2DocumentBlock{}},
			{"分块", &model.BidReviewV2DocumentChunk{}},
			{"术语索引", &model.BidReviewV2TermIndex{}},
			{"解析快照", &model.BidReviewV2AnalysisSnapshot{}},
			{"清单项", &model.BidReviewV2ChecklistItem{}},
			{"判定结论", &model.BidReviewV2Finding{}},
			{"证据", &model.BidReviewV2Evidence{}},
			{"整改项", &model.BidReviewV2Remediation{}},
			{"阶段运行", &model.BidReviewV2StageRun{}},
			{"导出记录", &model.BidReviewV2ExportRecord{}},
			{"操作日志", &model.BidReviewV2OpLog{}},
		}
		for _, t := range tables {
			if err := tx.Where("project_id = ?", id).Delete(t.model).Error; err != nil {
				return fmt.Errorf("删除审核%s: %w", t.name, err)
			}
		}
		if project.CreateType == "gen" && project.SourceBidGenProjectID > 0 {
			if err := tx.Model(&model.BidGenProject{}).
				Where("id = ? AND review_project_id = ?", project.SourceBidGenProjectID, id).
				Update("review_project_id", 0).Error; err != nil {
				return fmt.Errorf("重置来源标书审核标记: %w", err)
			}
		}
		if err := tx.Delete(&model.BidReviewV2Project{}, id).Error; err != nil {
			return fmt.Errorf("删除审核项目: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// ================================================================
// 文件
// ================================================================

func (s *svcImpl) BatchCreateFiles(ctx context.Context, files []*model.BidReviewV2File) error {
	if len(files) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).CreateInBatches(files, 50).Error
}

func (s *svcImpl) GetFilesByProjectAndType(ctx context.Context, projectID int64, fileType string) ([]*model.BidReviewV2File, error) {
	rows := make([]*model.BidReviewV2File, 0)
	q := s.db.WithContext(ctx).Where("project_id = ?", projectID)
	if fileType != "" {
		q = q.Where("file_type = ?", fileType)
	}
	if err := q.Order("sort_order").Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetFileByID(ctx context.Context, id int64) (*model.BidReviewV2File, error) {
	var f model.BidReviewV2File
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *svcImpl) UpdateFile(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.BidReviewV2File{}).Where("id = ?", id).Updates(fields).Error
}

// ================================================================
// 文档内容（页 / 块 / 分块 / 术语索引）
// ================================================================

// ReplaceDocumentContent 重写单文件的解析产物（重跑阶段幂等）
func (s *svcImpl) ReplaceDocumentContent(ctx context.Context, projectID, fileID int64, payload *DocumentPayload) error {
	if payload == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("file_id = ?", fileID).Delete(&model.BidReviewV2DocumentPage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("file_id = ?", fileID).Delete(&model.BidReviewV2DocumentBlock{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ? AND file_id = ?", projectID, fileID).Delete(&model.BidReviewV2TermIndex{}).Error; err != nil {
			return err
		}
		if err := tx.Where("file_id = ?", fileID).Delete(&model.BidReviewV2DocumentChunk{}).Error; err != nil {
			return err
		}
		if len(payload.Pages) > 0 {
			if err := tx.CreateInBatches(payload.Pages, 100).Error; err != nil {
				return err
			}
		}
		if len(payload.Blocks) > 0 {
			if err := tx.CreateInBatches(payload.Blocks, 200).Error; err != nil {
				return err
			}
		}
		if len(payload.Chunks) > 0 {
			if err := tx.CreateInBatches(payload.Chunks, 100).Error; err != nil {
				return err
			}
		}
		if len(payload.Terms) > 0 {
			byNo := make(map[int32]int64, len(payload.Chunks))
			for _, c := range payload.Chunks {
				byNo[c.ChunkNo] = c.ID
			}
			rows := make([]*model.BidReviewV2TermIndex, 0, len(payload.Terms))
			for _, t := range payload.Terms {
				chunkID, ok := byNo[t.ChunkNo]
				if !ok {
					continue
				}
				rows = append(rows, &model.BidReviewV2TermIndex{
					ProjectID: projectID, FileID: fileID, ChunkID: chunkID,
					Term: t.Term, PageNo: t.PageNo, Weight: t.Weight,
				})
			}
			if len(rows) > 0 {
				if err := tx.CreateInBatches(rows, 500).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *svcImpl) GetBlocksByFilePage(ctx context.Context, fileID int64, pageNo int32) ([]*model.BidReviewV2DocumentBlock, error) {
	rows := make([]*model.BidReviewV2DocumentBlock, 0)
	if err := s.db.WithContext(ctx).Where("file_id = ? AND page_no = ?", fileID, pageNo).
		Order("sort_order").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetPagesByFileRange(ctx context.Context, fileID int64, startPage, endPage int32) ([]*model.BidReviewV2DocumentPage, error) {
	rows := make([]*model.BidReviewV2DocumentPage, 0)
	if err := s.db.WithContext(ctx).
		Where("file_id = ? AND page_no >= ? AND page_no <= ?", fileID, startPage, endPage).
		Order("page_no").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetBlocksByProjectType(ctx context.Context, projectID int64, blockType string, limit int) ([]*model.BidReviewV2DocumentBlock, error) {
	if limit <= 0 {
		limit = 50
	}
	rows := make([]*model.BidReviewV2DocumentBlock, 0)
	if err := s.db.WithContext(ctx).
		Where("project_id = ? AND block_type = ?", projectID, blockType).
		Order("file_id").Order("page_no").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SearchTermIndex 术语倒排召回：按命中权重聚合到分块，返回 Top-N 分块
func (s *svcImpl) SearchTermIndex(ctx context.Context, projectID int64, terms []string, limit int) ([]*TermHit, error) {
	if len(terms) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 12
	}
	type aggRow struct {
		ChunkID int64
		Score   float64
	}
	agg := make([]aggRow, 0, limit)
	if err := s.db.WithContext(ctx).
		Table("bid_review_v2_term_index").
		Select("chunk_id, SUM(weight) AS score").
		Where("project_id = ? AND term IN ?", projectID, terms).
		Group("chunk_id").
		Order("score DESC").
		Limit(limit).
		Scan(&agg).Error; err != nil {
		return nil, err
	}
	if len(agg) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(agg))
	scoreByID := make(map[int64]float64, len(agg))
	for _, r := range agg {
		ids = append(ids, r.ChunkID)
		scoreByID[r.ChunkID] = r.Score
	}
	chunks := make([]*model.BidReviewV2DocumentChunk, 0, len(ids))
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&chunks).Error; err != nil {
		return nil, err
	}
	hits := make([]*TermHit, 0, len(chunks))
	for _, c := range chunks {
		hits = append(hits, &TermHit{
			ChunkID: c.ID, FileID: c.FileID, PageStart: c.PageStart, PageEnd: c.PageEnd,
			Content: c.Content, Score: scoreByID[c.ID],
		})
	}
	return hits, nil
}

// DeleteDocumentContentByProject 清空项目的解析产物（阶段重跑幂等）
func (s *svcImpl) DeleteDocumentContentByProject(ctx context.Context, projectID int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{
			&model.BidReviewV2DocumentPage{},
			&model.BidReviewV2DocumentBlock{},
			&model.BidReviewV2DocumentChunk{},
			&model.BidReviewV2TermIndex{},
		} {
			if err := tx.Where("project_id = ?", projectID).Delete(m).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteDocumentContentByType 仅清理指定类型文件的页、块和索引，供阶段重跑使用。
func (s *svcImpl) DeleteDocumentContentByType(ctx context.Context, projectID int64, fileType string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fileIDs := tx.Model(&model.BidReviewV2File{}).Select("id").Where("project_id = ? AND file_type = ?", projectID, fileType)
		for _, row := range []any{&model.BidReviewV2DocumentPage{}, &model.BidReviewV2DocumentBlock{}, &model.BidReviewV2DocumentChunk{}, &model.BidReviewV2TermIndex{}} {
			if err := tx.Where("project_id = ? AND file_id IN (?)", projectID, fileIDs).Delete(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ================================================================
// 解析快照
// ================================================================

func (s *svcImpl) SaveSnapshot(ctx context.Context, snap *model.BidReviewV2AnalysisSnapshot) error {
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"analysis_project_id", "analysis_run_id", "snapshot_json", "updated_at"}),
		}).Create(snap).Error
	})
}

func (s *svcImpl) GetSnapshot(ctx context.Context, projectID int64) (*model.BidReviewV2AnalysisSnapshot, error) {
	var snap model.BidReviewV2AnalysisSnapshot
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *svcImpl) DeleteSnapshot(ctx context.Context, projectID int64) error {
	return s.db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&model.BidReviewV2AnalysisSnapshot{}).Error
}

// ================================================================
// 清单项
// ================================================================

func (s *svcImpl) BatchCreateChecklistItems(ctx context.Context, items []*model.BidReviewV2ChecklistItem) error {
	if len(items) == 0 {
		return nil
	}
	return s.createIgnoreDuplicates(ctx, items)
}

// createIgnoreDuplicates “重复则跳过”插入（清单项 item_key 唯一，重跑/复用解析依据时会出现重复键）。
// MySQL 必须用 INSERT IGNORE：clause.OnConflict{DoNothing} 会被渲染成
// `ON DUPLICATE KEY UPDATE id=id`（对自增主键赋值），多行语句一旦命中重复键，
// MySQL 8.0.20+/9.x 直接报 1869 Auto-increment value in UPDATE conflicts with internally generated values。
func (s *svcImpl) createIgnoreDuplicates(ctx context.Context, items []*model.BidReviewV2ChecklistItem) error {
	tx := s.db.WithContext(ctx)
	dialect := ""
	if tx.Dialector != nil {
		dialect = tx.Dialector.Name()
	}
	return tx.Clauses(ignoreDuplicatesClause(dialect)).CreateInBatches(items, 100).Error
}

// ignoreDuplicatesClause “重复则跳过”子句：MySQL 用 INSERT IGNORE，其余方言用 ON CONFLICT DO NOTHING。
func ignoreDuplicatesClause(dialect string) clause.Expression {
	if dialect == "mysql" {
		return clause.Insert{Modifier: "IGNORE"}
	}
	return clause.OnConflict{DoNothing: true}
}

func (s *svcImpl) GetChecklistItems(ctx context.Context, projectID int64) ([]*model.BidReviewV2ChecklistItem, error) {
	rows := make([]*model.BidReviewV2ChecklistItem, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).
		Order("sort_order").Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetChecklistItemByID(ctx context.Context, id int64) (*model.BidReviewV2ChecklistItem, error) {
	var item model.BidReviewV2ChecklistItem
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *svcImpl) UpdateChecklistItem(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.BidReviewV2ChecklistItem{}).Where("id = ?", id).Updates(fields).Error
}

func (s *svcImpl) DeleteChecklistItem(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("checklist_item_id = ?", id).Delete(&model.BidReviewV2Finding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("checklist_item_id = ?", id).Delete(&model.BidReviewV2Evidence{}).Error; err != nil {
			return err
		}
		if err := tx.Where("checklist_item_id = ?", id).Delete(&model.BidReviewV2Remediation{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.BidReviewV2ChecklistItem{}, id).Error
	})
}

// DeleteChecklistItemsBySource 重跑清单生成时清理非人工项
func (s *svcImpl) DeleteChecklistItemsBySource(ctx context.Context, projectID int64, sources []string) error {
	if len(sources) == 0 {
		return nil
	}
	var ids []int64
	if err := s.db.WithContext(ctx).Model(&model.BidReviewV2ChecklistItem{}).
		Where("project_id = ? AND source IN ? AND is_user_edited = 0", projectID, sources).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("checklist_item_id IN ?", ids).Delete(&model.BidReviewV2Finding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("checklist_item_id IN ?", ids).Delete(&model.BidReviewV2Evidence{}).Error; err != nil {
			return err
		}
		if err := tx.Where("checklist_item_id IN ?", ids).Delete(&model.BidReviewV2Remediation{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.BidReviewV2ChecklistItem{}).Error
	})
}

// CountChecklistItems 统计清单结论分布（按最新判定）
func (s *svcImpl) CountChecklistItems(ctx context.Context, projectID int64) (total, passed, warning, errCount int64, err error) {
	type row struct {
		Total   int64
		Passed  int64
		Warning int64
		ErrCnt  int64
	}
	var r row
	err = s.db.WithContext(ctx).
		Table("bid_review_v2_checklist_item AS c").
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN f.status = 'pass' THEN 1 ELSE 0 END), 0) AS passed,
			COALESCE(SUM(CASE WHEN f.status = 'warning' THEN 1 ELSE 0 END), 0) AS warning,
			COALESCE(SUM(CASE WHEN f.status IN ('error','not_found') THEN 1 ELSE 0 END), 0) AS err_cnt`).
		Joins("LEFT JOIN bid_review_v2_finding AS f ON f.checklist_item_id = c.id AND f.is_latest = 1").
		Where("c.project_id = ?", projectID).
		Scan(&r).Error
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return r.Total, r.Passed, r.Warning, r.ErrCnt, nil
}

// ================================================================
// 判定与证据
// ================================================================

// SaveVerdict 一次事务写入判定 + 证据 + 整改项（判定阶段每个清单项只开一个事务，
// 减少锁竞争面；整改项状态由 upsert 保留，不覆盖人工已处理的进度）
func (s *svcImpl) SaveVerdict(ctx context.Context, finding *model.BidReviewV2Finding, evidences []*model.BidReviewV2Evidence, remediation *model.BidReviewV2Remediation) error {
	if finding == nil {
		return nil
	}
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := writeFindingTx(tx, finding, evidences); err != nil {
				return err
			}
			if remediation == nil {
				// 复检后已通过的清单项不应继续保留尚未处理的旧整改待办。
				return tx.Where("checklist_item_id = ? AND status = ?", finding.ChecklistItemID, "todo").
					Delete(&model.BidReviewV2Remediation{}).Error
			}
			remediation.ID = 0
			remediation.FindingID = finding.ID
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "checklist_item_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"finding_id", "severity", "suggestion", "title", "dimension", "updated_at"}),
			}).Create(remediation).Error
		})
	})
}

// writeFindingTx 判定与证据的写入主体（在给定事务内执行）
func writeFindingTx(tx *gorm.DB, finding *model.BidReviewV2Finding, evidences []*model.BidReviewV2Evidence) error {
	if err := tx.Model(&model.BidReviewV2Finding{}).
		Where("checklist_item_id = ?", finding.ChecklistItemID).
		Update("is_latest", false).Error; err != nil {
		return err
	}
	finding.IsLatest = true
	// GORM 在事务失败后仍会把自增 ID 写回对象；重试必须重新申请主键。
	finding.ID = 0
	if err := tx.Create(finding).Error; err != nil {
		return err
	}
	if err := tx.Where("checklist_item_id = ?", finding.ChecklistItemID).
		Delete(&model.BidReviewV2Evidence{}).Error; err != nil {
		return err
	}
	if len(evidences) == 0 {
		return nil
	}
	for _, e := range evidences {
		e.ID = 0
		e.ProjectID = finding.ProjectID
		e.ChecklistItemID = finding.ChecklistItemID
		e.FindingID = finding.ID
	}
	return tx.CreateInBatches(evidences, 100).Error
}

func (s *svcImpl) GetLatestFindings(ctx context.Context, projectID int64) ([]*model.BidReviewV2Finding, error) {
	rows := make([]*model.BidReviewV2Finding, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ? AND is_latest = 1", projectID).
		Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetLatestFindingByItem(ctx context.Context, itemID int64) (*model.BidReviewV2Finding, error) {
	var f model.BidReviewV2Finding
	if err := s.db.WithContext(ctx).Where("checklist_item_id = ? AND is_latest = 1", itemID).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *svcImpl) GetEvidencesByProject(ctx context.Context, projectID int64) ([]*model.BidReviewV2Evidence, error) {
	rows := make([]*model.BidReviewV2Evidence, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteFindingsByProject 清空项目判定与证据（判定阶段重跑幂等；整改项状态保留）
func (s *svcImpl) DeleteFindingsByProject(ctx context.Context, projectID int64) error {
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("project_id = ?", projectID).Delete(&model.BidReviewV2Evidence{}).Error; err != nil {
				return err
			}
			return tx.Where("project_id = ?", projectID).Delete(&model.BidReviewV2Finding{}).Error
		})
	})
}

// DeleteFindingsByDimension 只清理指定维度的判定与证据，避免版式阶段重跑抹掉逐条判定。
func (s *svcImpl) DeleteFindingsByDimension(ctx context.Context, projectID int64, dimension string) error {
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			itemIDs := tx.Model(&model.BidReviewV2ChecklistItem{}).
				Select("id").Where("project_id = ? AND dimension = ?", projectID, dimension)
			if err := tx.Where("project_id = ? AND checklist_item_id IN (?)", projectID, itemIDs).
				Delete(&model.BidReviewV2Evidence{}).Error; err != nil {
				return err
			}
			return tx.Where("project_id = ? AND checklist_item_id IN (?)", projectID, itemIDs).
				Delete(&model.BidReviewV2Finding{}).Error
		})
	})
}

// ================================================================
// 整改
// ================================================================

func (s *svcImpl) UpsertRemediation(ctx context.Context, r *model.BidReviewV2Remediation) error {
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "checklist_item_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"finding_id", "severity", "suggestion", "title", "dimension", "updated_at"}),
		}).Create(r).Error
	})
}

func (s *svcImpl) GetRemediations(ctx context.Context, projectID int64) ([]*model.BidReviewV2Remediation, error) {
	rows := make([]*model.BidReviewV2Remediation, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetRemediationByID(ctx context.Context, id int64) (*model.BidReviewV2Remediation, error) {
	var r model.BidReviewV2Remediation
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *svcImpl) UpdateRemediation(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.BidReviewV2Remediation{}).Where("id = ?", id).Updates(fields).Error
}

func (s *svcImpl) CountTodoRemediations(ctx context.Context, projectID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.BidReviewV2Remediation{}).
		Where("project_id = ? AND status IN ?", projectID, []string{RemediationTodo, RemediationDoing}).
		Count(&n).Error
	return n, err
}

// ================================================================
// 规则库
// ================================================================

func (s *svcImpl) ListRules(ctx context.Context, userID int64, dimension string, enabledOnly bool, keyword string) ([]*model.BidReviewV2Rule, error) {
	rows := make([]*model.BidReviewV2Rule, 0)
	q := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if dimension != "" {
		q = q.Where("dimension = ?", dimension)
	}
	if enabledOnly {
		q = q.Where("enabled = 1")
	}
	if keyword != "" {
		q = q.Where("title LIKE ?", "%"+keyword+"%")
	}
	if err := q.Order("dimension").Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) GetRuleByID(ctx context.Context, id int64) (*model.BidReviewV2Rule, error) {
	var r model.BidReviewV2Rule
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *svcImpl) AddRule(ctx context.Context, r *model.BidReviewV2Rule) error {
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *svcImpl) UpdateRule(ctx context.Context, userID, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	res := s.db.WithContext(ctx).Model(&model.BidReviewV2Rule{}).
		Where("id = ? AND user_id = ?", id, userID).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *svcImpl) DeleteRule(ctx context.Context, userID, id int64) error {
	res := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.BidReviewV2Rule{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *svcImpl) IncrRuleHit(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.BidReviewV2Rule{}).
		Where("id IN ?", ids).
		UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}

// ================================================================
// 阶段运行
// ================================================================

func (s *svcImpl) UpsertStageRun(ctx context.Context, r *model.BidReviewV2StageRun) error {
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "project_id"}, {Name: "stage"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"status", "attempts", "total", "completed", "failed", "progress",
				"last_error", "detail_json", "started_at", "finished_at", "updated_at",
			}),
		}).Create(r).Error
	})
}

func (s *svcImpl) UpdateStageRun(ctx context.Context, projectID int64, stage string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return s.withTxRetry(ctx, func() error {
		return s.db.WithContext(ctx).Model(&model.BidReviewV2StageRun{}).
			Where("project_id = ? AND stage = ?", projectID, stage).Updates(fields).Error
	})
}

func (s *svcImpl) GetStageRuns(ctx context.Context, projectID int64) ([]*model.BidReviewV2StageRun, error) {
	rows := make([]*model.BidReviewV2StageRun, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ================================================================
// 操作日志 / 导出
// ================================================================

func (s *svcImpl) AddOpLog(ctx context.Context, log *model.BidReviewV2OpLog) error {
	return s.db.WithContext(ctx).Create(log).Error
}

func (s *svcImpl) GetOpLogs(ctx context.Context, projectID int64, limit int) ([]*model.BidReviewV2OpLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows := make([]*model.BidReviewV2OpLog, 0)
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *svcImpl) AddExportRecord(ctx context.Context, r *model.BidReviewV2ExportRecord) error {
	return s.db.WithContext(ctx).Create(r).Error
}

// GetCompanyName 读取用户所属公司名称（暗标身份信息比对基准）
func (s *svcImpl) GetCompanyName(ctx context.Context, companyID int32) (string, error) {
	if companyID <= 0 {
		return "", nil
	}
	var name string
	err := s.db.WithContext(ctx).Table("company").Select("name").Where("id = ?", companyID).Scan(&name).Error
	if err != nil {
		return "", err
	}
	return name, nil
}
