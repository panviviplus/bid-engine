package home

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"bid-engine/lib/common/storage"
	llmrepo "bid-engine/pkg/repo/llm"
)

const (
	ModuleBidAnalysis   = "bid_analysis"
	ModuleBidGeneration = "bid_generation"
	ModuleBidReview     = "bid_review"
	ModuleMaterial      = "material"
)

type ModuleStat struct {
	Count          int64            `json:"count"`
	AttentionCount int64            `json:"attention_count"`
	Breakdown      map[string]int64 `json:"breakdown,omitempty"`
}

type WorkItem struct {
	Module        string    `json:"module"`
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	Stage         string    `json:"stage"`
	Progress      int32     `json:"progress"`
	UpdatedAt     time.Time `json:"updated_at"`
	WarningCount  int32     `json:"warning_count"`
	ErrorCount    int32     `json:"error_count"`
	AttentionType string    `json:"attention_type,omitempty"`
	Priority      int       `json:"-"`
}

type LLMConfigStatus struct {
	Modules        map[string]bool `json:"modules"`
	MissingModules []string        `json:"missing_modules"`
}

type Repository struct {
	db *gorm.DB
}

func New() *Repository {
	return &Repository{db: storage.GetDB()}
}

func NewWithDB(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Stats(ctx context.Context, userID int64, start, end time.Time) (map[string]ModuleStat, error) {
	type countRow struct {
		Count          int64
		AttentionCount int64
	}

	queries := []struct {
		key       string
		table     string
		baseWhere string
		attention string
	}{
		{
			key:       ModuleBidAnalysis,
			table:     "bid_analysis_v3_project",
			baseWhere: "user_id = ? AND is_internal = 0",
			attention: "status IN ('running','paused','failed','succeeded_with_warnings')",
		},
		{
			key:       ModuleBidGeneration,
			table:     "bid_gen_project",
			baseWhere: "user_id = ? AND deleted_at IS NULL",
			attention: "status IN ('parsing','generating','outline_review','failed')",
		},
		{
			key:       ModuleBidReview,
			table:     "bid_review_project",
			baseWhere: "user_id = ?",
			attention: "status IN ('running','failed') OR (status = 'succeed' AND (warning_items > 0 OR error_items > 0))",
		},
	}

	modules := make(map[string]ModuleStat, 4)
	for _, query := range queries {
		var row countRow
		selectSQL := fmt.Sprintf(
			"COALESCE(SUM(CASE WHEN created_at >= ? AND created_at <= ? THEN 1 ELSE 0 END), 0) AS count, COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS attention_count",
			query.attention,
		)
		if err := r.db.WithContext(ctx).Table(query.table).
			Select(selectSQL, start, end).
			Where(query.baseWhere, userID).
			Scan(&row).Error; err != nil {
			return nil, fmt.Errorf("count %s: %w", query.key, err)
		}
		modules[query.key] = ModuleStat{Count: row.Count, AttentionCount: row.AttentionCount}
	}

	breakdown := map[string]int64{
		"qualification": 0,
		"performance":   0,
		"template":      0,
	}
	materialTables := map[string]string{
		"qualification": "material_qualification",
		"performance":   "material_performance",
		"template":      "material_template",
	}
	var materialTotal int64
	for key, table := range materialTables {
		var count int64
		if err := r.db.WithContext(ctx).Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count material %s: %w", key, err)
		}
		breakdown[key] = count
		materialTotal += count
	}
	modules[ModuleMaterial] = ModuleStat{Count: materialTotal, Breakdown: breakdown}

	return modules, nil
}

func (r *Repository) RecentWork(ctx context.Context, userID int64, limit int) ([]WorkItem, error) {
	query := `
		SELECT * FROM (
			SELECT 'bid_analysis' AS module, id, name, status, stage, progress, updated_at,
				warning_count, 0 AS error_count, '' AS attention_type, 0 AS priority
			FROM bid_analysis_v3_project
			WHERE user_id = ? AND is_internal = 0
			UNION ALL
			SELECT 'bid_generation' AS module, id, name, status, stage, progress, updated_at,
				0 AS warning_count, 0 AS error_count, '' AS attention_type, 0 AS priority
			FROM bid_gen_project
			WHERE user_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT 'bid_review' AS module, id, name, status, stage, progress, updated_at,
				warning_items AS warning_count, error_items AS error_count, '' AS attention_type, 0 AS priority
			FROM bid_review_project
			WHERE user_id = ?
		) AS recent_work
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`

	items := make([]WorkItem, 0, limit)
	if err := r.db.WithContext(ctx).Raw(query, userID, userID, userID, limit).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list recent work: %w", err)
	}
	return items, nil
}

func (r *Repository) Attention(ctx context.Context, userID int64, limit int) ([]WorkItem, error) {
	query := `
		SELECT * FROM (
			SELECT 'bid_analysis' AS module, id, name, status, stage, progress, updated_at,
				warning_count, 0 AS error_count,
				CASE
					WHEN status = 'failed' THEN 'failed'
					WHEN status = 'succeeded_with_warnings' THEN 'risk'
					WHEN status = 'paused' THEN 'action_required'
					ELSE 'running'
				END AS attention_type,
				CASE WHEN status = 'failed' THEN 4 WHEN status = 'succeeded_with_warnings' THEN 3 WHEN status = 'paused' THEN 2 ELSE 1 END AS priority
			FROM bid_analysis_v3_project
			WHERE user_id = ? AND is_internal = 0 AND status IN ('running','paused','failed','succeeded_with_warnings')
			UNION ALL
			SELECT 'bid_generation' AS module, id, name, status, stage, progress, updated_at,
				0 AS warning_count, 0 AS error_count,
				CASE WHEN status = 'failed' THEN 'failed' WHEN status = 'outline_review' THEN 'action_required' ELSE 'running' END AS attention_type,
				CASE WHEN status = 'failed' THEN 4 WHEN status = 'outline_review' THEN 2 ELSE 1 END AS priority
			FROM bid_gen_project
			WHERE user_id = ? AND deleted_at IS NULL AND status IN ('parsing','generating','outline_review','failed')
			UNION ALL
			SELECT 'bid_review' AS module, id, name, status, stage, progress, updated_at,
				warning_items AS warning_count, error_items AS error_count,
				CASE WHEN status = 'failed' THEN 'failed' WHEN warning_items > 0 OR error_items > 0 THEN 'risk' ELSE 'running' END AS attention_type,
				CASE WHEN status = 'failed' THEN 4 WHEN warning_items > 0 OR error_items > 0 THEN 3 ELSE 1 END AS priority
			FROM bid_review_project
			WHERE user_id = ? AND (status IN ('running','failed') OR (status = 'succeed' AND (warning_items > 0 OR error_items > 0)))
		) AS attention_work
		ORDER BY priority DESC, updated_at DESC, id DESC
		LIMIT ?`

	items := make([]WorkItem, 0, limit)
	if err := r.db.WithContext(ctx).Raw(query, userID, userID, userID, limit).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list attention work: %w", err)
	}
	return items, nil
}

func (r *Repository) LLMConfigStatus(ctx context.Context, userID int64) (LLMConfigStatus, error) {
	var configured []string
	if err := r.db.WithContext(ctx).Table("user_llm_config").
		Where("user_id = ? AND TRIM(api_key) <> ''", userID).
		Pluck("module", &configured).Error; err != nil {
		return LLMConfigStatus{}, fmt.Errorf("list llm config: %w", err)
	}

	configuredSet := make(map[string]bool, len(configured))
	for _, module := range configured {
		configuredSet[strings.TrimSpace(module)] = true
	}
	allConfigured := configuredSet[llmrepo.ModuleAll]
	modules := make(map[string]bool, len(llmrepo.BusinessModules))
	missing := make([]string, 0, len(llmrepo.BusinessModules))
	for _, module := range llmrepo.BusinessModules {
		ready := allConfigured || configuredSet[module]
		modules[module] = ready
		if !ready {
			missing = append(missing, module)
		}
	}
	return LLMConfigStatus{Modules: modules, MissingModules: missing}, nil
}
