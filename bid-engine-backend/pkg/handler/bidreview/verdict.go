package bidreview

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"bid-engine/pkg/db/model"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
)

// ================================================================
// 分层判定：高危/关键项逐条深核 + 低风险项按维度批量
// ================================================================

const (
	verdictBatchSize = 5
	verdictTimeout   = 8 * time.Minute
)

type verdictResult struct {
	Status     string
	Severity   string
	Reason     string
	Suggestion string
	Confidence string
	BidPage    int
	BidQuote   string
	Engine     string
	Model      string
	LatencyMS  int64
}

type singleVerdictPayload struct {
	Status     string `json:"status"`
	Severity   string `json:"severity"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
	Confidence string `json:"confidence"`
	BidPage    int    `json:"bid_page"`
	BidQuote   string `json:"bid_quote"`
}

type singleVerdictResp struct {
	Item *singleVerdictPayload `json:"item"`
}

type batchVerdictPayload struct {
	Key        string `json:"key"`
	Status     string `json:"status"`
	Severity   string `json:"severity"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
	Confidence string `json:"confidence"`
	BidPage    int    `json:"bid_page"`
	BidQuote   string `json:"bid_quote"`
}

type batchVerdictResp struct {
	Items []*batchVerdictPayload `json:"items"`
}

const verdictSystemPrompt = `你是投标文件响应性审核专家。你要基于“审核清单项的判定口径”与“投标文件候选页原文”，给出该清单项的审核结论。

判定规则：
- status=pass：投标文件已满足要求，且有明确证据支撑
- status=warning：部分满足或存在疑点，需要人工确认（如表述不完整、材料疑似缺失、金额大小写待核对）
- status=error：明确不满足或存在废标风险（材料缺失、负偏离、名称或金额不符、超过限价、日期超期）
- status=na：该清单项不适用于本项目
- severity：warning/error 必填 high/medium/low；pass 填 low
- confidence：high/medium/low，表示结论把握程度
- bid_page 必须是候选页中出现的页码；找不到依据填 0
- bid_quote 为支撑结论的投标原文片段（不超过 120 字），找不到留空
- 不得编造页码与引文；证据不足时给 warning 并说明需要人工确认
- 只输出 JSON，不要输出解释文字`

const verdictSinglePrompt = `请判定以下单个审核清单项。

【审核维度】%s
【清单项】%s
【判定口径】%s
【期望证据】%s
【招标原文片段】%s（招标页码 %d）

【投标文件候选页】
%s

严格按以下 JSON 输出：
{
  "item": {
    "status": "pass",
    "severity": "low",
    "reason": "判定理由（说明依据了哪一页的什么内容）",
    "suggestion": "整改建议（pass 时可写无需整改或优化建议）",
    "confidence": "high",
    "bid_page": 12,
    "bid_quote": "投标原文支撑片段"
  }
}`

const verdictBatchPrompt = `请逐条判定以下审核清单项，并严格按 JSON 输出。

%s

每条清单项的候选页文本如下：
%s

输出格式：
{
  "items": [
    {"key": "清单项key", "status": "pass", "severity": "low", "reason": "理由", "suggestion": "建议", "confidence": "medium", "bid_page": 12, "bid_quote": "原文片段"}
  ]
}

注意：
- items 必须覆盖全部清单项，key 与输入一一对应
- bid_page 必须是该清单项候选页中出现的页码，找不到依据填 0
- 不得编造页码与引文`

// runVerdictStage 执行判定阶段，返回已判定条目数
func (s *svcImpl) runVerdictStage(ctx context.Context, proj *model.BidReviewV2Project) (int32, error) {
	items, err := s.repo.GetChecklistItems(ctx, proj.ID)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("审核清单为空，无法判定")
	}
	files, err := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "bid")
	if err != nil {
		return 0, err
	}
	fileMap := make(map[int64]*model.BidReviewV2File, len(files))
	for _, f := range files {
		fileMap[f.ID] = f
	}

	deepItems := make([]*model.BidReviewV2ChecklistItem, 0, len(items))
	batchItems := make([]*model.BidReviewV2ChecklistItem, 0, len(items))
	for _, item := range items {
		if item.Dimension == DimensionFormat {
			// 版式/暗标由确定性扫描负责（format_scan）
			continue
		}
		if item.Source == SourceUser {
			// 人工自定义项不进入 AI 判定（保持人工结论），需要时可手动复检
			continue
		}
		if isDeepCheckItem(item) {
			deepItems = append(deepItems, item)
		} else {
			batchItems = append(batchItems, item)
		}
	}

	s.logger.Infow("审核判定开始", "project_id", proj.ID, "deep_items", len(deepItems), "batch_items", len(batchItems))

	total := int32(len(deepItems) + len(batchItems))
	_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageVerdict, map[string]interface{}{
		"total": total, "completed": 0, "progress": 0,
	})

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		firstErr  error
		done      int32
		failedCnt int32
	)
	recordErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		failedCnt++
		if firstErr == nil {
			firstErr = err
		}
	}
	// 判定项较多、耗时以分钟计：每 5 项刷新一次阶段进度，便于前端观察
	reportProgress := func() {
		mu.Lock()
		current := done
		mu.Unlock()
		if current == 0 {
			return
		}
		percent := int32(0)
		if total > 0 {
			percent = current * 100 / total
		}
		_ = s.repo.UpdateStageRun(ctx, proj.ID, bidreviewRepo.StageVerdict, map[string]interface{}{
			"completed": current, "progress": percent,
		})
	}

	for _, item := range deepItems {
		wg.Add(1)
		go func(it *model.BidReviewV2ChecklistItem) {
			defer wg.Done()
			if err := s.verdictOneItem(ctx, proj, it, fileMap); err != nil {
				recordErr(err)
				return
			}
			mu.Lock()
			done++
			reached := done%5 == 0
			mu.Unlock()
			if reached {
				reportProgress()
			}
		}(item)
	}

	for _, group := range groupItemsByDimension(batchItems, verdictBatchSize) {
		wg.Add(1)
		go func(g []*model.BidReviewV2ChecklistItem) {
			defer wg.Done()
			if err := s.verdictBatchItems(ctx, proj, g, fileMap); err != nil {
				recordErr(err)
				return
			}
			mu.Lock()
			done += int32(len(g))
			reached := done%5 < int32(len(g))
			mu.Unlock()
			if reached {
				reportProgress()
			}
		}(group)
	}
	wg.Wait()

	// 单条失败不拖垮整个阶段：只要仍有条目判定成功就继续（失败项保持"待判定"，
	// 用户可单项复检）；全部失败才判定为系统性问题（例如模型配置错误）。
	if failedCnt > 0 {
		s.logger.Warnw("部分检查项判定失败", "project_id", proj.ID, "failed", failedCnt, "done", done, "err", firstErr)
	}
	if firstErr != nil && done == 0 {
		return done, firstErr
	}
	return done, nil
}

// isDeepCheckItem 关键项逐条深核：高风险、否决项、资格/业绩、评分对标
func isDeepCheckItem(item *model.BidReviewV2ChecklistItem) bool {
	if item.Severity == "high" {
		return true
	}
	switch item.Category {
	case "否决项", "资格材料", "资格与业绩", "评分对标", "报价", "保证金", "签章", "一致性":
		return true
	}
	return false
}

// groupItemsByDimension 按维度分组并切分为固定批大小
func groupItemsByDimension(items []*model.BidReviewV2ChecklistItem, size int) [][]*model.BidReviewV2ChecklistItem {
	byDim := map[string][]*model.BidReviewV2ChecklistItem{}
	seen := map[string]struct{}{}
	order := make([]string, 0, len(Dimensions))
	for _, d := range Dimensions {
		order = append(order, d)
		seen[d] = struct{}{}
	}
	for _, item := range items {
		if _, ok := seen[item.Dimension]; !ok {
			order = append(order, item.Dimension)
			seen[item.Dimension] = struct{}{}
		}
		byDim[item.Dimension] = append(byDim[item.Dimension], item)
	}
	out := make([][]*model.BidReviewV2ChecklistItem, 0)
	for _, dim := range order {
		list := byDim[dim]
		for i := 0; i < len(list); i += size {
			end := i + size
			if end > len(list) {
				end = len(list)
			}
			out = append(out, list[i:end])
		}
	}
	return out
}

// verdictOneItem 单条深核
func (s *svcImpl) verdictOneItem(ctx context.Context, proj *model.BidReviewV2Project, item *model.BidReviewV2ChecklistItem, files map[int64]*model.BidReviewV2File) error {
	hits, err := s.recallForItem(ctx, proj.ID, item, files)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		// 未召回候选页：不打扰模型，直接给确定性结论（人工复核）
		return s.persistVerdict(ctx, proj, item, verdictResult{
			Status:     FindingNotFound,
			Severity:   item.Severity,
			Reason:     "未在投标文件中检索到与该清单项相关的候选内容（关键词召回为空），需人工确认是否缺失。",
			Suggestion: "请人工核对投标文件中是否存在对应内容；若确属缺失，请补充材料后复检。",
			Confidence: "low",
			Engine:     "rule",
		}, nil, files)
	}

	prompt := fmt.Sprintf(verdictSinglePrompt,
		bidreviewRepo.DimensionLabel(item.Dimension), item.Title, item.Requirement,
		item.ExpectedEvidence, truncateRunes(item.TenderQuote, 200), item.TenderPage,
		formatHitsForPrompt(hits))

	started := time.Now()
	var resp singleVerdictResp
	if err := s.chatJSON(ctx, llmFeatureVerdict, verdictSystemPrompt, prompt, verdictTimeout, &resp); err != nil {
		return err
	}
	if resp.Item == nil {
		return fmt.Errorf("判定结果为空: %s", item.Title)
	}
	status, severity := normalizeVerdict(resp.Item.Status, resp.Item.Severity)
	result := verdictResult{
		Status: status, Severity: severity,
		Reason: strings.TrimSpace(resp.Item.Reason), Suggestion: strings.TrimSpace(resp.Item.Suggestion),
		Confidence: normalizeConfidence(resp.Item.Confidence), BidPage: resp.Item.BidPage,
		BidQuote: truncateRunes(resp.Item.BidQuote, 200),
		Engine:   "llm", Model: s.reviewModelName(ctx, llmFeatureVerdict),
		LatencyMS: time.Since(started).Milliseconds(),
	}
	return s.persistVerdict(ctx, proj, item, result, hits, files)
}

// verdictBatchItems 低风险批量判定
func (s *svcImpl) verdictBatchItems(ctx context.Context, proj *model.BidReviewV2Project, items []*model.BidReviewV2ChecklistItem, files map[int64]*model.BidReviewV2File) error {
	if len(items) == 0 {
		return nil
	}
	hitsByItem := make(map[int64][]pageHit, len(items))
	var itemLines, pageLines strings.Builder
	for _, item := range items {
		hits, err := s.recallForItem(ctx, proj.ID, item, files)
		if err != nil {
			return err
		}
		hitsByItem[item.ID] = hits
		fmt.Fprintf(&itemLines, "- key=%d | 维度=%s | 清单项=%s | 判定口径=%s | 期望证据=%s\n",
			item.ID, bidreviewRepo.DimensionLabel(item.Dimension), item.Title,
			truncateRunes(item.Requirement, 240), truncateRunes(item.ExpectedEvidence, 120))
		if len(hits) == 0 {
			fmt.Fprintf(&pageLines, "\n[key=%d] 无候选页（召回为空）\n", item.ID)
			continue
		}
		fmt.Fprintf(&pageLines, "\n[key=%d]\n%s\n", item.ID, formatHitsForPrompt(hits))
	}

	started := time.Now()
	var resp batchVerdictResp
	if err := s.chatJSON(ctx, llmFeatureVerdictBatch, verdictSystemPrompt,
		fmt.Sprintf(verdictBatchPrompt, itemLines.String(), pageLines.String()), verdictTimeout, &resp); err != nil {
		return err
	}

	byKey := make(map[string]*batchVerdictPayload, len(resp.Items))
	for _, r := range resp.Items {
		if r != nil && r.Key != "" {
			byKey[r.Key] = r
		}
	}
	modelName := s.reviewModelName(ctx, llmFeatureVerdictBatch)
	latency := time.Since(started).Milliseconds()
	for _, item := range items {
		payload := byKey[itoa(item.ID)]
		hits := hitsByItem[item.ID]
		if payload == nil {
			if len(hits) == 0 {
				if err := s.persistVerdict(ctx, proj, item, verdictResult{
					Status: FindingNotFound, Severity: item.Severity,
					Reason:     "未在投标文件中检索到与该清单项相关的候选内容，需人工确认。",
					Suggestion: "请人工核对投标文件是否缺失对应内容。",
					Confidence: "low", Engine: "rule",
				}, nil, files); err != nil {
					return err
				}
				continue
			}
			if err := s.persistVerdict(ctx, proj, item, verdictResult{
				Status: FindingWarning, Severity: item.Severity,
				Reason:     "批量判定未返回该项结论，需人工确认。",
				Suggestion: "请人工复核该项响应情况。",
				Confidence: "low", Engine: "rule",
			}, hits, files); err != nil {
				return err
			}
			continue
		}
		status, severity := normalizeVerdict(payload.Status, payload.Severity)
		if err := s.persistVerdict(ctx, proj, item, verdictResult{
			Status: status, Severity: severity,
			Reason: strings.TrimSpace(payload.Reason), Suggestion: strings.TrimSpace(payload.Suggestion),
			Confidence: normalizeConfidence(payload.Confidence), BidPage: payload.BidPage,
			BidQuote: truncateRunes(payload.BidQuote, 200),
			Engine:   "llm", Model: modelName, LatencyMS: latency,
		}, hits, files); err != nil {
			return err
		}
	}
	return nil
}

// persistVerdict 落库判定与证据（招标侧 + 投标侧）
func (s *svcImpl) persistVerdict(ctx context.Context, proj *model.BidReviewV2Project, item *model.BidReviewV2ChecklistItem, result verdictResult, hits []pageHit, files map[int64]*model.BidReviewV2File, extraEvidences ...*model.BidReviewV2Evidence) error {
	finding := &model.BidReviewV2Finding{
		ProjectID: proj.ID, ChecklistItemID: item.ID,
		Status: result.Status, Severity: firstNonEmpty(result.Severity, item.Severity),
		Reason: result.Reason, Suggestion: result.Suggestion,
		Confidence: firstNonEmpty(result.Confidence, "medium"),
		Engine:     firstNonEmpty(result.Engine, "llm"),
		Model:      result.Model, LatencyMS: result.LatencyMS,
	}
	evidences := make([]*model.BidReviewV2Evidence, 0, 4)
	if item.TenderPage > 0 || strings.TrimSpace(item.TenderQuote) != "" {
		tenderFileID, tenderFileName := s.tenderFileRef(ctx, proj.ID, tenderEvidenceFileID(item.OriginJSON, 0))
		evidences = append(evidences, &model.BidReviewV2Evidence{
			ProjectID: proj.ID, Side: "tender", FileID: tenderFileID, FileName: tenderFileName,
			PageNo: item.TenderPage, Quote: truncateRunes(item.TenderQuote, 200),
		})
	}
	if result.BidPage > 0 || strings.TrimSpace(result.BidQuote) != "" {
		fileID, fileName := matchHitFile(result.BidPage, result.BidQuote, hits, files)
		evidences = append(evidences, &model.BidReviewV2Evidence{
			ProjectID: proj.ID, Side: "bid", FileID: fileID, FileName: fileName,
			PageNo: int32(result.BidPage), Quote: truncateRunes(result.BidQuote, 200),
		})
	}
	evidences = append(evidences, extraEvidences...)
	var remediation *model.BidReviewV2Remediation
	if result.Status == FindingError || result.Status == FindingWarning || result.Status == FindingNotFound {
		remediation = &model.BidReviewV2Remediation{
			ProjectID: proj.ID, ChecklistItemID: item.ID, FindingID: finding.ID,
			Dimension: item.Dimension, Title: item.Title,
			Suggestion: firstNonEmpty(result.Suggestion, result.Reason),
			Severity:   firstNonEmpty(result.Severity, item.Severity),
			Status:     RemediationTodo,
		}
	}
	// 判定 + 证据 + 整改项一次事务落库（内部串行化并带死锁重试）
	return s.repo.SaveVerdict(ctx, finding, evidences, remediation)
}

// tenderFileRef 返回招标文件引用（用于证据定位）
func (s *svcImpl) tenderFileRef(ctx context.Context, projectID, preferredFileID int64) (int64, string) {
	files, err := s.repo.GetFilesByProjectAndType(ctx, projectID, "tender")
	if err != nil || len(files) == 0 {
		return 0, ""
	}
	for _, file := range files {
		if file.ID == preferredFileID {
			return file.ID, file.FileName
		}
	}
	return files[0].ID, files[0].FileName
}

// matchHitFile 依据模型回填页码匹配召回文件（匹配不到则取最高分命中）
func matchHitFile(page int, quote string, hits []pageHit, files map[int64]*model.BidReviewV2File) (int64, string) {
	for _, h := range hits {
		if int(h.PageNo) == page {
			return h.FileID, h.FileName
		}
	}
	if strings.TrimSpace(quote) != "" {
		probe := truncateRunes(quote, 40)
		for _, h := range hits {
			if strings.Contains(h.Text, probe) {
				return h.FileID, h.FileName
			}
		}
	}
	if len(hits) > 0 {
		return hits[0].FileID, hits[0].FileName
	}
	for id, f := range files {
		return id, f.FileName
	}
	return 0, ""
}

// formatHitsForPrompt 候选页拼装（含页码标记，供模型回填 bid_page）
func formatHitsForPrompt(hits []pageHit) string {
	var sb strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&sb, "【第%d页】%s\n\n", h.PageNo, h.Text)
	}
	return sb.String()
}

// normalizeVerdict 归一化 LLM 返回的 status/severity
func normalizeVerdict(status, severity string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass", "通过", "一致", "ok", "满足":
		return FindingPass, "low"
	case "na", "不适用", "不涉及":
		return FindingNA, "low"
	case "error", "失败", "不一致", "高风险", "fail", "不满足", "缺失":
		return FindingError, normalizeSeverity(severity, "high")
	default:
		return FindingWarning, normalizeSeverity(severity, "medium")
	}
}

func normalizeSeverity(sev, def string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "high", "高":
		return "high"
	case "medium", "中":
		return "medium"
	case "low", "低":
		return "low"
	default:
		return def
	}
}

func normalizeConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
