// Package tenderintel 提供招标情报站的数据访问能力。
//
// 表结构见 docs/sql/tender-intel.sql；全库无外键，表间一致性由本层与应用层维护。
package tenderintel

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
)

// DefaultCollectCron 默认自动采集节奏：每天 06:00 一轮。
const DefaultCollectCron = "0 0 6 * * *"

// Repository 招标情报站数据访问层。
type Repository struct {
	db *gorm.DB
}

// New 创建数据访问层。
func New() *Repository {
	return &Repository{db: storage.GetDB()}
}

// NewWithDB 使用指定连接创建数据访问层（测试用）。
func NewWithDB(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ── 采集源 ──────────────────────────────────────────────────────

// ListSources 返回全部采集源，按 priority 升序。
func (r *Repository) ListSources(ctx context.Context) ([]*model.TenderIntelSource, error) {
	var items []*model.TenderIntelSource
	err := r.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&items).Error
	return items, err
}

// ListEnabledSources 返回启用的采集源，按 priority 升序。
func (r *Repository) ListEnabledSources(ctx context.Context) ([]*model.TenderIntelSource, error) {
	var items []*model.TenderIntelSource
	err := r.db.WithContext(ctx).
		Where("enabled = ?", 1).
		Order("priority ASC, id ASC").
		Find(&items).Error
	return items, err
}

// GetSource 按 source_key 查询采集源。
func (r *Repository) GetSource(ctx context.Context, sourceKey string) (*model.TenderIntelSource, error) {
	var item model.TenderIntelSource
	if err := r.db.WithContext(ctx).Where("source_key = ?", sourceKey).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateSourceEnabled 启停采集源。
func (r *Repository) UpdateSourceEnabled(ctx context.Context, sourceKey string, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelSource{}).
		Where("source_key = ?", sourceKey).
		Update("enabled", value).Error
}

// UpdateSourceFields 按字段更新采集源（编辑、启停、优先级调整共用）。
func (r *Repository) UpdateSourceFields(ctx context.Context, sourceKey string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.TenderIntelSource{}).
		Where("source_key = ?", sourceKey).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteSource 删除采集源（批次历史保留，便于追溯）。
func (r *Repository) DeleteSource(ctx context.Context, sourceKey string) error {
	res := r.db.WithContext(ctx).
		Where("source_key = ?", sourceKey).
		Delete(&model.TenderIntelSource{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpsertSources 批量导入采集源：source_key 已存在时按 overwrite 决定更新或跳过。
// 返回值：新增数、更新数、跳过数。
func (r *Repository) UpsertSources(ctx context.Context, sources []*model.TenderIntelSource, overwrite bool) (int, int, int, error) {
	created, updated, skipped := 0, 0, 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range sources {
			var existing model.TenderIntelSource
			err := tx.Where("source_key = ?", item.SourceKey).First(&existing).Error
			switch {
			case err == nil:
				if !overwrite {
					skipped++
					continue
				}
				// 已有源只更新管理员可维护的字段，不覆盖采集健康度与游标
				updates := map[string]interface{}{
					"name":           item.Name,
					"homepage_url":   item.HomepageURL,
					"list_url":       item.ListURL,
					"category":       item.Category,
					"region":         item.Region,
					"industry_hint":  item.IndustryHint,
					"discovery_mode": item.DiscoveryMode,
					"needs_browser":  item.NeedsBrowser,
					"enabled":        item.Enabled,
					"priority":       item.Priority,
					"params":         item.Params,
				}
				if err := tx.Model(&model.TenderIntelSource{}).
					Where("source_key = ?", item.SourceKey).
					Updates(updates).Error; err != nil {
					return err
				}
				updated++
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error; err != nil {
					return err
				}
				created++
			default:
				return err
			}
		}
		return nil
	})
	return created, updated, skipped, err
}

// SaveSourceHealth 记录采集源执行结果与健康度。
func (r *Repository) SaveSourceHealth(ctx context.Context, sourceKey, cursor string, runAt interface{}, success bool, errMsg string) error {
	updates := map[string]interface{}{
		"last_run_at": runAt,
	}
	if success {
		updates["last_success_at"] = runAt
		updates["last_error"] = ""
		updates["consecutive_failures"] = 0
		if cursor != "" {
			updates["cursor"] = cursor
		}
	} else {
		updates["last_error"] = errMsg
		updates["consecutive_failures"] = gorm.Expr("consecutive_failures + 1")
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelSource{}).
		Where("source_key = ?", sourceKey).
		Updates(updates).Error
}

// ── 自动采集任务配置（单行）──────────────────────────────────────

// GetCollectSchedule 返回自动采集任务配置；无记录时返回默认值（每天 06:00 启用）。
func (r *Repository) GetCollectSchedule(ctx context.Context) (*model.TenderIntelCollectSchedule, error) {
	var item model.TenderIntelCollectSchedule
	err := r.db.WithContext(ctx).Order("id ASC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.TenderIntelCollectSchedule{
			ID:       1,
			CronExpr: DefaultCollectCron,
			Enabled:  1,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.CronExpr) == "" {
		item.CronExpr = DefaultCollectCron
	}
	return &item, nil
}

// SaveCollectSchedule 保存自动采集任务配置（固定写 id=1 的行）。
func (r *Repository) SaveCollectSchedule(ctx context.Context, cronExpr string, enabled bool, updatedBy int64) error {
	enabledValue := int32(0)
	if enabled {
		enabledValue = 1
	}
	item := &model.TenderIntelCollectSchedule{
		ID:        1,
		CronExpr:  strings.TrimSpace(cronExpr),
		Enabled:   enabledValue,
		UpdatedBy: updatedBy,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"cron_expr", "enabled", "updated_by", "updated_at"}),
		}).
		Create(item).Error
}

// ── 行业枚举 ────────────────────────────────────────────────────

// ListIndustries 返回启用的行业枚举，按 sort 升序。
func (r *Repository) ListIndustries(ctx context.Context) ([]*model.TenderIntelIndustry, error) {
	var items []*model.TenderIntelIndustry
	err := r.db.WithContext(ctx).
		Where("enabled = ?", 1).
		Order("sort ASC, id ASC").
		Find(&items).Error
	return items, err
}

// ReplaceNoticeIndustries 覆盖写入公告的行业标签。
func (r *Repository) ReplaceNoticeIndustries(ctx context.Context, noticeID int64, codes []string, weights map[string]int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("notice_id = ?", noticeID).
			Delete(&model.TenderIntelNoticeIndustry{}).Error; err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		rows := make([]*model.TenderIntelNoticeIndustry, 0, len(codes))
		for _, code := range codes {
			rows = append(rows, &model.TenderIntelNoticeIndustry{
				NoticeID:     noticeID,
				IndustryCode: code,
				Weight:       int32(weights[code]),
			})
		}
		return tx.Create(&rows).Error
	})
}

// ListNoticeIndustryMap 批量查询公告的行业标签，返回 notice_id -> codes。
func (r *Repository) ListNoticeIndustryMap(ctx context.Context, noticeIDs []int64) (map[int64][]string, error) {
	out := make(map[int64][]string, len(noticeIDs))
	if len(noticeIDs) == 0 {
		return out, nil
	}
	var rows []*model.TenderIntelNoticeIndustry
	if err := r.db.WithContext(ctx).
		Where("notice_id IN ?", noticeIDs).
		Order("weight DESC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.NoticeID] = append(out[row.NoticeID], row.IndustryCode)
	}
	return out, nil
}
