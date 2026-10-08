package tenderintel

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

// CreateRun 创建采集批次记录。
func (r *Repository) CreateRun(ctx context.Context, run *model.TenderIntelCollectRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// GetRun 按 run_id 查询批次。
func (r *Repository) GetRun(ctx context.Context, runID string) (*model.TenderIntelCollectRun, error) {
	var item model.TenderIntelCollectRun
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListRuns 分页查询采集批次。
func (r *Repository) ListRuns(ctx context.Context, pageNum, pageSize int) ([]*model.TenderIntelCollectRun, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	q := r.db.WithContext(ctx).Model(&model.TenderIntelCollectRun{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.TenderIntelCollectRun
	err := q.Order("id DESC").
		Offset((pageNum - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

// UpdateRunCounters 累加批次统计字段。
func (r *Repository) UpdateRunCounters(ctx context.Context, runID string, deltas map[string]int) error {
	if len(deltas) == 0 {
		return nil
	}
	updates := make(map[string]interface{}, len(deltas))
	for field, delta := range deltas {
		if delta == 0 {
			continue
		}
		updates[field] = gorm.Expr(field+" + ?", delta)
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Where("run_id = ?", runID).
		Updates(updates).Error
}

// SetRunCounters 直接覆盖批次统计字段（用于收尾时按源明细重算，修正重试带来的累计偏差）。
func (r *Repository) SetRunCounters(ctx context.Context, runID string, values map[string]int) error {
	if len(values) == 0 {
		return nil
	}
	updates := make(map[string]interface{}, len(values))
	for field, value := range values {
		updates[field] = value
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Where("run_id = ?", runID).
		Updates(updates).Error
}

// FinishRun 结束批次，写入终态与错误摘要。
func (r *Repository) FinishRun(ctx context.Context, runID, status, errorSummary string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Where("run_id = ?", runID).
		Updates(map[string]interface{}{
			"status":        status,
			"finished_at":   now,
			"error_summary": errorSummary,
		}).Error
}

// GetRunningRun 返回当前处于 running 状态的批次（用于防重入）。
func (r *Repository) GetRunningRun(ctx context.Context) (*model.TenderIntelCollectRun, error) {
	var item model.TenderIntelCollectRun
	if err := r.db.WithContext(ctx).
		Where("status = ?", "running").
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListRunningRuns 返回全部 running 批次，按开始时间倒序。
func (r *Repository) ListRunningRuns(ctx context.Context) ([]*model.TenderIntelCollectRun, error) {
	var items []*model.TenderIntelCollectRun
	err := r.db.WithContext(ctx).
		Where("status = ?", "running").
		Order("started_at DESC").
		Find(&items).Error
	return items, err
}

// CloseStaleRuns 把超时仍未结束的 running 批次标记为 failed。
//
// 背景：采集任务投递后若 worker 未消费（后端重启、队列丢失、采集服务不可用），
// 批次会永远停在 running，而防重入逻辑会因此永久拒绝后续触发。
func (r *Repository) CloseStaleRuns(ctx context.Context, staleBefore time.Time, reason string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Where("status = ? AND started_at < ?", "running", staleBefore).
		Updates(map[string]interface{}{
			"status":        "failed",
			"finished_at":   time.Now(),
			"error_summary": reason,
		})
	return res.RowsAffected, res.Error
}

// GetRunSourceKeys 读取批次覆盖的源（scope=all 时返回空切片）。
func (r *Repository) GetRunSourceKeys(run *model.TenderIntelCollectRun) []string {
	if run == nil || strings.TrimSpace(run.SourceKeys) == "" {
		return nil
	}
	var keys []string
	if err := json.Unmarshal([]byte(run.SourceKeys), &keys); err != nil {
		return nil
	}
	return keys
}

// DeleteRun 删除批次及其源明细（仅删除任务历史，已入库公告不受影响）。
func (r *Repository) DeleteRun(ctx context.Context, runID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&model.TenderIntelRunSource{}).Error; err != nil {
			return err
		}
		res := tx.Where("run_id = ?", runID).Delete(&model.TenderIntelCollectRun{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// UpsertRunSource 创建或更新批次内单源明细。
func (r *Repository) UpsertRunSource(ctx context.Context, item *model.TenderIntelRunSource) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "run_id"}, {Name: "source_key"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"source_name", "status", "cursor_before", "cursor_after",
				"discovered", "extracted", "inserted", "skipped",
				"duration_ms", "error", "updated_at",
			}),
		}).
		Create(item).Error
}

// ListRunSources 返回批次内所有源明细。
func (r *Repository) ListRunSources(ctx context.Context, runID string) ([]*model.TenderIntelRunSource, error) {
	var items []*model.TenderIntelRunSource
	err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// GetRunSource 按批次与源读取明细；不存在时返回 gorm.ErrRecordNotFound。
func (r *Repository) GetRunSource(ctx context.Context, runID, sourceKey string) (*model.TenderIntelRunSource, error) {
	var item model.TenderIntelRunSource
	if err := r.db.WithContext(ctx).
		Where("run_id = ? AND source_key = ?", runID, sourceKey).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CountRunSourcesByStatus 统计批次内各状态的源数量。
func (r *Repository) CountRunSourcesByStatus(ctx context.Context, runID string) (total, success, failed int64, err error) {
	countByStatus := func(status string) (int64, error) {
		var n int64
		q := r.db.WithContext(ctx).Model(&model.TenderIntelRunSource{}).Where("run_id = ?", runID)
		if status != "" {
			q = q.Where("status = ?", status)
		}
		err := q.Count(&n).Error
		return n, err
	}
	if total, err = countByStatus(""); err != nil {
		return
	}
	if success, err = countByStatus("success"); err != nil {
		return
	}
	failed, err = countByStatus("failed")
	return
}

// RunSourceSummary 批次源明细汇总（用于收尾时按明细重算批次统计）。
type RunSourceSummary struct {
	Total      int
	Success    int
	Failed     int
	Pending    int // 仍处于 running 等中间态的源
	Discovered int
	Extracted  int
	Inserted   int
	Skipped    int
}

// SummarizeRunSources 汇总批次内全部源明细的计数与状态分布。
//
// 采集任务可能重试，逐次累加会让统计口径漂移；收尾时统一以明细为准重算。
func (r *Repository) SummarizeRunSources(ctx context.Context, runID string) (*RunSourceSummary, error) {
	var rows []struct {
		Status     string
		Total      int
		Discovered int
		Extracted  int
		Inserted   int
		Skipped    int
	}
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelRunSource{}).
		Select("status, COUNT(*) AS total, "+
			"CAST(COALESCE(SUM(discovered), 0) AS SIGNED) AS discovered, "+
			"CAST(COALESCE(SUM(extracted), 0) AS SIGNED) AS extracted, "+
			"CAST(COALESCE(SUM(inserted), 0) AS SIGNED) AS inserted, "+
			"CAST(COALESCE(SUM(skipped), 0) AS SIGNED) AS skipped").
		Where("run_id = ?", runID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := &RunSourceSummary{}
	for _, row := range rows {
		out.Total += row.Total
		out.Discovered += row.Discovered
		out.Extracted += row.Extracted
		out.Inserted += row.Inserted
		out.Skipped += row.Skipped
		switch row.Status {
		case "success":
			out.Success += row.Total
		case "failed":
			out.Failed += row.Total
		default:
			out.Pending += row.Total
		}
	}
	return out, nil
}

// CountRunsByStatus 统计全部批次的执行状态分布（管理页概览）。
func (r *Repository) CountRunsByStatus(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Total  int64
	}
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Select("status, COUNT(*) AS total").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{"total": 0, "running": 0, "success": 0, "partial": 0, "failed": 0}
	for _, row := range rows {
		out["total"] += row.Total
		if _, ok := out[row.Status]; ok {
			out[row.Status] = row.Total
		}
	}
	return out, nil
}
