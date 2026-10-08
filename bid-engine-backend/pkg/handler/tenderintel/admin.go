package tenderintel

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// sourceView 采集源视图（含健康度）。
type sourceView struct {
	SourceKey           string `json:"source_key"`
	Name                string `json:"name"`
	HomepageURL         string `json:"homepage_url"`
	ListURL             string `json:"list_url"`
	Category            string `json:"category"`
	Region              string `json:"region"`
	DiscoveryMode       string `json:"discovery_mode"`
	NeedsBrowser        bool   `json:"needs_browser"`
	Enabled             bool   `json:"enabled"`
	Priority            int32  `json:"priority"`
	Cursor              string `json:"cursor"`
	LastRunAt           string `json:"last_run_at"`
	LastSuccessAt       string `json:"last_success_at"`
	LastError           string `json:"last_error"`
	ConsecutiveFailures int32  `json:"consecutive_failures"`
}

// RunSummary 采集批次概览：管理页折叠态直接展示，避免前端只统计当前分页而口径失真。
type RunSummary struct {
	Total   int64 `json:"total"`
	Running int64 `json:"running"`
	Success int64 `json:"success"`
	Partial int64 `json:"partial"`
	Failed  int64 `json:"failed"`
}

// ListSources 采集源列表（超管）。
func (s *svcImpl) ListSources(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	items, err := s.repo.ListSources(c.Request.Context())
	if err != nil {
		logger.Warnw("查询采集源失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询采集源失败", nil)
		return
	}
	out := make([]sourceView, 0, len(items))
	for _, item := range items {
		out = append(out, toSourceView(item))
	}
	handler.SendOKResp(c, out)
}

// ListRuns 采集批次列表（超管）。
func (s *svcImpl) ListRuns(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	pageNum := queryInt(c, "pageNum", 1)
	pageSize := queryInt(c, "pageSize", 20)
	items, total, err := s.repo.ListRuns(c.Request.Context(), pageNum, pageSize)
	if err != nil {
		logger.Warnw("查询采集批次失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询采集批次失败", nil)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, runToMap(item))
	}
	summary := RunSummary{}
	statusCounts, err := s.repo.CountRunsByStatus(c.Request.Context())
	if err != nil {
		// 概览统计失败不阻断列表本身，前端会退化为只展示总数
		logger.Warnw("统计采集批次概览失败", "err", err)
		summary.Total = total
	} else {
		summary.Total = statusCounts["total"]
		summary.Running = statusCounts["running"]
		summary.Success = statusCounts["success"]
		summary.Partial = statusCounts["partial"]
		summary.Failed = statusCounts["failed"]
	}
	handler.SendPageRespV2Extra(c, out, total, pageNum, pageSize, map[string]any{
		"summary": summary,
	})
}

// GetRunDetail 采集批次明细（超管）。
func (s *svcImpl) GetRunDetail(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	runID := c.Param("runId")
	run, err := s.repo.GetRun(c.Request.Context(), runID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "批次不存在", nil)
		return
	}
	details, err := s.repo.ListRunSources(c.Request.Context(), runID)
	if err != nil {
		logger.Warnw("查询批次明细失败", "run_id", runID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询批次明细失败", nil)
		return
	}
	sources := make([]map[string]any, 0, len(details))
	for _, item := range details {
		sources = append(sources, map[string]any{
			"source_key":    item.SourceKey,
			"source_name":   item.SourceName,
			"status":        item.Status,
			"cursor_before": item.CursorBefore,
			"cursor_after":  item.CursorAfter,
			"discovered":    item.Discovered,
			"extracted":     item.Extracted,
			"inserted":      item.Inserted,
			"skipped":       item.Skipped,
			"duration_ms":   item.DurationMs,
			"error":         item.Error,
		})
	}
	handler.SendOKResp(c, map[string]any{
		"run":     runToMap(run),
		"sources": sources,
	})
}

// TriggerCollect 手动触发一轮采集（超管）。
func (s *svcImpl) TriggerCollect(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	var req struct {
		SourceKeys []string `json:"source_keys"`
	}
	_ = c.ShouldBindJSON(&req)

	runID, err := s.StartCollectRound(c.Request.Context(), "manual", req.SourceKeys)
	if err != nil {
		logger.Warnw("手动触发采集失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "触发采集失败："+err.Error(), nil)
		return
	}
	// 回带范围信息，前端提示可直接说明本次覆盖了多少源、是全量还是指定源
	payload := map[string]any{"run_id": runID, "trigger_type": "manual"}
	if run, err := s.repo.GetRun(c.Request.Context(), runID); err == nil && run != nil {
		payload["scope"] = run.Scope
		payload["source_total"] = run.SourceTotal
		payload["source_keys"] = s.repo.GetRunSourceKeys(run)
	}
	handler.SendOKResp(c, payload)
}

func requireSuperAdmin(c *gin.Context) bool {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return false
	}
	if !entity.IsSuperAdmin(c) {
		handler.SendForbiddenResp(c)
		return false
	}
	return true
}

func toSourceView(item *model.TenderIntelSource) sourceView {
	return sourceView{
		SourceKey:           item.SourceKey,
		Name:                item.Name,
		HomepageURL:         item.HomepageURL,
		ListURL:             item.ListURL,
		Category:            item.Category,
		Region:              item.Region,
		DiscoveryMode:       item.DiscoveryMode,
		NeedsBrowser:        item.NeedsBrowser == 1,
		Enabled:             item.Enabled == 1,
		Priority:            item.Priority,
		Cursor:              item.Cursor,
		LastRunAt:           formatTime(item.LastRunAt),
		LastSuccessAt:       formatTime(item.LastSuccessAt),
		LastError:           item.LastError,
		ConsecutiveFailures: item.ConsecutiveFailures,
	}
}

func runToMap(item *model.TenderIntelCollectRun) map[string]any {
	sourceKeys := []string{}
	if item.SourceKeys != "" {
		_ = json.Unmarshal([]byte(item.SourceKeys), &sourceKeys)
	}
	scopeLabel := "全部启用源"
	if item.Scope == "single" && len(sourceKeys) > 0 {
		scopeLabel = "指定源：" + truncateRunes(strings.Join(sourceKeys, "、"), 120)
	}
	return map[string]any{
		"run_id":           item.RunID,
		"trigger_type":     item.TriggerType,
		"scope":            item.Scope,
		"scope_label":      scopeLabel,
		"source_keys":      sourceKeys,
		"retry_of":         item.RetryOf,
		"status":           item.Status,
		"started_at":       formatTime(item.StartedAt),
		"finished_at":      formatTimePtr(item.FinishedAt),
		"source_total":     item.SourceTotal,
		"source_success":   item.SourceSuccess,
		"source_failed":    item.SourceFailed,
		"discovered_count": item.DiscoveredCount,
		"extracted_count":  item.ExtractedCount,
		"inserted_count":   item.InsertedCount,
		"duplicate_count":  item.DuplicateCount,
		"enriched_count":   item.EnrichedCount,
		"matched_count":    item.MatchedCount,
		"error_summary":    item.ErrorSummary,
	}
}
