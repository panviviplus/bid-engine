package tenderintel

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
)

// ── 订阅规则 ────────────────────────────────────────────────────

// ListSubscriptions 返回用户的全部订阅，按创建时间倒序。
func (r *Repository) ListSubscriptions(ctx context.Context, userID int64) ([]*model.TenderIntelSubscription, error) {
	var items []*model.TenderIntelSubscription
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

// ListEnabledSubscriptions 返回全部启用中的订阅（用于新公告匹配）。
func (r *Repository) ListEnabledSubscriptions(ctx context.Context) ([]*model.TenderIntelSubscription, error) {
	var items []*model.TenderIntelSubscription
	err := r.db.WithContext(ctx).
		Where("enabled = ?", 1).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// GetSubscription 按主键与用户查询订阅。
func (r *Repository) GetSubscription(ctx context.Context, userID, id int64) (*model.TenderIntelSubscription, error) {
	var item model.TenderIntelSubscription
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateSubscription 新增订阅。
func (r *Repository) CreateSubscription(ctx context.Context, item *model.TenderIntelSubscription) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// UpdateSubscription 保存订阅。
func (r *Repository) UpdateSubscription(ctx context.Context, item *model.TenderIntelSubscription) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// DeleteSubscription 删除订阅（同时清理其产生的提醒）。
func (r *Repository) DeleteSubscription(ctx context.Context, userID, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", id, userID).
			Delete(&model.TenderIntelSubscription{}).Error; err != nil {
			return err
		}
		return tx.Where("subscription_id = ? AND user_id = ?", id, userID).
			Delete(&model.TenderIntelAlert{}).Error
	})
}

// SetSubscriptionEnabled 启停订阅。
func (r *Repository) SetSubscriptionEnabled(ctx context.Context, userID, id int64, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelSubscription{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("enabled", value).Error
}

// BumpSubscriptionMatched 记录订阅命中次数与最近命中时间。
func (r *Repository) BumpSubscriptionMatched(ctx context.Context, ids []int64, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelSubscription{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"last_matched_at": at,
			"matched_count":   gorm.Expr("matched_count + 1"),
		}).Error
}

// ── 站内提醒 ────────────────────────────────────────────────────

// CreateAlertsIgnoreDuplicate 批量写入提醒，冲突（同用户+同订阅+同公告）时跳过。
func (r *Repository) CreateAlertsIgnoreDuplicate(ctx context.Context, items []*model.TenderIntelAlert) (int64, error) {
	if len(items) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&items)
	return res.RowsAffected, res.Error
}

// alertsQuery 组装“当前用户的提醒”基础查询，可选按订阅收敛。
// subID <= 0 表示不限订阅；越权边界始终由 user_id 兜底。
func (r *Repository) alertsQuery(ctx context.Context, userID, subID int64) *gorm.DB {
	q := r.db.WithContext(ctx).
		Model(&model.TenderIntelAlert{}).
		Where("user_id = ?", userID)
	if subID > 0 {
		q = q.Where("subscription_id = ?", subID)
	}
	return q
}

// ListAlerts 分页查询用户提醒；subID > 0 时只看该订阅命中的提醒。
func (r *Repository) ListAlerts(ctx context.Context, userID, subID int64, unreadOnly bool, pageNum, pageSize int) ([]*model.TenderIntelAlert, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	countQ := r.alertsQuery(ctx, userID, subID)
	if unreadOnly {
		countQ = countQ.Where("is_read = ?", 0)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	listQ := r.alertsQuery(ctx, userID, subID)
	if unreadOnly {
		listQ = listQ.Where("is_read = ?", 0)
	}
	var items []*model.TenderIntelAlert
	err := listQ.Order("is_read ASC, id DESC").
		Offset((pageNum - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

// CountUnreadAlerts 统计用户未读提醒数；subID > 0 时只统计该订阅。
func (r *Repository) CountUnreadAlerts(ctx context.Context, userID, subID int64) (int64, error) {
	var total int64
	err := r.alertsQuery(ctx, userID, subID).
		Where("is_read = ?", 0).
		Count(&total).Error
	return total, err
}

// subscriptionUnreadRow 订阅未读聚合的扫描行。
type subscriptionUnreadRow struct {
	SubscriptionID int64 `gorm:"column:subscription_id"`
	Total          int64 `gorm:"column:cnt"`
}

// CountUnreadAlertsBySubscription 一次性统计用户各订阅的未读数。
//
// 订阅列表需要逐条展示未读数，逐条 COUNT 会退化成 N+1；这里用一次
// GROUP BY 聚合，调用方按 subscription_id 查表即可。
func (r *Repository) CountUnreadAlertsBySubscription(ctx context.Context, userID int64) (map[int64]int64, error) {
	rows := make([]subscriptionUnreadRow, 0)
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelAlert{}).
		Select("subscription_id, COUNT(*) AS cnt").
		Where("user_id = ? AND is_read = ?", userID, 0).
		Group("subscription_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.SubscriptionID] = row.Total
	}
	return out, nil
}

// CountSubscriptions 统计用户订阅数。
func (r *Repository) CountSubscriptions(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelSubscription{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	return total, err
}

// SetAlertsStatus 标记提醒已读或未读。
//
// ids 为空表示“当前用户（或指定订阅）的全部提醒”；取消已读时清空 read_at，
// 保证“设为未读”后再次标记已读能拿到新的读取时间。
func (r *Repository) SetAlertsStatus(ctx context.Context, userID, subID int64, ids []int64, isRead bool) error {
	q := r.alertsQuery(ctx, userID, subID)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	if isRead {
		// 只处理未读行，避免刷新已读行的时间
		q = q.Where("is_read = ?", 0)
		return q.Updates(map[string]interface{}{
			"is_read": 1,
			"read_at": time.Now(),
		}).Error
	}
	return q.Updates(map[string]interface{}{
		"is_read": 0,
		"read_at": nil,
	}).Error
}

// DeleteAlerts 硬删除指定提醒（只允许删当前用户自己的）；ids 不能为空。
func (r *Repository) DeleteAlerts(ctx context.Context, userID int64, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Where("user_id = ? AND id IN ?", userID, ids).
		Delete(&model.TenderIntelAlert{})
	return res.RowsAffected, res.Error
}

// ── 收藏 ────────────────────────────────────────────────────────

// SetFavorite 收藏或取消收藏。
func (r *Repository) SetFavorite(ctx context.Context, userID, noticeID int64, favorite bool) error {
	if favorite {
		item := &model.TenderIntelFavorite{UserID: userID, NoticeID: noticeID}
		return r.db.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(item).Error
	}
	return r.db.WithContext(ctx).
		Where("user_id = ? AND notice_id = ?", userID, noticeID).
		Delete(&model.TenderIntelFavorite{}).Error
}

// ListFavoriteNoticeIDs 返回用户收藏的公告 ID 列表。
func (r *Repository) ListFavoriteNoticeIDs(ctx context.Context, userID int64) ([]int64, error) {
	out := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelFavorite{}).
		Where("user_id = ?", userID).
		Order("id DESC").
		Pluck("notice_id", &out).Error
	return out, err
}

// IsFavorite 判断某公告是否已被用户收藏。
func (r *Repository) IsFavorite(ctx context.Context, userID, noticeID int64) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelFavorite{}).
		Where("user_id = ? AND notice_id = ?", userID, noticeID).
		Count(&total).Error
	return total > 0, err
}
