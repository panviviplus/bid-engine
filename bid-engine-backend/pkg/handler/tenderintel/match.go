package tenderintel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/taskqueue"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

// TaskTypeTenderIntelMatch 订阅匹配任务类型（与 task_queues 配置键一致）。
const TaskTypeTenderIntelMatch = "tender_intel_match"

// matchBatchSize 单批处理的情报条数。
const matchBatchSize = 200

// matchBodyExcerptRunes 关键词匹配只看正文前 2 万字符，避免长正文拖慢匹配。
const matchBodyExcerptRunes = 20000

// 匹配任务类型与范围类型。
const (
	MatchTaskTypeCollectRun = "collect_run"
	MatchTaskTypeManual     = "manual_notice"
	MatchTaskTypeImport     = "import_batch"
	MatchTaskTypeBackfill   = "subscription_backfill"
	MatchTaskTypeRescan     = "manual_rescan"

	MatchScopeRun          = "run"
	MatchScopeNotice       = "notice"
	MatchScopeBatch        = "batch"
	MatchScopeSubscription = "subscription"
	MatchScopeWindow       = "window"

	// 提醒来源：与用户界面无关，只供运维排查
	MatchSourceCollect  = "collect"
	MatchSourceBackfill = "backfill"
)

// backfillWindowDays 订阅回溯的时间窗（天）。
const backfillWindowDays = 30

// matchTaskPayload 队列负载：只带任务号，范围与断点都以账本为准。
type matchTaskPayload struct {
	TaskNo string `json:"task_no"`
}

// MatchTaskHandler 订阅匹配队列处理器。
type MatchTaskHandler struct {
	Svc Service
}

// Handle 执行一个匹配任务。
func (h *MatchTaskHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload matchTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil {
		return err
	}
	if payload.TaskNo == "" {
		return fmt.Errorf("匹配任务负载缺少 task_no")
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("招标情报站匹配处理器初始化异常")
	}
	return impl.runMatchTask(ctx, payload.TaskNo)
}

// HandleTaskFinal 匹配任务终态回调：重试耗尽时把账本落到失败态，避免永远停在执行中。
func (h *MatchTaskHandler) HandleTaskFinal(ctx context.Context, task *taskqueue.Task, succeeded bool) {
	if task == nil || succeeded {
		return
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return
	}
	var payload matchTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil || payload.TaskNo == "" {
		return
	}
	current, err := impl.repo.GetMatchTask(ctx, payload.TaskNo)
	if err != nil || current == nil {
		return
	}
	if current.Status != "running" && current.Status != "pending" {
		return
	}
	impl.finishMatchTask(ctx, payload.TaskNo, "failed",
		fmt.Sprintf("匹配任务重试耗尽（已尝试 %d 次），请查看任务详情后重试", task.Attempts))
}

// ── 任务创建 ────────────────────────────────────────────────────

// matchTaskSpec 创建匹配任务的输入。
type matchTaskSpec struct {
	TaskType       string
	ScopeKind      string
	ScopeRef       string
	NoticeID       int64
	RangeFrom      *time.Time
	RangeTo        *time.Time
	UserID         int64
	SubscriptionID int64
	RetryOf        string
}

// newMatchTaskNo 生成任务号：时间戳 + 随机后缀，跨进程也不会撞号。
func newMatchTaskNo() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("match_%s_%s", NowFunc().Format("20060102T150405"), hex.EncodeToString(b))
}

// matchTaskToScope 把账本行还原成情报范围。
func matchTaskToScope(item *model.TenderIntelMatchTask) intelRepo.MatchScope {
	scope := intelRepo.MatchScope{
		Kind:   item.ScopeKind,
		Ref:    item.ScopeRef,
		From:   item.RangeFrom,
		To:     item.RangeTo,
		UpToID: item.NoticeIDSnapshot,
		Cursor: item.CursorNoticeID,
		Limit:  matchBatchSize,
	}
	if item.ScopeKind == MatchScopeNotice {
		scope.NoticeID, _ = strconv.ParseInt(item.ScopeRef, 10, 64)
	}
	return scope
}

// enqueueMatchTask 建账本并入队；范围在创建瞬间固定（快照情报 ID 上界）。
func (s *svcImpl) enqueueMatchTask(ctx context.Context, spec matchTaskSpec) (*model.TenderIntelMatchTask, error) {
	snapshot, err := s.repo.MaxNoticeID(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取情报 ID 上界失败: %w", err)
	}
	taskNo := newMatchTaskNo()
	item := &model.TenderIntelMatchTask{
		TaskNo:           taskNo,
		TaskType:         spec.TaskType,
		ScopeKind:        spec.ScopeKind,
		ScopeRef:         spec.ScopeRef,
		RangeFrom:        spec.RangeFrom,
		RangeTo:          spec.RangeTo,
		NoticeIDSnapshot: snapshot,
		Status:           "pending",
		Priority:         "normal",
		RetryOf:          spec.RetryOf,
		UserID:           spec.UserID,
		SubscriptionID:   spec.SubscriptionID,
	}
	// 范围一旦确定就先统计覆盖情报数，管理端列表不必再回算
	if total, err := s.repo.CountNoticesForMatch(ctx, matchTaskToScope(item)); err == nil {
		item.NoticeTotal = int32(total)
	}
	if err := s.repo.CreateMatchTask(ctx, item); err != nil {
		return nil, fmt.Errorf("创建匹配任务失败: %w", err)
	}
	// 队列任务 ID 直接复用任务号：重试会生成新的任务号，不会被入队脚本的 EXISTS 判定吞掉
	if _, err := s.queue.Enqueue(ctx, TaskTypeTenderIntelMatch, matchTaskPayload{TaskNo: taskNo}, taskqueue.EnqueueOpts{
		TaskID: taskNo,
	}); err != nil {
		_ = s.repo.UpdateMatchTask(ctx, taskNo, map[string]interface{}{
			"status":      "failed",
			"last_error":  "任务投递失败: " + err.Error(),
			"finished_at": NowFunc(),
		})
		return item, err
	}
	if err := s.repo.UpdateMatchTask(ctx, taskNo, map[string]interface{}{
		"queued_task_id": taskNo,
	}); err != nil {
		s.logger.Warnw("回写匹配任务队列 ID 失败", "task_no", taskNo, "err", err)
	}
	return item, nil
}

// enqueueMatchTaskForRun 采集批次完成后触发（范围 = 本批次入库的情报）。
func (s *svcImpl) enqueueMatchTaskForRun(ctx context.Context, runID string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	// 重试/重复回调可能重复触发，账本里已有同批次任务时不再新建
	if existing, err := s.repo.FindMatchTaskByScope(ctx, MatchScopeRun, runID); err == nil && existing != nil {
		return
	}
	if _, err := s.enqueueMatchTask(ctx, matchTaskSpec{
		TaskType:  MatchTaskTypeCollectRun,
		ScopeKind: MatchScopeRun,
		ScopeRef:  runID,
	}); err != nil {
		s.logger.Warnw("创建采集批次匹配任务失败", "run_id", runID, "err", err)
	}
}

// enqueueMatchTaskForNotices 手工发布 / 批量导入后触发（范围 = 具体的这类情报）。
func (s *svcImpl) enqueueMatchTaskForNotices(ctx context.Context, taskType, scopeKind, scopeRef string, noticeID int64) {
	if _, err := s.enqueueMatchTask(ctx, matchTaskSpec{
		TaskType:  taskType,
		ScopeKind: scopeKind,
		ScopeRef:  scopeRef,
		NoticeID:  noticeID,
	}); err != nil {
		s.logger.Warnw("创建情报匹配任务失败",
			"task_type", taskType, "scope_ref", scopeRef, "notice_id", noticeID, "err", err)
	}
}

// enqueueSubscriptionBackfill 订阅新建 / 首次启用时触发回溯（近 30 天在架情报）。
func (s *svcImpl) enqueueSubscriptionBackfill(ctx context.Context, sub *model.TenderIntelSubscription) {
	if sub == nil || sub.ID <= 0 {
		return
	}
	if sub.BackfilledAt != nil {
		return
	}
	if existing, err := s.repo.FindRunningMatchTaskBySubscription(ctx, sub.ID); err == nil && existing != nil {
		return
	}
	from := NowFunc().AddDate(0, 0, -backfillWindowDays)
	to := NowFunc()
	if _, err := s.enqueueMatchTask(ctx, matchTaskSpec{
		TaskType:       MatchTaskTypeBackfill,
		ScopeKind:      MatchScopeSubscription,
		ScopeRef:       strconv.FormatInt(sub.ID, 10),
		RangeFrom:      &from,
		RangeTo:        &to,
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
	}); err != nil {
		s.logger.Warnw("创建订阅回溯任务失败", "subscription_id", sub.ID, "err", err)
		return
	}
	if err := s.repo.SetSubscriptionBackfill(ctx, sub.ID, map[string]interface{}{
		"backfill_status": "running",
	}); err != nil {
		s.logger.Warnw("回写订阅回溯状态失败", "subscription_id", sub.ID, "err", err)
	}
}

// ── 任务执行 ────────────────────────────────────────────────────

// runMatchTask 执行一个匹配任务：按固定范围分批处理，命中即写站内提醒。
func (s *svcImpl) runMatchTask(ctx context.Context, taskNo string) error {
	item, err := s.repo.GetMatchTask(ctx, taskNo)
	if err != nil {
		return fmt.Errorf("读取匹配任务失败: %w", err)
	}
	switch item.Status {
	case "success", "failed", "cancelled":
		// 已是终态：重投递不重复执行
		return nil
	}
	// 僵尸任务收尾：与采集批次同口径，避免“执行中”永远不消失
	if closed, err := s.repo.CloseStaleMatchTasks(ctx, NowFunc().Add(-30*time.Minute), "执行超时未完成，已自动标记为失败"); err == nil && closed > 0 {
		s.logger.Warnw("收尾超时匹配任务", "closed", closed)
	}
	claimed, err := s.repo.UpdateMatchTaskIfStatus(ctx, taskNo, []string{"pending"}, map[string]interface{}{
		"status":     "running",
		"started_at": NowFunc(),
		"attempts":   item.Attempts + 1,
	})
	if err != nil {
		return fmt.Errorf("领取匹配任务失败: %w", err)
	}
	if !claimed {
		// 已被取消或已被其它 worker 领取
		return nil
	}

	// 订阅集合只加载一次：用户多、订阅多时也不该按情报逐条查库
	subs, err := s.repo.ListEnabledSubscriptions(ctx)
	if err != nil {
		s.finishMatchTask(ctx, taskNo, "failed", "读取订阅规则失败: "+err.Error())
		return fmt.Errorf("读取订阅规则失败: %w", err)
	}
	// 回溯任务只针对自己那条订阅
	var restrictSubID int64
	matchSource := MatchSourceCollect
	if item.ScopeKind == MatchScopeSubscription {
		restrictSubID = item.SubscriptionID
		matchSource = MatchSourceBackfill
		filtered := make([]*model.TenderIntelSubscription, 0, 1)
		for _, sub := range subs {
			if sub.ID == item.SubscriptionID {
				filtered = append(filtered, sub)
			}
		}
		subs = filtered
	}

	scope := matchTaskToScope(item)
	scanned, matchedNotices, alertCount := 0, 0, 0
	for {
		// 取消请求在批次边界生效：单批只有 200 条，响应足够及时
		if fresh, err := s.repo.GetMatchTask(ctx, taskNo); err == nil && fresh.CancelRequested == 1 {
			s.finishMatchTask(ctx, taskNo, "cancelled", "管理员取消")
			return taskqueue.Cancelled("管理员取消")
		}
		notices, err := s.repo.ListNoticesForMatch(ctx, scope)
		if err != nil {
			s.finishMatchTask(ctx, taskNo, "failed", "读取待匹配情报失败: "+err.Error())
			return fmt.Errorf("读取待匹配情报失败: %w", err)
		}
		if len(notices) == 0 {
			break
		}
		ids := make([]int64, 0, len(notices))
		for _, notice := range notices {
			ids = append(ids, notice.ID)
		}
		industryMap, err := s.repo.ListNoticeIndustryMap(ctx, ids)
		if err != nil {
			s.finishMatchTask(ctx, taskNo, "failed", "读取情报行业标签失败: "+err.Error())
			return fmt.Errorf("读取情报行业标签失败: %w", err)
		}
		hitNotices, alerts := s.matchNoticesBatch(ctx, notices, industryMap, subs, matchSource, restrictSubID)
		scanned += len(notices)
		matchedNotices += hitNotices
		alertCount += alerts
		// 断点推进到本批最后一条，worker 崩溃后从这里续跑
		scope.Cursor = notices[len(notices)-1].ID
		if err := s.repo.UpdateMatchTask(ctx, taskNo, map[string]interface{}{
			"cursor_notice_id":     scope.Cursor,
			"scanned_count":        scanned,
			"matched_notice_count": matchedNotices,
			"alert_count":          alertCount,
		}); err != nil {
			s.logger.Warnw("回写匹配任务进度失败", "task_no", taskNo, "err", err)
		}
		if len(notices) < matchBatchSize {
			break
		}
	}

	if scope.UpToID > 0 && scope.Cursor == 0 && item.NoticeIDSnapshot > 0 {
		// 范围内没有情报也要把断点推进到快照上界，避免任务看起来“没动”
		scope.Cursor = item.NoticeIDSnapshot
	}
	if err := s.repo.UpdateMatchTask(ctx, taskNo, map[string]interface{}{
		"cursor_notice_id": scope.Cursor,
		"scanned_count":    scanned,
	}); err != nil {
		s.logger.Warnw("回写匹配任务断点失败", "task_no", taskNo, "err", err)
	}
	s.finishMatchTask(ctx, taskNo, "success", "")

	// 回溯任务收尾：上报订阅回溯状态
	if item.ScopeKind == MatchScopeSubscription {
		if err := s.repo.SetSubscriptionBackfill(ctx, item.SubscriptionID, map[string]interface{}{
			"backfill_status":           "success",
			"backfill_cursor_notice_id": scope.Cursor,
			"backfilled_at":             NowFunc(),
		}); err != nil {
			s.logger.Warnw("回写订阅回溯完成状态失败", "subscription_id", item.SubscriptionID, "err", err)
		}
	}
	// 采集批次回填匹配提醒数
	if item.ScopeKind == MatchScopeRun && alertCount > 0 {
		if err := s.repo.AddCollectRunMatchedCount(ctx, item.ScopeRef, alertCount); err != nil {
			s.logger.Warnw("回填采集批次匹配数失败", "run_id", item.ScopeRef, "err", err)
		}
	}
	s.logger.Infow("订阅匹配任务完成",
		"task_no", taskNo, "task_type", item.TaskType,
		"scanned", scanned, "matched_notices", matchedNotices, "alerts", alertCount)
	return nil
}

// finishMatchTask 落任务终态。
func (s *svcImpl) finishMatchTask(ctx context.Context, taskNo, status, errMsg string) {
	updates := map[string]interface{}{
		"status":      status,
		"finished_at": NowFunc(),
	}
	if errMsg != "" {
		updates["last_error"] = truncateRunes(errMsg, 500)
	}
	if err := s.repo.UpdateMatchTask(ctx, taskNo, updates); err != nil {
		s.logger.Warnw("回写匹配任务终态失败", "task_no", taskNo, "status", status, "err", err)
	}
}

// matchNoticesBatch 对一批情报执行订阅匹配，命中即写站内提醒。
//
// 返回命中的情报条数与实际写入的提醒条数。提醒写入用唯一键去重，
// 因此任务重跑不会重复提醒。
func (s *svcImpl) matchNoticesBatch(
	ctx context.Context,
	notices []*model.TenderIntelNotice,
	industryMap map[int64][]string,
	subs []*model.TenderIntelSubscription,
	matchSource string,
	restrictSubID int64,
) (int, int) {
	if len(notices) == 0 || len(subs) == 0 {
		return 0, 0
	}
	alerts := make([]*model.TenderIntelAlert, 0, 16)
	matchedSubIDs := make([]int64, 0, 16)
	matchedNotices := 0
	for _, notice := range notices {
		industries := industryMap[notice.ID]
		haystackTitle := strings.ToLower(notice.Title)
		haystackBody := strings.ToLower(truncateRunes(notice.BodyText, matchBodyExcerptRunes))
		hitInNotice := false
		for _, sub := range subs {
			if restrictSubID > 0 && sub.ID != restrictSubID {
				continue
			}
			// 结构化条件先过一遍：不满足就不必做关键词全文扫描
			if !structuralMatchPossible(sub, notice, industries) {
				continue
			}
			hit, keywords, reason := MatchSubscription(sub, notice, industries, haystackTitle, haystackBody)
			if !hit {
				continue
			}
			hitInNotice = true
			alerts = append(alerts, &model.TenderIntelAlert{
				UserID:           sub.UserID,
				SubscriptionID:   sub.ID,
				NoticeID:         notice.ID,
				SubscriptionName: sub.Name,
				NoticeTitle:      truncateRunes(notice.Title, 240),
				MatchedKeywords:  EncodeStringList(keywords),
				MatchedReason:    truncateRunes(reason, 240),
				MatchSource:      matchSource,
				FeishuPushStatus: "pending",
			})
			matchedSubIDs = append(matchedSubIDs, sub.ID)
		}
		if hitInNotice {
			matchedNotices++
		}
	}
	if len(alerts) == 0 {
		return matchedNotices, 0
	}
	rows, err := s.repo.CreateAlertsIgnoreDuplicate(ctx, alerts)
	if err != nil {
		s.logger.Warnw("写入站内提醒失败", "count", len(alerts), "err", err)
		return matchedNotices, 0
	}
	if rows > 0 {
		_ = s.repo.BumpSubscriptionMatched(ctx, matchedSubIDs, NowFunc())
	}
	return matchedNotices, int(rows)
}

// structuralMatchPossible 结构化条件的粗筛。
//
// 只做“肯定不可能命中”的排除，语义必须与 MatchSubscription 完全一致；
// 目的是让用户很多时不必为每条情报做全文关键词扫描。
//
// 注意：行业不参与预筛。行业与关键词互为替代路径，行业不命中并不代表
// 整条订阅不命中（关键词命中同样算），用它预筛会误杀。
func structuralMatchPossible(sub *model.TenderIntelSubscription, notice *model.TenderIntelNotice, industries []string) bool {
	_ = industries
	if types := DecodeStringList(sub.NoticeTypes); len(types) > 0 && isRecognizedNoticeType(notice.NoticeType) {
		hit := false
		for _, item := range types {
			if item == notice.NoticeType {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if regions := DecodeStringList(sub.Regions); len(regions) > 0 {
		hit := false
		for _, region := range regions {
			if region == notice.RegionProvince || region == notice.RegionCity ||
				strings.Contains(notice.RegionText, region) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if notice.BudgetAmount != nil {
		amount := *notice.BudgetAmount
		if sub.BudgetMin != nil && amount < *sub.BudgetMin {
			return false
		}
		if sub.BudgetMax != nil && amount > *sub.BudgetMax {
			return false
		}
	}
	return true
}
