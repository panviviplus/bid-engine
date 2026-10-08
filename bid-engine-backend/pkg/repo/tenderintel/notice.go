package tenderintel

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
)

// NoticeFilter 情报大厅列表筛选条件。
type NoticeFilter struct {
	Keyword      string
	Industries   []string
	NoticeTypes  []string
	Regions      []string
	SourceKeys   []string
	BudgetMin    *float64
	BudgetMax    *float64
	PublishFrom  *time.Time
	PublishTo    *time.Time
	CollectFrom  *time.Time // 采集时间下限（first_seen_at）
	TagStatus    string
	OnlyValid    bool     // 仅返回 status=normal（情报大厅）
	Statuses     []string // 显式状态列表（情报管理）；非空时覆盖 OnlyValid
	Origins      []string // 入库来源：collect / manual
	ImportBatch  string   // 手工录入/导入批次号
	PinnedOnly   bool     // 仅返回置顶公告
	FavoriteIDs  []int64  // 非 nil 时仅返回这些公告（用于“只看收藏”）
	PageNum      int
	PageSize     int
	OrderByField string
}

func (f *NoticeFilter) normalize() {
	if f.PageNum <= 0 {
		f.PageNum = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
}

func (r *Repository) applyNoticeFilter(q *gorm.DB, f *NoticeFilter) *gorm.DB {
	q = q.Model(&model.TenderIntelNotice{})
	// 状态：情报管理传入显式状态列表（含隐藏/下架），情报大厅只看 normal。
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	} else if f.OnlyValid {
		q = q.Where("status = ?", "normal")
	}
	if f.TagStatus != "" {
		q = q.Where("tag_status = ?", f.TagStatus)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("title LIKE ? OR body_text LIKE ?", like, like)
	}
	if len(f.NoticeTypes) > 0 {
		q = q.Where("notice_type IN ?", f.NoticeTypes)
	}
	if len(f.SourceKeys) > 0 {
		q = q.Where("source_key IN ?", f.SourceKeys)
	}
	if len(f.Origins) > 0 {
		q = q.Where("origin IN ?", f.Origins)
	}
	if f.ImportBatch != "" {
		q = q.Where("import_batch = ?", f.ImportBatch)
	}
	if f.PinnedOnly {
		q = q.Where("pinned = 1")
	}
	if len(f.Regions) > 0 {
		q = q.Where("region_province IN ? OR region_city IN ?", f.Regions, f.Regions)
	}
	if len(f.Industries) > 0 {
		sub := r.db.Model(&model.TenderIntelNoticeIndustry{}).
			Select("notice_id").
			Where("industry_code IN ?", f.Industries)
		q = q.Where("id IN (?)", sub)
	}
	if f.BudgetMin != nil {
		q = q.Where("budget_amount IS NOT NULL AND budget_amount >= ?", *f.BudgetMin)
	}
	if f.BudgetMax != nil {
		q = q.Where("budget_amount IS NOT NULL AND budget_amount <= ?", *f.BudgetMax)
	}
	if f.PublishFrom != nil {
		q = q.Where("publish_date >= ?", *f.PublishFrom)
	}
	if f.PublishTo != nil {
		q = q.Where("publish_date <= ?", *f.PublishTo)
	}
	if f.CollectFrom != nil {
		q = q.Where("first_seen_at >= ?", *f.CollectFrom)
	}
	if f.FavoriteIDs != nil {
		if len(f.FavoriteIDs) == 0 {
			// 空收藏列表：强制返回空结果
			q = q.Where("1 = 0")
		} else {
			q = q.Where("id IN ?", f.FavoriteIDs)
		}
	}
	return q
}

// PageNotices 分页查询公告。
func (r *Repository) PageNotices(ctx context.Context, f *NoticeFilter) ([]*model.TenderIntelNotice, int64, error) {
	f.normalize()
	var total int64
	countQ := r.applyNoticeFilter(r.db.WithContext(ctx), f)
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.TenderIntelNotice
	listQ := r.applyNoticeFilter(r.db.WithContext(ctx), f)
	// 置顶优先：置顶公告始终排在列表最前，其余按发布时间倒序。
	order := "pinned DESC, publish_date DESC, id DESC"
	switch f.OrderByField {
	case "created_at":
		order = "pinned DESC, created_at DESC, id DESC"
	case "budget_desc":
		order = "pinned DESC, budget_amount IS NULL, budget_amount DESC, id DESC"
	case "first_seen":
		order = "pinned DESC, first_seen_at DESC, id DESC"
	}
	err := listQ.
		Order(order).
		Offset((f.PageNum - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetNotice 按主键查询公告。
func (r *Repository) GetNotice(ctx context.Context, id int64) (*model.TenderIntelNotice, error) {
	var item model.TenderIntelNotice
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetNoticeByURLHash 按 url_hash 查询公告。
func (r *Repository) GetNoticeByURLHash(ctx context.Context, urlHash string) (*model.TenderIntelNotice, error) {
	var item model.TenderIntelNotice
	if err := r.db.WithContext(ctx).Where("url_hash = ?", urlHash).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateNotice 新增公告。
func (r *Repository) CreateNotice(ctx context.Context, item *model.TenderIntelNotice) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// NoticeContentRefresh 描述一次重新采集得到的正文与抽取元信息。
type NoticeContentRefresh struct {
	BodyHTML          string
	BodyMarkdown      string
	BodyText          string
	ContentHash       string
	FetchStrategy     string
	ExtractConfidence float64
	ExtractWarnings   string
	SeenAt            time.Time
}

// RefreshNoticeContent 刷新重复公告正文；纯文本变化时同步使打标与 AI 解读失效。
func (r *Repository) RefreshNoticeContent(ctx context.Context, noticeID int64, refresh NoticeContentRefresh) (bool, error) {
	contentChanged := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.TenderIntelNotice
		if err := tx.First(&existing, noticeID).Error; err != nil {
			return err
		}

		contentChanged = existing.ContentHash != refresh.ContentHash
		seenAt := refresh.SeenAt
		if seenAt.IsZero() {
			seenAt = time.Now()
		}
		updates := map[string]interface{}{
			"last_seen_at":       seenAt,
			"fetch_strategy":     refresh.FetchStrategy,
			"extract_confidence": refresh.ExtractConfidence,
			"extract_warnings":   refresh.ExtractWarnings,
		}
		if contentChanged {
			updates["body_html"] = strings.TrimSpace(refresh.BodyHTML)
			updates["body_markdown"] = refresh.BodyMarkdown
			updates["body_text"] = refresh.BodyText
			updates["content_hash"] = refresh.ContentHash
			updates["tag_status"] = "pending"
		} else if strings.TrimSpace(refresh.BodyHTML) != "" {
			updates["body_html"] = strings.TrimSpace(refresh.BodyHTML)
		}

		if err := tx.Model(&model.TenderIntelNotice{}).
			Where("id = ?", noticeID).
			Updates(updates).Error; err != nil {
			return err
		}
		if contentChanged {
			if err := tx.Where("notice_id = ?", noticeID).
				Delete(&model.TenderIntelNoticeInsight{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return contentChanged, err
}

// ListNoticeHTMLBackfillBatch 返回需要补齐语义 HTML 的自动采集公告。
func (r *Repository) ListNoticeHTMLBackfillBatch(ctx context.Context, afterID int64, sourceKey string, limit int, includeExisting bool) ([]*model.TenderIntelNotice, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Where("id > ?", afterID).
		Where("origin = ?", "collect").
		Where("url LIKE ? OR url LIKE ?", "http://%", "https://%")
	if !includeExisting {
		query = query.Where("body_html IS NULL OR body_html = ''")
	}
	if strings.TrimSpace(sourceKey) != "" {
		query = query.Where("source_key = ?", strings.TrimSpace(sourceKey))
	}
	var notices []*model.TenderIntelNotice
	err := query.Order("id ASC").Limit(limit).Find(&notices).Error
	return notices, err
}

// UpdateNoticeTagResult 写回打标结果。
func (r *Repository) UpdateNoticeTagResult(ctx context.Context, id int64, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ListPendingTagNotices 返回待打标的公告（按发布时间倒序，限制批量大小）。
func (r *Repository) ListPendingTagNotices(ctx context.Context, limit int) ([]*model.TenderIntelNotice, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []*model.TenderIntelNotice
	err := r.db.WithContext(ctx).
		Where("tag_status = ?", "pending").
		Order("id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// CountTodayNew 统计今日新增公告数（按首次发现时间）。
func (r *Repository) CountTodayNew(ctx context.Context) (int64, error) {
	start := time.Now().Truncate(24 * time.Hour)
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("first_seen_at >= ?", start).
		Count(&total).Error
	return total, err
}

// ListRegions 返回出现过的省级地区列表（用于筛选项）。
func (r *Repository) ListRegions(ctx context.Context) ([]string, error) {
	var out []string
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Distinct().
		Where("region_province <> ''").
		Order("region_province ASC").
		Pluck("region_province", &out).Error
	return out, err
}

// ListNoticeTypes 返回出现过的公告类型（用于筛选项）。
func (r *Repository) ListNoticeTypes(ctx context.Context) ([]string, error) {
	var out []string
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Distinct().
		Order("notice_type ASC").
		Pluck("notice_type", &out).Error
	return out, err
}

// DeleteNoticesBefore 清理保留期之外的公告，返回删除条数。
func (r *Repository) DeleteNoticesBefore(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("publish_date IS NOT NULL AND publish_date < ?", before).
		Delete(&model.TenderIntelNotice{})
	return res.RowsAffected, res.Error
}

// ── 情报管理（超管） ────────────────────────────────────────────

// UpdateNoticesByIDs 批量更新公告字段（状态/置顶/备注等），返回受影响条数。
func (r *Repository) UpdateNoticesByIDs(ctx context.Context, ids []int64, updates map[string]interface{}) (int64, error) {
	if len(ids) == 0 || len(updates) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("id IN ?", ids).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// UpdateNoticeFields 更新单条公告字段。
func (r *Repository) UpdateNoticeFields(ctx context.Context, id int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteNotices 删除公告及其派生的行业标签、提醒、收藏与 AI 解读缓存。
//
// 全库无外键，关联数据必须在同一个事务里显式清理，避免留下孤儿行。
func (r *Repository) DeleteNotices(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("notice_id IN ?", ids).Delete(&model.TenderIntelNoticeIndustry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("notice_id IN ?", ids).Delete(&model.TenderIntelNoticeInsight{}).Error; err != nil {
			return err
		}
		if err := tx.Where("notice_id IN ?", ids).Delete(&model.TenderIntelAlert{}).Error; err != nil {
			return err
		}
		if err := tx.Where("notice_id IN ?", ids).Delete(&model.TenderIntelFavorite{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", ids).Delete(&model.TenderIntelNotice{})
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

// CountNoticesByStatus 按状态统计公告数（情报管理页概览）。
func (r *Repository) CountNoticesByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Select("status, COUNT(*) AS total").
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

// CountPinnedNotices 统计置顶公告数。
func (r *Repository) CountPinnedNotices(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("pinned = 1").
		Count(&total).Error
	return total, err
}

// CountManualNotices 统计手工录入的公告数。
func (r *Repository) CountManualNotices(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.TenderIntelNotice{}).
		Where("origin = ?", "manual").
		Count(&total).Error
	return total, err
}

// ── AI 解读缓存 ─────────────────────────────────────────────────

// GetInsight 读取公告的 AI 解读缓存。
func (r *Repository) GetInsight(ctx context.Context, noticeID int64) (*model.TenderIntelNoticeInsight, error) {
	var item model.TenderIntelNoticeInsight
	if err := r.db.WithContext(ctx).Where("notice_id = ?", noticeID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// SaveInsight 覆盖写入公告的 AI 解读缓存。
func (r *Repository) SaveInsight(ctx context.Context, item *model.TenderIntelNoticeInsight) error {
	var existing model.TenderIntelNoticeInsight
	err := r.db.WithContext(ctx).Where("notice_id = ?", item.NoticeID).First(&existing).Error
	if err == nil {
		item.ID = existing.ID
		return r.db.WithContext(ctx).Save(item).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.WithContext(ctx).Create(item).Error
}
