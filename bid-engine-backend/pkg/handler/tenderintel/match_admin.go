package tenderintel

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/taskqueue"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

// maxRescanNotices 单次手动补扫允许覆盖的情报上限，避免一次把队列压满。
const maxRescanNotices = 20000

// matchTaskView 匹配任务视图。
type matchTaskView struct {
	TaskNo             string `json:"task_no"`
	TaskType           string `json:"task_type"`
	TaskTypeName       string `json:"task_type_name"`
	ScopeKind          string `json:"scope_kind"`
	ScopeRef           string `json:"scope_ref"`
	RangeFrom          string `json:"range_from"`
	RangeTo            string `json:"range_to"`
	NoticeTotal        int32  `json:"notice_total"`
	ScannedCount       int32  `json:"scanned_count"`
	MatchedNoticeCount int32  `json:"matched_notice_count"`
	AlertCount         int32  `json:"alert_count"`
	Status             string `json:"status"`
	Priority           string `json:"priority"`
	CancelRequested    bool   `json:"cancel_requested"`
	Attempts           int32  `json:"attempts"`
	RetryOf            string `json:"retry_of"`
	SubscriptionID     int64  `json:"subscription_id"`
	UserID             int64  `json:"user_id"`
	SubscriptionName   string `json:"subscription_name"`
	UserName           string `json:"user_name"`
	UserMobile         string `json:"user_mobile"`
	LastError          string `json:"last_error"`
	StartedAt          string `json:"started_at"`
	FinishedAt         string `json:"finished_at"`
	CreatedAt          string `json:"created_at"`
	// QueueOrder 排队中的执行顺位（1 = 下一个执行）；0 表示不在队列或无法确定
	QueueOrder int64 `json:"queue_order"`
}

// ListMatchTasks 匹配任务列表。
func (s *svcImpl) ListMatchTasks(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	items, total, err := s.repo.ListMatchTasks(c.Request.Context(), intelRepo.MatchTaskFilter{
		Status:   strings.TrimSpace(c.Query("status")),
		TaskType: strings.TrimSpace(c.Query("task_type")),
		PageNum:  queryInt(c, "pageNum", 1),
		PageSize: queryInt(c, "pageSize", 20),
	})
	if err != nil {
		logger.Warnw("查询匹配任务失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询匹配任务失败", nil)
		return
	}
	out := make([]matchTaskView, 0, len(items))
	lookups := s.loadMatchLookups(c.Request.Context(), items)
	for _, item := range items {
		out = append(out, s.toMatchTaskView(c.Request.Context(), item, lookups))
	}
	handler.SendPageRespV2(c, out, total, queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
}

// GetMatchTask 匹配任务详情。
func (s *svcImpl) GetMatchTask(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	item, err := s.repo.GetMatchTask(c.Request.Context(), strings.TrimSpace(c.Param("taskNo")))
	if err != nil || item == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "匹配任务不存在", nil)
		return
	}
	lookups := s.loadMatchLookups(c.Request.Context(), []*model.TenderIntelMatchTask{item})
	handler.SendOKResp(c, s.toMatchTaskView(c.Request.Context(), item, lookups))
}

// MatchOverview 订阅匹配概览。
func (s *svcImpl) MatchOverview(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	counts, err := s.repo.CountMatchTasksByStatus(ctx)
	if err != nil {
		logger.Warnw("统计匹配任务失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "统计匹配任务失败", nil)
		return
	}
	dayStart := NowFunc().Truncate(24 * time.Hour)
	todayAlerts, err := s.repo.CountMatchAlertsSince(ctx, dayStart)
	if err != nil {
		logger.Warnw("统计今日匹配提醒失败", "err", err)
		todayAlerts = 0
	}
	// 排队中 + 执行中给明细，运维要能直接看到“谁在跑、谁在等”
	active := make([]matchTaskView, 0, 10)
	activeItems := make([]*model.TenderIntelMatchTask, 0, 20)
	for _, status := range []string{"running", "pending"} {
		items, _, err := s.repo.ListMatchTasks(ctx, intelRepo.MatchTaskFilter{Status: status, PageSize: 10})
		if err != nil {
			continue
		}
		activeItems = append(activeItems, items...)
	}
	activeLookups := s.loadMatchLookups(ctx, activeItems)
	for _, item := range activeItems {
		active = append(active, s.toMatchTaskView(ctx, item, activeLookups))
	}
	recent, _, err := s.repo.ListMatchTasks(ctx, intelRepo.MatchTaskFilter{PageSize: 10})
	if err != nil {
		recent = nil
	}
	recentViews := make([]matchTaskView, 0, len(recent))
	recentLookups := s.loadMatchLookups(ctx, recent)
	for _, item := range recent {
		recentViews = append(recentViews, s.toMatchTaskView(ctx, item, recentLookups))
	}
	handler.SendOKResp(c, map[string]any{
		"counts":       counts,
		"today_alerts": todayAlerts,
		"active":       active,
		"recent":       recentViews,
	})
}

// SetMatchTaskPriority 调整排队中任务的优先级。
func (s *svcImpl) SetMatchTaskPriority(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	taskNo := strings.TrimSpace(c.Param("taskNo"))
	var req struct {
		Priority string `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	priority := strings.TrimSpace(req.Priority)
	if priority != "high" && priority != "normal" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "优先级只支持 normal 或 high", nil)
		return
	}
	item, err := s.repo.GetMatchTask(ctx, taskNo)
	if err != nil || item == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "匹配任务不存在", nil)
		return
	}
	if item.Status != "pending" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "仅排队中的任务可以调整优先级，当前状态："+MatchStatusName(item.Status), nil)
		return
	}
	if item.QueuedTaskID == "" {
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "任务缺少队列标识，请先重新入队", nil)
		return
	}
	if err := s.queue.ReprioritizeTask(ctx, item.QueuedTaskID, taskqueue.TaskPriority(priority)); err != nil {
		logger.Warnw("调整匹配任务优先级失败", "task_no", taskNo, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "调整优先级失败："+err.Error(), nil)
		return
	}
	if err := s.repo.UpdateMatchTask(ctx, taskNo, map[string]interface{}{"priority": priority}); err != nil {
		logger.Warnw("回写匹配任务优先级失败", "task_no", taskNo, "err", err)
	}
	handler.SendOKResp(c, map[string]any{"task_no": taskNo, "priority": priority})
}

// CancelMatchTask 取消匹配任务。
//
// pending 直接出队；running 走协作式取消，worker 在批次边界退出。
// 取消是终态：该任务覆盖的情报不会被自动重匹配。
func (s *svcImpl) CancelMatchTask(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	taskNo := strings.TrimSpace(c.Param("taskNo"))
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "管理员取消"
	}
	item, err := s.repo.GetMatchTask(ctx, taskNo)
	if err != nil || item == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "匹配任务不存在", nil)
		return
	}
	switch item.Status {
	case "success", "failed", "cancelled":
		handler.SendNormalResp(c, entity.ErrCodeParam, "任务已结束，无需取消", nil)
		return
	case "pending":
		if item.QueuedTaskID != "" {
			// 队列元数据可能已过期，取消失败不阻断账本收尾
			if err := s.queue.CancelTask(ctx, item.QueuedTaskID); err != nil {
				logger.Warnw("取消队列任务失败", "task_no", taskNo, "err", err)
			}
		}
		if _, err := s.repo.UpdateMatchTaskIfStatus(ctx, taskNo, []string{"pending"}, map[string]interface{}{
			"status":      "cancelled",
			"last_error":  truncateRunes(reason, 500),
			"finished_at": NowFunc(),
		}); err != nil {
			logger.Warnw("取消匹配任务失败", "task_no", taskNo, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeDBWrite, "取消失败，请稍后重试", nil)
			return
		}
	default: // running：请求取消，由 worker 在批次边界收尾
		updated, err := s.repo.UpdateMatchTaskIfStatus(ctx, taskNo, []string{"running"}, map[string]interface{}{
			"cancel_requested": 1,
			"last_error":       truncateRunes(reason, 500),
		})
		if err != nil {
			logger.Warnw("请求取消匹配任务失败", "task_no", taskNo, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeDBWrite, "请求取消失败，请稍后重试", nil)
			return
		}
		if !updated {
			handler.SendNormalResp(c, entity.ErrCodeParam, "任务状态已变化，请刷新后重试", nil)
			return
		}
	}
	s.syncSubscriptionBackfillCancelled(ctx, item)
	handler.SendOKResp(c, map[string]any{"task_no": taskNo, "status": "cancelled"})
}

// RetryMatchTask 用同一份固定情报范围重新执行一次。
//
// 执行成功、失败或已取消的任务都可以重跑，用于在匹配规则更新后按原范围补算；
// 失败任务显示为“重试”，已取消任务显示为“重新入队”，但都不改成拿全部在架情报重跑。
func (s *svcImpl) RetryMatchTask(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	taskNo := strings.TrimSpace(c.Param("taskNo"))
	item, err := s.repo.GetMatchTask(ctx, taskNo)
	if err != nil || item == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "匹配任务不存在", nil)
		return
	}
	if !matchTaskRetryable(item.Status) {
		handler.SendNormalResp(c, entity.ErrCodeParam,
			"仅成功、失败或已取消的任务可以重新入队，当前状态："+MatchStatusName(item.Status), nil)
		return
	}
	spec := matchTaskSpec{
		TaskType:       item.TaskType,
		ScopeKind:      item.ScopeKind,
		ScopeRef:       item.ScopeRef,
		RangeFrom:      item.RangeFrom,
		RangeTo:        item.RangeTo,
		UserID:         item.UserID,
		SubscriptionID: item.SubscriptionID,
		RetryOf:        item.TaskNo,
	}
	if item.ScopeKind == MatchScopeNotice {
		spec.NoticeID, _ = strconv.ParseInt(item.ScopeRef, 10, 64)
	}
	created, err := s.enqueueMatchTask(ctx, spec)
	if err != nil {
		logger.Warnw("重新入队匹配任务失败", "task_no", taskNo, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "重新入队失败："+err.Error(), nil)
		return
	}
	if item.ScopeKind == MatchScopeSubscription && item.SubscriptionID > 0 {
		if err := s.repo.SetSubscriptionBackfill(ctx, item.SubscriptionID, map[string]interface{}{
			"backfill_status": "running",
		}); err != nil {
			logger.Warnw("回写订阅回溯状态失败", "subscription_id", item.SubscriptionID, "err", err)
		}
	}
	handler.SendOKResp(c, map[string]any{"task_no": created.TaskNo, "retry_of": taskNo})
}

// matchTaskRetryable 判断任务是否处于可重跑的终态。
func matchTaskRetryable(status string) bool {
	switch status {
	case "success", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// DeleteMatchTask 删除终态任务（排队中 / 执行中必须先取消）。
func (s *svcImpl) DeleteMatchTask(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	taskNo := strings.TrimSpace(c.Param("taskNo"))
	item, err := s.repo.GetMatchTask(ctx, taskNo)
	if err != nil || item == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "匹配任务不存在", nil)
		return
	}
	if item.Status == "pending" || item.Status == "running" {
		handler.SendNormalResp(c, entity.ErrCodeParam,
			"任务仍在排队或执行中，请先取消后再删除", nil)
		return
	}
	if err := s.repo.DeleteMatchTask(ctx, taskNo); err != nil {
		logger.Warnw("删除匹配任务失败", "task_no", taskNo, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败，请稍后重试", nil)
		return
	}
	if item.QueuedTaskID != "" {
		// 队列元数据可能已过期，清理失败不影响账本删除结果
		if err := s.queue.DeleteTask(ctx, item.QueuedTaskID); err != nil {
			logger.Infow("清理队列任务元数据失败", "task_no", taskNo, "err", err)
		}
	}
	handler.SendOKResp(c, map[string]any{"task_no": taskNo})
}

// RescanMatches 手动补扫：按时间窗对在架情报重新匹配全量启用订阅。
func (s *svcImpl) RescanMatches(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	ctx := c.Request.Context()
	var req struct {
		DateFrom string `json:"date_from"`
		DateTo   string `json:"date_to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	from, err := parseDay(req.DateFrom)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "开始日期格式应为 YYYY-MM-DD", nil)
		return
	}
	to, err := parseDay(req.DateTo)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "结束日期格式应为 YYYY-MM-DD", nil)
		return
	}
	if from == nil || to == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请提供完整的补扫时间窗", nil)
		return
	}
	if to.Before(*from) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "结束日期不能早于开始日期", nil)
		return
	}
	snapshot, err := s.repo.MaxNoticeID(ctx)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "读取情报范围失败", nil)
		return
	}
	total, err := s.repo.CountNoticesForMatch(ctx, intelRepo.MatchScope{
		Kind: MatchScopeWindow, From: from, To: to, UpToID: snapshot,
	})
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "统计待补扫情报失败", nil)
		return
	}
	if total == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "该时间窗内没有在架情报，无需补扫", nil)
		return
	}
	if total > maxRescanNotices {
		handler.SendNormalResp(c, entity.ErrCodeParam,
			"该时间窗覆盖情报过多（"+strconv.FormatInt(total, 10)+" 条），请缩小时间范围后重试", nil)
		return
	}
	created, err := s.enqueueMatchTask(ctx, matchTaskSpec{
		TaskType:  MatchTaskTypeRescan,
		ScopeKind: MatchScopeWindow,
		ScopeRef:  from.Format("20060102") + "-" + to.Format("20060102"),
		RangeFrom: from,
		RangeTo:   to,
	})
	if err != nil {
		logger.Warnw("创建补扫任务失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "创建补扫任务失败："+err.Error(), nil)
		return
	}
	handler.SendOKResp(c, map[string]any{
		"task_no":      created.TaskNo,
		"notice_total": total,
	})
}

// ── 内部工具 ────────────────────────────────────────────────────

// matchTaskLookups 列表级联查结果：用户与订阅只按 ID 批量取一次，避免逐行查库。
type matchTaskLookups struct {
	users map[int64]*model.User
	subs  map[int64]*model.TenderIntelSubscription
}

// loadMatchLookups 收集任务里的用户 / 订阅 ID 并批量查询。
func (s *svcImpl) loadMatchLookups(ctx context.Context, items []*model.TenderIntelMatchTask) matchTaskLookups {
	lookups := matchTaskLookups{}
	userIDs := make([]int64, 0, len(items))
	subIDs := make([]int64, 0, len(items))
	seenUser := make(map[int64]struct{}, len(items))
	seenSub := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.UserID > 0 {
			if _, ok := seenUser[item.UserID]; !ok {
				seenUser[item.UserID] = struct{}{}
				userIDs = append(userIDs, item.UserID)
			}
		}
		if item.SubscriptionID > 0 {
			if _, ok := seenSub[item.SubscriptionID]; !ok {
				seenSub[item.SubscriptionID] = struct{}{}
				subIDs = append(subIDs, item.SubscriptionID)
			}
		}
	}
	if users, err := s.repo.ListMatchUsersByIDs(ctx, userIDs); err == nil {
		lookups.users = users
	}
	if subs, err := s.repo.ListSubscriptionsByIDs(ctx, subIDs); err == nil {
		lookups.subs = subs
	}
	return lookups
}

// toMatchTaskView 组装视图；排队中的任务补上真实的执行顺位。
func (s *svcImpl) toMatchTaskView(ctx context.Context, item *model.TenderIntelMatchTask, lookups matchTaskLookups) matchTaskView {
	view := matchTaskView{
		TaskNo:             item.TaskNo,
		TaskType:           item.TaskType,
		TaskTypeName:       MatchTaskTypeName(item.TaskType),
		ScopeKind:          item.ScopeKind,
		ScopeRef:           item.ScopeRef,
		RangeFrom:          formatTimePtr(item.RangeFrom),
		RangeTo:            formatTimePtr(item.RangeTo),
		NoticeTotal:        item.NoticeTotal,
		ScannedCount:       item.ScannedCount,
		MatchedNoticeCount: item.MatchedNoticeCount,
		AlertCount:         item.AlertCount,
		Status:             item.Status,
		Priority:           item.Priority,
		CancelRequested:    item.CancelRequested == 1,
		Attempts:           item.Attempts,
		RetryOf:            item.RetryOf,
		SubscriptionID:     item.SubscriptionID,
		UserID:             item.UserID,
		LastError:          item.LastError,
		StartedAt:          formatTimePtr(item.StartedAt),
		FinishedAt:         formatTimePtr(item.FinishedAt),
		CreatedAt:          formatTime(item.CreatedAt),
	}
	// 订阅回溯类任务要能一眼看出对应哪个用户、哪条订阅，运维才不用再去翻库
	if sub, ok := lookups.subs[item.SubscriptionID]; ok && sub != nil {
		view.SubscriptionName = sub.Name
	}
	if lookup, ok := lookups.users[item.UserID]; ok && lookup != nil {
		view.UserName = firstNonEmpty(lookup.Nickname, lookup.Username)
		view.UserMobile = lookup.Mobile
	}
	if item.Status == "pending" && item.QueuedTaskID != "" {
		if order, err := s.queue.QueueOrder(ctx, item.QueuedTaskID); err == nil {
			view.QueueOrder = order
		}
	}
	return view
}

// syncSubscriptionBackfillCancelled 取消回溯任务时同步订阅的回溯状态。
func (s *svcImpl) syncSubscriptionBackfillCancelled(ctx context.Context, item *model.TenderIntelMatchTask) {
	if item.ScopeKind != MatchScopeSubscription || item.SubscriptionID <= 0 {
		return
	}
	if err := s.repo.SetSubscriptionBackfill(ctx, item.SubscriptionID, map[string]interface{}{
		"backfill_status": "cancelled",
	}); err != nil {
		s.logger.Warnw("回写订阅回溯取消状态失败", "subscription_id", item.SubscriptionID, "err", err)
	}
}

// parseDay 解析 YYYY-MM-DD。
func parseDay(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// MatchTaskTypeName 任务类型展示名。
func MatchTaskTypeName(taskType string) string {
	switch taskType {
	case MatchTaskTypeCollectRun:
		return "采集后匹配"
	case MatchTaskTypeManual:
		return "手工发布匹配"
	case MatchTaskTypeImport:
		return "批量导入匹配"
	case MatchTaskTypeBackfill:
		return "订阅回溯"
	case MatchTaskTypeRescan:
		return "手动补扫"
	default:
		return taskType
	}
}

// MatchStatusName 任务状态展示名。
func MatchStatusName(status string) string {
	switch status {
	case "pending":
		return "排队中"
	case "running":
		return "执行中"
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "cancelled":
		return "已取消"
	default:
		return status
	}
}
