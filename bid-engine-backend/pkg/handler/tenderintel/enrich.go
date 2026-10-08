package tenderintel

import (
	"context"
	"fmt"
	"strings"

	"bid-engine/pkg/db/model"
)

// enrichRun 对一轮采集新入库的公告做规则抽取 + 全局模型批量打标 + 订阅匹配。
//
// 设计要点：打标失败不丢数据——公告保持 tag_status=pending，等待下一轮或手动补打。
func (s *svcImpl) enrichRun(ctx context.Context, runID string) error {
	totalEnriched := 0
	for {
		pending, err := s.repo.ListPendingTagNotices(ctx, enrichBatchSize)
		if err != nil {
			return fmt.Errorf("读取待打标公告失败: %w", err)
		}
		if len(pending) == 0 {
			break
		}
		enriched, failed := s.tagBatch(ctx, pending)
		totalEnriched += enriched
		_ = failed
		if enriched == 0 {
			// 本批全部失败：避免死循环，直接结束本轮，留待下一轮重试
			s.logger.Warnw("本批公告打标全部失败，保留待打标状态", "run_id", runID, "batch", len(pending))
			break
		}
	}

	if totalEnriched > 0 {
		_ = s.repo.UpdateRunCounters(ctx, runID, map[string]int{"enriched_count": totalEnriched})
	}

	// 收尾：按源明细重算批次统计并写入终态
	if err := s.finalizeRun(ctx, runID, ""); err != nil {
		return fmt.Errorf("批次收尾失败: %w", err)
	}
	// 打标完成后再投递订阅匹配任务：匹配需要行业标签参与，且两者已分属不同队列，
	// 匹配不会阻塞采集与打标，也不会被打标拖住（打标失败时由终态回调兜底投递）。
	s.enqueueMatchTaskForRun(ctx, runID)
	return nil
}

// tagBatch 对一批公告打标并写入行业标签；返回成功条数与失败条数。
func (s *svcImpl) tagBatch(ctx context.Context, notices []*model.TenderIntelNotice) (int, int) {
	success, failed := 0, 0
	results := map[int64]*TagResult{}

	if tagged, err := s.TagNotices(ctx, notices); err != nil {
		s.logger.Warnw("批量打标失败，回退关键词兜底打标", "count", len(notices), "err", err)
	} else {
		results = tagged
	}

	for _, notice := range notices {
		result := results[notice.ID]
		industries := []string(nil)
		if result == nil {
			// LLM 不可用或未返回：使用关键词兜底打标，保证公告仍可被行业筛选到
			industries = RankIndustriesByKeyword(notice.Title, truncateRunes(notice.BodyText, tagBodyExcerptRunes))
		} else {
			industries = result.Industries
		}
		weights := make(map[string]int, len(industries))
		for idx, code := range industries {
			weights[code] = len(industries) - idx
		}
		if err := s.repo.ReplaceNoticeIndustries(ctx, notice.ID, industries, weights); err != nil {
			s.logger.Warnw("写入行业标签失败", "notice_id", notice.ID, "err", err)
			failed++
			continue
		}

		updates := map[string]interface{}{
			"tag_status": "done",
		}
		if result == nil {
			updates["tag_status"] = "failed"
		} else {
			if result.Publisher != "" {
				updates["publisher"] = truncateRunes(result.Publisher, 120)
			}
			if result.Agency != "" {
				updates["agency"] = truncateRunes(result.Agency, 120)
			}
			if result.ProjectCode != "" {
				updates["project_code"] = truncateRunes(result.ProjectCode, 120)
			}
			if result.BudgetAmount != nil && notice.BudgetAmount == nil {
				updates["budget_amount"] = *result.BudgetAmount
			}
			if result.Province != "" && notice.RegionProvince == "" {
				updates["region_province"] = result.Province
			}
			if result.NoticeType != "" && result.NoticeType != notice.NoticeType && IsProcurementNotice(result.NoticeType) {
				updates["notice_type"] = result.NoticeType
			}
		}
		if err := s.repo.UpdateNoticeTagResult(ctx, notice.ID, updates); err != nil {
			s.logger.Warnw("回写打标结果失败", "notice_id", notice.ID, "err", err)
			failed++
			continue
		}
		if result == nil {
			failed++
		} else {
			success++
		}
		// 订阅匹配已解耦到独立的 tender_intel_match 队列：这里只负责打标，
		// 打标终态后由 enrichRun 统一投递匹配任务，避免匹配拖慢打标。
	}
	return success, failed
}

// MatchSubscription 判断一条情报是否命中订阅规则。
//
// 匹配语义：关键词与行业互为替代路径，其余条件用于收窄。
//
//		命中 =（关键词命中 或 行业命中）且 地区命中 且 公告类型命中 且 预算命中
//
//	  - 召回组：关键词命中或行业命中，任一命中即通过；两者都未配置时整组跳过。
//	    关键词覆盖标题与正文，match_mode=any 命中任一即可、all 需要全部命中；
//	    行业支持枚举编码、枚举名称与自定义文本（自定义文本还会与标题做包含匹配）。
//	  - 收紧条件：地区、公告类型、预算必须同时满足，未配置的条件跳过。
//	    未识别不等于不匹配：公告类型为 other、公告没有行业标签、
//	    公告未解析出预算金额时，不因该条件淘汰（与既有预算口径一致）。
//	  - 之所以让关键词与行业互为替代：行业标签是打标阶段机器打的粗粒度标签，
//	    拿它当硬过滤会让“关键词没写中、行业也没标中”的订阅直接归零。
func MatchSubscription(sub *model.TenderIntelSubscription, notice *model.TenderIntelNotice, industries []string, haystackTitle, haystackBody string) (bool, []string, string) {
	reasons := make([]string, 0, 4)

	keywords := DecodeStringList(sub.Keywords)
	subIndustries := DecodeStringList(sub.Industries)

	// ── 召回组：关键词命中 或 行业命中 ──
	keywordHit := false
	hitKeywords := make([]string, 0, len(keywords))
	if len(keywords) > 0 {
		hits := make([]string, 0, len(keywords))
		for _, kw := range keywords {
			lower := strings.ToLower(kw)
			if lower == "" {
				continue
			}
			if strings.Contains(haystackTitle, lower) || strings.Contains(haystackBody, lower) {
				hits = append(hits, kw)
			}
		}
		if sub.MatchMode == "all" {
			keywordHit = len(hits) == len(keywords)
		} else {
			keywordHit = len(hits) > 0
		}
		if keywordHit {
			hitKeywords = hits
		}
	}

	industryHit := false
	industryLabels := make([]string, 0, len(subIndustries))
	if len(subIndustries) > 0 {
		industryLabels = matchIndustries(subIndustries, industries, notice.Title)
		industryHit = len(industryLabels) > 0
	}

	switch {
	case len(keywords) > 0 && len(subIndustries) > 0:
		// 两条路径可以互补：任一命中即可进入收紧条件
		if !keywordHit && !industryHit {
			return false, nil, ""
		}
	case len(keywords) > 0:
		if !keywordHit {
			return false, nil, ""
		}
	case len(subIndustries) > 0:
		if !industryHit {
			return false, nil, ""
		}
	default:
		// 召回组完全未配置：跳过本组，仅按收紧条件判定
	}
	if keywordHit {
		reasons = append(reasons, fmt.Sprintf("命中关键词：%s", strings.Join(hitKeywords, "、")))
	}
	if industryHit {
		reasons = append(reasons, fmt.Sprintf("命中行业：%s", strings.Join(industryLabels, "、")))
	}

	// ── 收紧条件：地区 ──
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
			return false, nil, ""
		}
		reasons = append(reasons, fmt.Sprintf("命中地区：%s",
			firstNonEmpty(notice.RegionProvince, notice.RegionCity, notice.RegionText)))
	}

	// ── 收紧条件：公告类型（未识别为 other 时跳过，避免因识别失败误杀） ──
	if types := DecodeStringList(sub.NoticeTypes); len(types) > 0 && isRecognizedNoticeType(notice.NoticeType) {
		hit := false
		for _, item := range types {
			if item == notice.NoticeType {
				hit = true
				break
			}
		}
		if !hit {
			return false, nil, ""
		}
		reasons = append(reasons, fmt.Sprintf("命中公告类型：%s", NoticeTypeName(notice.NoticeType)))
	}

	// ── 收紧条件：预算（未解析出金额时跳过） ──
	if notice.BudgetAmount != nil {
		amount := *notice.BudgetAmount
		if sub.BudgetMin != nil && amount < *sub.BudgetMin {
			return false, nil, ""
		}
		if sub.BudgetMax != nil && amount > *sub.BudgetMax {
			return false, nil, ""
		}
		if sub.BudgetMin != nil || sub.BudgetMax != nil {
			reasons = append(reasons, "命中预算区间")
		}
	}

	if len(reasons) == 0 {
		// 无任何有效条件的规则不应存在（保存时已校验），此处保守返回未命中
		return false, nil, ""
	}
	return true, hitKeywords, strings.Join(reasons, "；")
}

// isRecognizedNoticeType 判断公告类型是否已被识别为具体类型。
//
// “未识别”（other）不等于“与订阅要求的类型不符”，因此类型条件遇到
// other 时跳过，而不是直接淘汰——否则类型识别失败会连带吃掉整条订阅。
func isRecognizedNoticeType(code string) bool {
	if code == "" || code == NoticeTypeOther {
		return false
	}
	_, ok := noticeTypeNames[code]
	return ok
}

func intersect(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(b))
	for _, item := range b {
		set[item] = struct{}{}
	}
	out := make([]string, 0, len(a))
	for _, item := range a {
		if _, ok := set[item]; ok {
			out = append(out, item)
		}
	}
	return out
}

// matchIndustries 判断订阅的行业条件是否命中公告，返回命中的展示名。
//
// 支持三种写法（行业名称无法全量枚举，因此必须兼容用户自定义文本）：
//  1. 枚举编码：与公告行业标签精确比较（如 it_informatization）；
//  2. 枚举名称：先按名称反查编码再比较（如“医疗”→ healthcare）；
//  3. 自定义行业文本：与公告行业名称或公告标题做包含匹配（如“智慧水务”）。
func matchIndustries(entries, noticeCodes []string, title string) []string {
	if len(entries) == 0 {
		return nil
	}
	codeSet := make(map[string]struct{}, len(noticeCodes))
	nameSet := make([]string, 0, len(noticeCodes))
	for _, code := range noticeCodes {
		codeSet[code] = struct{}{}
		nameSet = append(nameSet, IndustryName(code))
	}

	lowerTitle := strings.ToLower(title)
	labels := make([]string, 0, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		// 1. 枚举编码
		if _, ok := codeSet[entry]; ok {
			labels = append(labels, IndustryName(entry))
			continue
		}
		// 2. 枚举名称 → 编码
		if code, ok := IndustryCodeByName(entry); ok {
			if _, hit := codeSet[code]; hit {
				labels = append(labels, IndustryName(code))
				continue
			}
		}
		// 3. 自定义行业文本：与公告行业名称或标题匹配
		lowerEntry := strings.ToLower(entry)
		hit := false
		for _, name := range nameSet {
			if name == "" {
				continue
			}
			lowerName := strings.ToLower(name)
			if strings.Contains(lowerName, lowerEntry) || strings.Contains(lowerEntry, lowerName) {
				hit = true
				break
			}
		}
		if !hit && lowerTitle != "" && strings.Contains(lowerTitle, lowerEntry) {
			hit = true
		}
		if hit {
			labels = append(labels, entry)
		}
	}
	return labels
}
