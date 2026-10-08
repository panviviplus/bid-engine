package tenderintel

import (
	"context"
	"time"

	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
)

// ── 订阅匹配任务账本 ────────────────────────────────────────────

// MatchTaskFilter 匹配任务列表过滤条件。
type MatchTaskFilter struct {
	Status   string
	TaskType string
	PageNum  int
	PageSize int
}

// CreateMatchTask 新建匹配任务。
func (r *Repository) CreateMatchTask(ctx context.Context, item *model.TenderIntelMatchTask) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// GetMatchTask 按任务号查询匹配任务。
func (r *Repository) GetMatchTask(ctx context.Context, taskNo string) (*model.TenderIntelMatchTask, error) {
	var item model.TenderIntelMatchTask
	if err := r.db.WithContext(ctx).Where("task_no = ?", taskNo).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateMatchTask 更新匹配任务字段。
func (r *Repository) UpdateMatchTask(ctx context.Context, taskNo string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelMatchTask{}).
		Where("task_no = ?", taskNo).
		Updates(updates).Error
}

// UpdateMatchTaskIfStatus 条件更新（状态前置校验的原子实现）。
func (r *Repository) UpdateMatchTaskIfStatus(
	ctx context.Context,
	taskNo string,
	statuses []string,
	updates map[string]interface{},
) (bool, error) {
	if len(updates) == 0 || len(statuses) == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.TenderIntelMatchTask{}).
		Where("task_no = ? AND status IN ?", taskNo, statuses).
		Updates(updates)
	return res.RowsAffected > 0, res.Error
}

// DeleteMatchTask 删除匹配任务账本行。
func (r *Repository) DeleteMatchTask(ctx context.Context, taskNo string) error {
	return r.db.WithContext(ctx).
		Where("task_no = ?", taskNo).
		Delete(&model.TenderIntelMatchTask{}).Error
}

// FindMatchTaskByScope 按范围查找已有的匹配任务（用于重复触发时去重）。
//
// 已取消或已失败的任务不算“已存在”，管理员重新入队后仍可再建新任务。
func (r *Repository) FindMatchTaskByScope(ctx context.Context, scopeKind, scopeRef string) (*model.TenderIntelMatchTask, error) {
	var item model.TenderIntelMatchTask
	if err := r.db.WithContext(ctx).
		Where("scope_kind = ? AND scope_ref = ? AND status IN ?", scopeKind, scopeRef, []string{"pending", "running", "success"}).
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FindRunningMatchTaskBySubscription 查找该订阅尚未结束的回溯任务。
func (r *Repository) FindRunningMatchTaskBySubscription(ctx context.Context, subID int64) (*model.TenderIntelMatchTask, error) {
	var item model.TenderIntelMatchTask
	if err := r.db.WithContext(ctx).
		Where("subscription_id = ? AND status IN ?", subID, []string{"pending", "running"}).
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListMatchTasks 分页查询匹配任务。
func (r *Repository) ListMatchTasks(ctx context.Context, f MatchTaskFilter) ([]*model.TenderIntelMatchTask, int64, error) {
	if f.PageNum <= 0 {
		f.PageNum = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	q := r.db.WithContext(ctx).Model(&model.TenderIntelMatchTask{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.TaskType != "" {
		q = q.Where("task_type = ?", f.TaskType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.TenderIntelMatchTask
	err := q.Order("id DESC").
		Offset((f.PageNum - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&items).Error
	return items, total, err
}

// CountMatchTasksByStatus 按状态统计任务数（管理端概览）。
func (r *Repository) CountMatchTasksByStatus(ctx context.Context) (map[string]int64, error) {
	type statusRow struct {
		Status string `gorm:"column:status"`
		Total  int64  `gorm:"column:cnt"`
	}
	rows := make([]statusRow, 0)
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelMatchTask{}).
		Select("status, COUNT(*) AS cnt").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, item := range rows {
		out[item.Status] = item.Total
	}
	return out, nil
}

// CountMatchAlertsSince 统计某个时间点之后完成任务产出的提醒数。
func (r *Repository) CountMatchAlertsSince(ctx context.Context, since time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelMatchTask{}).
		Where("finished_at IS NOT NULL AND finished_at >= ?", since).
		Select("COALESCE(SUM(alert_count), 0)").
		Scan(&total).Error
	return total, err
}

// CloseStaleMatchTasks 把超时仍未结束的执行中任务收尾为失败。
func (r *Repository) CloseStaleMatchTasks(ctx context.Context, staleBefore time.Time, reason string) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&model.TenderIntelMatchTask{}).
		Where("status = ? AND started_at IS NOT NULL AND started_at < ?", "running", staleBefore).
		Updates(map[string]interface{}{
			"status":      "failed",
			"last_error":  reason,
			"finished_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// AddCollectRunMatchedCount 累加采集批次的匹配提醒数（同一批次可能有多个任务）。
func (r *Repository) AddCollectRunMatchedCount(ctx context.Context, runID string, delta int) error {
	if runID == "" || delta == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelCollectRun{}).
		Where("run_id = ?", runID).
		UpdateColumn("matched_count", gorm.Expr("matched_count + ?", delta)).Error
}

// ── 任务的情报范围 ──────────────────────────────────────────────

// MatchScope 匹配任务的情报范围。
//
// 范围在任务创建瞬间固定：UpToID 是当时的情报 ID 上界，Cursor 是任务自身断点，
// 两者共同保证任务之间互不影响（新采集到的情报不会漏进旧任务）。
type MatchScope struct {
	Kind     string // run / batch / notice / window
	Ref      string
	NoticeID int64
	From     *time.Time
	To       *time.Time
	UpToID   int64
	Cursor   int64
	Limit    int
}

// MaxNoticeID 当前库内最大的情报 ID（创建任务时用于快照范围上界）。
func (r *Repository) MaxNoticeID(ctx context.Context) (int64, error) {
	var maxID int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Select("COALESCE(MAX(id), 0)").
		Scan(&maxID).Error
	return maxID, err
}

// matchScopeQuery 把范围翻译成情报查询，只取在架情报。
func (r *Repository) matchScopeQuery(ctx context.Context, s MatchScope) *gorm.DB {
	q := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("status = ?", "normal")
	switch s.Kind {
	case "run":
		q = q.Where("collect_run_id = ?", s.Ref)
	case "batch":
		q = q.Where("import_batch = ?", s.Ref)
	case "notice":
		q = q.Where("id = ?", s.NoticeID)
	default:
		// window（管理端补扫）与订阅回溯：发布时间窗 + 固定上界
		if s.From != nil {
			q = q.Where("publish_date >= ?", *s.From)
		}
		if s.To != nil {
			q = q.Where("publish_date <= ?", *s.To)
		}
	}
	if s.UpToID > 0 {
		q = q.Where("id <= ?", s.UpToID)
	}
	return q
}

// CountNoticesForMatch 统计范围内情报数。
func (r *Repository) CountNoticesForMatch(ctx context.Context, s MatchScope) (int64, error) {
	var total int64
	err := r.matchScopeQuery(ctx, s).Count(&total).Error
	return total, err
}

// ListNoticesForMatch 按范围取一批情报（ID 升序，从断点之后继续）。
func (r *Repository) ListNoticesForMatch(ctx context.Context, s MatchScope) ([]*model.TenderIntelNotice, error) {
	limit := s.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := r.matchScopeQuery(ctx, s)
	// 单条情报范围没有断点概念，重复执行也是幂等的
	if s.Kind != "notice" && s.Cursor > 0 {
		q = q.Where("id > ?", s.Cursor)
	}
	var items []*model.TenderIntelNotice
	err := q.Order("id ASC").Limit(limit).Find(&items).Error
	return items, err
}

// ── 订阅回溯状态 ────────────────────────────────────────────────

// SetSubscriptionBackfill 更新订阅的回溯状态。
func (r *Repository) SetSubscriptionBackfill(ctx context.Context, subID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelSubscription{}).
		Where("id = ?", subID).
		Updates(updates).Error
}

// GetSubscriptionByID 按主键查订阅（回溯任务执行时需要，不校验归属）。
func (r *Repository) GetSubscriptionByID(ctx context.Context, id int64) (*model.TenderIntelSubscription, error) {
	var item model.TenderIntelSubscription
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListSubscriptionsByIDs 批量取订阅（管理端展示订阅名称用，避免逐条查库）。
func (r *Repository) ListSubscriptionsByIDs(ctx context.Context, ids []int64) (map[int64]*model.TenderIntelSubscription, error) {
	out := make(map[int64]*model.TenderIntelSubscription, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var items []*model.TenderIntelSubscription
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		out[item.ID] = item
	}
	return out, nil
}

// ListMatchUsersByIDs 批量取用户展示信息（昵称/登录名/手机号）。
//
// 只读展示用途：管理端要能在匹配任务里认出“这条任务对应哪个用户”，
// 因此这里直接按主键批量读 user 表，而不是让视图层逐条回调用户服务。
func (r *Repository) ListMatchUsersByIDs(ctx context.Context, ids []int64) (map[int64]*model.User, error) {
	out := make(map[int64]*model.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var items []*model.User
	if err := r.db.WithContext(ctx).Where("user_id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		out[item.UserID] = item
	}
	return out, nil
}
