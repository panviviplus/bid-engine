package tenderintel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/taskqueue"
	intelRepo "bid-engine/pkg/repo/tenderintel"
)

// TaskTypeTenderIntelCollect 采集任务类型（与 task_queues 配置键一致）。
const TaskTypeTenderIntelCollect = "tender_intel_collect"

// TaskTypeTenderIntelEnrich 打标入库任务类型。
const TaskTypeTenderIntelEnrich = "tender_intel_enrich"

// collectTaskPayload 采集任务负载。
type collectTaskPayload struct {
	RunID     string `json:"run_id"`
	SourceKey string `json:"source_key"`
}

// enrichTaskPayload 打标任务负载。
type enrichTaskPayload struct {
	RunID string `json:"run_id"`
}

// CollectTaskHandler 采集队列处理器。
type CollectTaskHandler struct {
	Svc Service
}

// EnrichTaskHandler 打标队列处理器。
type EnrichTaskHandler struct {
	Svc Service
}

// Handle 执行单个源的采集：调用采集服务 → 去重入库 → 回写源健康度与批次明细。
func (h *CollectTaskHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload collectTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil {
		return err
	}
	if payload.SourceKey == "" || payload.RunID == "" {
		return fmt.Errorf("采集任务负载缺少 run_id 或 source_key")
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("招标情报站采集处理器初始化异常")
	}
	return impl.collectOneSource(ctx, payload.RunID, payload.SourceKey)
}

// Handle 执行打标与入库收尾。
func (h *EnrichTaskHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload enrichTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil {
		return err
	}
	if payload.RunID == "" {
		return fmt.Errorf("打标任务负载缺少 run_id")
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("招标情报站打标处理器初始化异常")
	}
	return impl.enrichRun(ctx, payload.RunID)
}

// HandleTaskFinal 采集任务的终态回调（成功不入此分支）。
//
// 采集任务重试耗尽后不会再回到 collectOneSource 的成功分支：若不在这里兜底收尾，
// 批次会永远停在 running，既误导运维（看不到失败原因），也会阻塞后续触发（防重入）。
func (h *CollectTaskHandler) HandleTaskFinal(ctx context.Context, task *taskqueue.Task, succeeded bool) {
	if task == nil || succeeded {
		return
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return
	}
	var payload collectTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil {
		impl.logger.Warnw("采集任务终态回调解析负载失败", "task_id", task.ID, "err", err)
		return
	}
	if payload.RunID == "" || payload.SourceKey == "" {
		return
	}
	// 批次可能已被管理员删除：此时不再补写明细，避免留下孤儿记录
	if run, err := impl.repo.GetRun(ctx, payload.RunID); err != nil || run == nil {
		return
	}
	detail, err := impl.repo.GetRunSource(ctx, payload.RunID, payload.SourceKey)
	reason := "采集任务执行失败（已重试 " + fmt.Sprintf("%d", task.Attempts) + " 次），请查看源状态与采集服务"
	switch {
	case err != nil:
		// 明细缺失：任务在写回前就失败（解析异常、源记录缺失等），
		// 补一条失败明细保证批次能被正常收尾，并补记一次源健康度。
		fallback := &model.TenderIntelRunSource{
			RunID:     payload.RunID,
			SourceKey: payload.SourceKey,
			Status:    "failed",
			Error:     reason,
		}
		if src, srcErr := impl.repo.GetSource(ctx, payload.SourceKey); srcErr == nil && src != nil {
			fallback.SourceName = src.Name
		}
		_ = impl.repo.UpsertRunSource(ctx, fallback)
		if err := impl.repo.SaveSourceHealth(ctx, payload.SourceKey, "", NowFunc(), false, reason); err != nil {
			impl.logger.Warnw("回写采集源失败状态失败", "source_key", payload.SourceKey, "err", err)
		}
	case detail.Status == "success":
		// 重试后已成功写回，无需再改状态
		impl.maybeDispatchEnrich(payload.RunID)
		return
	default:
		// 明细已存在（collectOneSource 已写入失败原因并记过一次源健康度），
		// 这里只收敛状态，避免“连续失败次数”被重复累加。
		detail.Status = "failed"
		if strings.TrimSpace(detail.Error) == "" {
			detail.Error = reason
		}
		_ = impl.repo.UpsertRunSource(ctx, detail)
	}
	impl.maybeDispatchEnrich(payload.RunID)
}

// HandleTaskFinal 打标任务的终态回调：打标失败也要把批次落到终态，避免批次永久停在 running。
func (h *EnrichTaskHandler) HandleTaskFinal(ctx context.Context, task *taskqueue.Task, succeeded bool) {
	if task == nil || succeeded {
		return
	}
	impl, ok := h.Svc.(*svcImpl)
	if !ok {
		return
	}
	var payload enrichTaskPayload
	if err := decodeTaskPayload(task.Payload, &payload); err != nil {
		impl.logger.Warnw("打标任务终态回调解析负载失败", "task_id", task.ID, "err", err)
		return
	}
	if payload.RunID == "" {
		return
	}
	if run, err := impl.repo.GetRun(ctx, payload.RunID); err != nil || run == nil {
		return
	}
	impl.finalizeRun(ctx, payload.RunID, "打标任务执行失败，公告保留未打标状态，等待下一轮补打")
	// 打标失败也要投递匹配任务：未打标的情报已有关键词兜底行业标签，
	// 匹配照常执行，避免因为打标链路的偶发失败让提醒静默丢失。
	impl.enqueueMatchTaskForRun(ctx, payload.RunID)
}

// decodeTaskPayload 解析队列任务负载。
//
// 历史包袱：早期版本把 json.Marshal 得到的 []byte 交给 queue.Enqueue，而 Enqueue 内部
// 还会再序列化一次，于是 Redis 中存的是 base64 字符串（形如 "eyJydW5faWQi...")。
// 两种形态都兼容，避免历史任务永远卡在解析阶段。
func decodeTaskPayload(raw string, out any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("任务负载为空")
	}
	if err := json.Unmarshal([]byte(trimmed), out); err == nil {
		return nil
	}
	var encoded string
	if err := json.Unmarshal([]byte(trimmed), &encoded); err != nil {
		return fmt.Errorf("解析任务负载失败: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return fmt.Errorf("解析任务负载失败: %w", err)
	}
	if err := json.Unmarshal(decoded, out); err != nil {
		return fmt.Errorf("解析任务负载失败: %w", err)
	}
	return nil
}

// StartCollectRound 生成一轮采集批次，并为每个启用源投递采集任务。
func (s *svcImpl) StartCollectRound(ctx context.Context, triggerType string, sourceKeys []string) (string, error) {
	return s.startCollectRound(ctx, triggerType, sourceKeys, "")
}

// startCollectRound 采集轮次的实际实现；retryOf 记录该批次由哪个历史批次重试而来。
func (s *svcImpl) startCollectRound(ctx context.Context, triggerType string, sourceKeys []string, retryOf string) (string, error) {
	if triggerType == "" {
		triggerType = "cron"
	}

	// 先把超时未结束的批次收尾：worker 未消费（重启/队列丢失/采集服务不可用）时，
	// 批次会永远停在 running，从而永久阻塞后续触发。
	if closed, err := s.repo.CloseStaleRuns(ctx, NowFunc().Add(-staleRunAfter), "执行超时未完成，已自动标记为失败（可能是采集服务不可用或后端重启导致任务未消费）"); err != nil {
		s.logger.Warnw("收尾超时批次失败", "err", err)
	} else if closed > 0 {
		s.logger.Infow("已收尾超时未完成的采集批次", "count", closed)
	}

	sources, err := s.repo.ListEnabledSources(ctx)
	if err != nil {
		return "", fmt.Errorf("读取采集源失败: %w", err)
	}
	if len(sourceKeys) > 0 {
		want := make(map[string]struct{}, len(sourceKeys))
		for _, key := range sourceKeys {
			want[key] = struct{}{}
		}
		filtered := make([]*model.TenderIntelSource, 0, len(sources))
		for _, src := range sources {
			if _, ok := want[src.SourceKey]; ok {
				filtered = append(filtered, src)
			}
		}
		sources = filtered
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("没有启用的采集源")
	}

	// 防重入：整轮采集（全部源）不允许与正在执行的整轮重叠；
	// 指定源的触发只要求「该源当前没有正在执行的批次」，避免被无关的整轮长时间阻塞。
	keys := make([]string, 0, len(sources))
	for _, src := range sources {
		keys = append(keys, src.SourceKey)
	}
	scope := "all"
	if len(sourceKeys) > 0 {
		scope = "single"
	}
	if blocked, running := s.findBlockingRun(ctx, scope, keys); blocked {
		return "", fmt.Errorf("已有采集批次正在执行（%s，%s 启动），请稍后再试或等待其结束", running.RunID, running.StartedAt.Format("15:04:05"))
	}

	now := NowFunc()
	runID := fmt.Sprintf("run_%s_%d", now.Format("20060102T150405"), now.UnixNano()%1e6)
	sourceKeysJSON := ""
	if scope == "single" {
		encoded, err := json.Marshal(keys)
		if err != nil {
			return "", err
		}
		sourceKeysJSON = string(encoded)
	}
	run := &model.TenderIntelCollectRun{
		RunID:       runID,
		TriggerType: triggerType,
		Scope:       scope,
		SourceKeys:  sourceKeysJSON,
		Status:      "running",
		StartedAt:   now,
		SourceTotal: int32(len(sources)),
	}
	if retryOf != "" {
		run.RetryOf = retryOf
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return "", fmt.Errorf("创建采集批次失败: %w", err)
	}

	ctxBg := context.Background()
	for _, src := range sources {
		taskID := fmt.Sprintf("intel_collect_%s_%s", runID, src.SourceKey)
		// payload 直接传结构体：Enqueue 内部会做一次 json.Marshal，
		// 若这里再传 []byte 会被序列化成 base64 字符串，消费端无法解析。
		if _, err := s.queue.Enqueue(ctxBg, TaskTypeTenderIntelCollect, collectTaskPayload{
			RunID:     runID,
			SourceKey: src.SourceKey,
		}, taskqueue.EnqueueOpts{
			TaskID: taskID,
		}); err != nil {
			// 单源投递失败不阻断整轮；写入明细便于排查
			_ = s.repo.UpsertRunSource(ctx, &model.TenderIntelRunSource{
				RunID:      runID,
				SourceKey:  src.SourceKey,
				SourceName: src.Name,
				Status:     "failed",
				Error:      "任务投递失败: " + err.Error(),
			})
			s.logger.Warnw("投递采集任务失败", "run_id", runID, "source_key", src.SourceKey, "err", err)
		}
	}
	return runID, nil
}

// findBlockingRun 判断是否存在会与本次触发冲突的正在执行批次。
func (s *svcImpl) findBlockingRun(ctx context.Context, scope string, keys []string) (bool, *model.TenderIntelCollectRun) {
	running, err := s.repo.ListRunningRuns(ctx)
	if err != nil {
		s.logger.Warnw("读取执行中批次失败", "err", err)
		return false, nil
	}
	for _, item := range running {
		// 整轮采集尚未结束时，任何触发都排队等待（避免同时打满上游站点）
		if item.Scope == "" || item.Scope == "all" {
			return true, item
		}
		if scope == "all" {
			return true, item
		}
		// 双方都是指定源：只要覆盖到同一个源就算冲突
		for _, exist := range s.repo.GetRunSourceKeys(item) {
			for _, want := range keys {
				if exist == want {
					return true, item
				}
			}
		}
	}
	return false, nil
}

// collectOneSource 执行单个源的采集与入库，并回写源健康度与批次明细。
func (s *svcImpl) collectOneSource(ctx context.Context, runID, sourceKey string) error {
	started := NowFunc()
	src, err := s.repo.GetSource(ctx, sourceKey)
	if err != nil {
		return fmt.Errorf("采集源不存在: %w", err)
	}

	detail := &model.TenderIntelRunSource{
		RunID:        runID,
		SourceKey:    sourceKey,
		SourceName:   src.Name,
		Status:       "running",
		CursorBefore: src.Cursor,
	}

	resp, collectErr := s.collector.Collect(ctx, collectorCollectRequest{
		SourceKey:     src.SourceKey,
		ListURL:       src.ListURL,
		DiscoveryMode: src.DiscoveryMode,
		NeedsBrowser:  src.NeedsBrowser == 1,
		Cursor:        src.Cursor,
		MaxPages:      collectMaxPagesPerSource,
		MaxItems:      collectMaxItemsPerSource,
		Params:        src.Params,
	})
	if collectErr != nil {
		detail.Status = "failed"
		detail.Error = collectErr.Error()
		detail.DurationMs = int32(time.Since(started).Milliseconds())
		_ = s.repo.UpsertRunSource(ctx, detail)
		_ = s.repo.SaveSourceHealth(ctx, sourceKey, "", started, false, collectErr.Error())
		_ = s.repo.UpdateRunCounters(ctx, runID, map[string]int{"source_failed": 1})
		s.maybeDispatchEnrich(runID)
		return collectErr
	}

	inserted, skipped := 0, 0
	for _, doc := range resp.Items {
		ok, err := s.persistCollectedDoc(ctx, runID, src, doc)
		if err != nil {
			s.logger.Warnw("公告入库失败", "source_key", sourceKey, "url", doc.URL, "err", err)
			skipped++
			continue
		}
		if ok {
			inserted++
		} else {
			skipped++
		}
	}

	detail.Status = "success"
	detail.CursorAfter = resp.Cursor
	detail.Discovered = int32(len(resp.Items))
	detail.Extracted = int32(len(resp.Items))
	detail.Inserted = int32(inserted)
	detail.Skipped = int32(skipped)
	detail.DurationMs = int32(time.Since(started).Milliseconds())
	if len(resp.Errors) > 0 {
		detail.Error = truncateRunes(fmt.Sprintf("%d 条公告抽取失败", len(resp.Errors)), 500)
	}
	if err := s.repo.UpsertRunSource(ctx, detail); err != nil {
		return err
	}
	_ = s.repo.SaveSourceHealth(ctx, sourceKey, resp.Cursor, started, true, "")
	_ = s.repo.UpdateRunCounters(ctx, runID, map[string]int{
		"source_success":   1,
		"discovered_count": len(resp.Items),
		"extracted_count":  len(resp.Items),
		"inserted_count":   inserted,
		"duplicate_count":  skipped,
	})
	s.maybeDispatchEnrich(runID)
	return nil
}

// maybeDispatchEnrich 在批次内所有源都写回明细后，投递一次打标任务。
//
// 幂等保护：Redis SETNX 保证同一批次只投递一次（worker 可能并发完成多个源）。
func (s *svcImpl) maybeDispatchEnrich(runID string) {
	ctx := context.Background()
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		s.logger.Warnw("读取采集批次失败", "run_id", runID, "err", err)
		return
	}
	if run.FinishedAt != nil {
		return
	}
	total, _, _, err := s.repo.CountRunSourcesByStatus(ctx, runID)
	if err != nil {
		s.logger.Warnw("统计批次源明细失败", "run_id", runID, "err", err)
		return
	}
	if total < int64(run.SourceTotal) {
		// 还有源未完成
		return
	}

	lockKey := "lock:tender_intel:enrich:" + runID
	if s.cache != nil {
		acquired, err := s.cache.Client().SetNX(ctx, lockKey, time.Now().Unix(), 24*time.Hour).Result()
		if err != nil {
			s.logger.Warnw("获取打标投递锁失败", "run_id", runID, "err", err)
			return
		}
		if !acquired {
			return
		}
	}

	if _, err := s.queue.Enqueue(ctx, TaskTypeTenderIntelEnrich, enrichTaskPayload{RunID: runID}, taskqueue.EnqueueOpts{
		TaskID: "intel_enrich_" + runID,
	}); err != nil {
		s.logger.Warnw("投递打标任务失败", "run_id", runID, "err", err)
	}
}

// finalizeRun 按源明细重算批次统计并落终态，保证统计口径与明细一致。
//
// 采集任务失败重试会让累加计数偏高，因此收尾统一以 tender_intel_run_source 为准。
func (s *svcImpl) finalizeRun(ctx context.Context, runID, failSummary string) error {
	summary, err := s.repo.SummarizeRunSources(ctx, runID)
	if err != nil {
		return fmt.Errorf("统计批次源明细失败: %w", err)
	}
	failed := summary.Failed + summary.Pending
	if err := s.repo.SetRunCounters(ctx, runID, map[string]int{
		"source_success":   summary.Success,
		"source_failed":    failed,
		"discovered_count": summary.Discovered,
		"extracted_count":  summary.Extracted,
		"inserted_count":   summary.Inserted,
		"duplicate_count":  summary.Skipped,
	}); err != nil {
		s.logger.Warnw("回写批次统计失败", "run_id", runID, "err", err)
	}

	status := "success"
	detail := ""
	switch {
	case summary.Success == 0:
		// 没有任何源成功：无论明细是否齐全都算失败，避免“空的成功批次”
		status = "failed"
		if failed > 0 {
			detail = fmt.Sprintf("全部源采集失败（共 %d 个源）", failed)
		} else {
			detail = "没有任何源完成采集，请检查采集服务与源配置"
		}
	case failed > 0:
		status = "partial"
		detail = fmt.Sprintf("%d 个源采集失败", failed)
	}
	if failSummary != "" {
		if detail == "" {
			detail = failSummary
		} else {
			detail = detail + "；" + failSummary
		}
	}
	return s.repo.FinishRun(ctx, runID, status, detail)
}

// persistCollectedDoc 将采集文档写入情报库；返回是否有效（false 表示被判为无效内容而跳过）。
//
// runID 会写进情报的 collect_run_id：它既是溯源字段，也是“本次采集新增的这批情报”
// 这个固定匹配范围的查询依据。
func (s *svcImpl) persistCollectedDoc(ctx context.Context, runID string, src *model.TenderIntelSource, doc collectorDoc) (bool, error) {
	title := strings.TrimSpace(doc.Title)
	body := strings.TrimSpace(doc.BodyText)
	if title == "" && body == "" {
		return false, nil
	}
	if title == "" {
		title = truncateRunes(body, 80)
	}

	canonical := CanonicalizeURL(firstNonEmpty(doc.CanonicalURL, doc.URL))
	if canonical == "" {
		return false, fmt.Errorf("缺少有效链接")
	}
	urlHash := HashURL(canonical)
	contentHash := HashContent(body)
	warnings, _ := json.Marshal(doc.Extract.Warnings)

	// 去重：命中时刷新正文与抽取元信息；正文实质变化会使派生结果失效。
	if existing, err := s.repo.GetNoticeByURLHash(ctx, urlHash); err == nil && existing != nil {
		_, err = s.repo.RefreshNoticeContent(ctx, existing.ID, intelRepo.NoticeContentRefresh{
			BodyHTML:          strings.TrimSpace(doc.BodyHTML),
			BodyMarkdown:      doc.BodyMarkdown,
			BodyText:          body,
			ContentHash:       contentHash,
			FetchStrategy:     doc.Extract.Strategy,
			ExtractConfidence: doc.Extract.Confidence,
			ExtractWarnings:   string(warnings),
			SeenAt:            NowFunc(),
		})
		return true, err
	}

	noticeType, stage := ClassifyNoticeType(title, doc.NoticeTypeText)
	if !IsProcurementNotice(noticeType) {
		// 情报站只收录采购阶段公告，非采购类仅在抽取文本无法判断时按 other 落库
		if strings.TrimSpace(doc.NoticeTypeText) != "" {
			return false, nil
		}
	}

	budgetText := strings.TrimSpace(doc.BudgetText)
	budgetAmount := ParseBudgetAmount(budgetText)
	if budgetAmount == nil {
		budgetAmount = ParseBudgetAmount(body)
	}

	regionText := strings.TrimSpace(doc.RegionText)
	province := ExtractProvince(regionText)
	if province == "" {
		province = ExtractProvince(title)
	}
	if province == "" {
		province = ExtractProvince(body)
	}

	attachments, _ := json.Marshal(doc.Attachments)

	notice := &model.TenderIntelNotice{
		SourceKey:         src.SourceKey,
		SourceName:        firstNonEmpty(doc.SourceName, src.Name),
		SourceCategory:    src.Category,
		URL:               doc.URL,
		CanonicalURL:      canonical,
		URLHash:           urlHash,
		ExternalID:        strings.TrimSpace(doc.ExternalID),
		Title:             truncateRunes(title, 240),
		Publisher:         truncateRunes(strings.TrimSpace(doc.PublisherName), 120),
		Agency:            truncateRunes(strings.TrimSpace(doc.AgencyName), 120),
		ProjectCode:       truncateRunes(strings.TrimSpace(doc.ProjectCode), 120),
		BudgetText:        truncateRunes(budgetText, 200),
		BudgetAmount:      budgetAmount,
		RegionProvince:    province,
		RegionText:        truncateRunes(regionText, 120),
		NoticeType:        noticeType,
		NoticeStage:       stage,
		PublishDate:       ParseDate(firstNonEmpty(doc.PublishDate, body)),
		PublishAt:         ParseDateTime(firstNonEmpty(doc.PublishDate, "")),
		DeadlineAt:        ParseDateTime(doc.DeadlineText),
		BodyHTML:          strings.TrimSpace(doc.BodyHTML),
		BodyMarkdown:      doc.BodyMarkdown,
		BodyText:          body,
		ContentHash:       contentHash,
		Attachments:       string(attachments),
		FetchStrategy:     doc.Extract.Strategy,
		ExtractConfidence: doc.Extract.Confidence,
		ExtractWarnings:   string(warnings),
		TagStatus:         "pending",
		Status:            "normal",
		CollectRunID:      runID,
		FirstSeenAt:       NowFunc(),
		LastSeenAt:        NowFunc(),
	}
	if err := s.repo.CreateNotice(ctx, notice); err != nil {
		return false, err
	}
	return true, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
