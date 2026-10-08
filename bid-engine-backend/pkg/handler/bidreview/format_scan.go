package bidreview

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"bid-engine/pkg/db/model"
)

// ================================================================
// 暗标 / 版式确定性扫描（不依赖大模型，结果可复现）
// ================================================================

var (
	phoneRe   = regexp.MustCompile(`(?:^|[^0-9])(1[3-9][0-9]{9})(?:[^0-9]|$)`)
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	usccRe    = regexp.MustCompile(`[0-9A-HJ-NPQRTUWXY]{18}`)
	siteRe    = regexp.MustCompile(`(?i)(?:https?://|www\.)[A-Za-z0-9.\-/_%?=&#]+`)
	companyRe = regexp.MustCompile(`[\p{Han}A-Za-z0-9（）()]{2,30}(?:有限公司|股份有限公司|集团|事务所|研究院)`)
)

// runFormatScanStage 执行暗标/版式扫描（仅对清单中的 format 维度项产出结论）
func (s *svcImpl) runFormatScanStage(ctx context.Context, proj *model.BidReviewV2Project) (int32, error) {
	items, err := s.repo.GetChecklistItems(ctx, proj.ID)
	if err != nil {
		return 0, err
	}
	formatItems := make([]*model.BidReviewV2ChecklistItem, 0, len(items))
	for _, it := range items {
		if it.Dimension == DimensionFormat {
			formatItems = append(formatItems, it)
		}
	}
	if len(formatItems) == 0 {
		// 非暗标项目：该阶段无实际检查项，正常跳过
		return 0, nil
	}

	files, err := s.repo.GetFilesByProjectAndType(ctx, proj.ID, "bid")
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("审核项目缺少投标文件")
	}
	fileMap := make(map[int64]*model.BidReviewV2File, len(files))
	for _, f := range files {
		fileMap[f.ID] = f
	}

	pages, pageCount, err := s.loadBidPages(ctx, files)
	if err != nil {
		return 0, err
	}
	companyName := ""
	if name, cErr := s.repo.GetCompanyName(ctx, proj.UserCompanyID); cErr == nil {
		companyName = strings.TrimSpace(name)
	}

	done := int32(0)
	for _, item := range formatItems {
		var result verdictResult
		var evidences []*model.BidReviewV2Evidence
		switch item.ItemKey {
		case "format:identity_terms":
			result, evidences = scanIdentityTerms(pages, companyName, fileMap, proj.ID)
		case "format:header_footer":
			result, evidences = s.scanHeaderFooter(ctx, pages, fileMap, proj.ID)
		case "format:layout_consistency":
			result, evidences = scanLayoutConsistency(pages, fileMap, proj.ID, pageCount)
		default:
			continue
		}
		result.Engine = "rule"
		result.Severity = firstNonEmpty(result.Severity, item.Severity)
		if err := s.persistVerdict(ctx, proj, item, result, nil, fileMap, evidences...); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// loadBidPages 载入投标文件全部页面（含文件引用，用于证据定位）
func (s *svcImpl) loadBidPages(ctx context.Context, files []*model.BidReviewV2File) ([]*model.BidReviewV2DocumentPage, int, error) {
	all := make([]*model.BidReviewV2DocumentPage, 0)
	for _, f := range files {
		rows, err := s.repo.GetPagesByFileRange(ctx, f.ID, 1, int32(maxInt32(f.PageCount, 100000)))
		if err != nil {
			return nil, 0, err
		}
		all = append(all, rows...)
	}
	return all, len(all), nil
}

// scanIdentityTerms 身份信息关键词扫描（公司名 + 电话/邮箱/信用代码/网址）
func scanIdentityTerms(pages []*model.BidReviewV2DocumentPage, companyName string, files map[int64]*model.BidReviewV2File, projectID int64) (verdictResult, []*model.BidReviewV2Evidence) {
	evidences := make([]*model.BidReviewV2Evidence, 0, 6)
	hardHits, softHits := 0, 0

	addEvidence := func(p *model.BidReviewV2DocumentPage, quote string, score float64) {
		name := ""
		if f, ok := files[p.FileID]; ok && f != nil {
			name = f.FileName
		}
		evidences = append(evidences, &model.BidReviewV2Evidence{
			ProjectID: projectID, Side: "bid", FileID: p.FileID, FileName: name,
			PageNo: p.PageNo, Quote: truncateRunes(quote, 200), MatchScore: score,
		})
	}

	companyProbe := ""
	if companyName != "" {
		companyProbe = strings.TrimSpace(companyName)
	}
	for _, p := range pages {
		text := strings.TrimSpace(p.Content)
		if text == "" {
			continue
		}
		if companyProbe != "" && strings.Contains(text, companyProbe) {
			hardHits++
			addEvidence(p, companyProbe, 3)
		}
		if m := usccRe.FindString(text); m != "" {
			hardHits++
			addEvidence(p, m, 3)
		}
		if m := companyRe.FindString(text); m != "" {
			softHits++
			addEvidence(p, m, 2)
		}
		if m := phoneRe.FindStringSubmatch(text); len(m) > 1 {
			softHits++
			addEvidence(p, m[1], 2)
		}
		if m := emailRe.FindString(text); m != "" {
			softHits++
			addEvidence(p, m, 2)
		}
		if m := siteRe.FindString(text); m != "" {
			softHits++
			addEvidence(p, m, 1)
		}
		if len(evidences) >= 8 {
			break
		}
	}

	switch {
	case hardHits > 0:
		return verdictResult{
			Status: FindingError, Severity: "high",
			Reason:     fmt.Sprintf("正文中命中投标人身份硬证据 %d 处（投标人名称或统一社会信用代码），暗标评审下存在被识别风险。", hardHits),
			Suggestion: "删除或脱敏相关身份信息（名称、简称、信用代码、公章、签字）后重新导出并复检。",
			Confidence: "high",
		}, evidences
	case softHits > 0:
		return verdictResult{
			Status: FindingWarning, Severity: "medium",
			Reason:     fmt.Sprintf("正文中命中疑似身份信息 %d 处（可能包含公司名、电话、邮箱或网址），需人工确认是否构成暗标泄露。", softHits),
			Suggestion: "逐处确认并清理与投标人身份相关的信息（尤其是联系方式与企业称谓）。",
			Confidence: "medium",
		}, evidences
	default:
		return verdictResult{
			Status: FindingPass, Severity: "low",
			Reason:     "未在投标文件正文中扫描到投标人名称、联系方式或统一社会信用代码等身份信息。",
			Suggestion: "无需整改；如后续替换正文，请重新复检该项。",
			Confidence: "medium",
		}, nil
	}
}

// scanHeaderFooter 页眉/页脚（Docling furniture 层）与跨页重复行检查
func (s *svcImpl) scanHeaderFooter(ctx context.Context, pages []*model.BidReviewV2DocumentPage, files map[int64]*model.BidReviewV2File, projectID int64) (verdictResult, []*model.BidReviewV2Evidence) {
	evidences := make([]*model.BidReviewV2Evidence, 0, 6)
	blocks, err := s.repo.GetBlocksByProjectType(ctx, projectID, "furniture", 50)
	if err != nil {
		s.logger.Warnw("读取页眉页脚块失败", "project_id", projectID, "err", err)
	}
	furnitureCount := 0
	sample := ""
	for _, b := range blocks {
		text := strings.TrimSpace(b.Content)
		if text == "" {
			continue
		}
		furnitureCount++
		if sample == "" {
			sample = text
		}
		name := ""
		if f, ok := files[b.FileID]; ok && f != nil {
			name = f.FileName
		}
		evidences = append(evidences, &model.BidReviewV2Evidence{
			ProjectID: projectID, Side: "bid", FileID: b.FileID, FileName: name,
			PageNo: b.PageNo, Quote: truncateRunes(text, 200), MatchScore: 2,
		})
		if len(evidences) >= 6 {
			break
		}
	}

	repeated := repeatedLineRatio(pages)
	switch {
	case furnitureCount > 0:
		return verdictResult{
			Status: FindingWarning, Severity: "medium",
			Reason:     fmt.Sprintf("检测到页眉/页脚装饰层文本 %d 处（示例：%s），暗标评审下需要确认其中不含投标人身份信息。", furnitureCount, truncateRunes(sample, 60)),
			Suggestion: "清理页眉页脚中的公司名称、项目编号或个人署名后再导出。",
			Confidence: "medium",
		}, evidences
	case repeated.ratio >= 0.5 && repeated.line != "":
		return verdictResult{
			Status: FindingWarning, Severity: "low",
			Reason:     fmt.Sprintf("检测到贯穿全文的重复行（出现于 %.0f%% 页面，示例：%s），可能是页眉页脚或水印线索。", repeated.ratio*100, truncateRunes(repeated.line, 60)),
			Suggestion: "确认该重复行是否为版式水印；暗标项目建议去除。",
			Confidence: "low",
		}, evidences
	default:
		return verdictResult{
			Status: FindingPass, Severity: "low",
			Reason:     "未检测到页眉/页脚身份信息或异常重复版式线索。",
			Suggestion: "无需整改。",
			Confidence: "medium",
		}, nil
	}
}

// scanLayoutConsistency 版式一致性（扫描页占比）
func scanLayoutConsistency(pages []*model.BidReviewV2DocumentPage, files map[int64]*model.BidReviewV2File, projectID int64, pageCount int) (verdictResult, []*model.BidReviewV2Evidence) {
	if pageCount == 0 {
		return verdictResult{
			Status: FindingWarning, Severity: "low",
			Reason: "未解析到投标文件页面，无法完成版式一致性检查。", Suggestion: "请确认投标文件可正常解析。", Confidence: "low",
		}, nil
	}
	emptyPages := make([]*model.BidReviewV2DocumentPage, 0)
	for _, p := range pages {
		if strings.TrimSpace(p.Content) == "" {
			emptyPages = append(emptyPages, p)
		}
	}
	ratio := float64(len(emptyPages)) / float64(pageCount)
	evidences := make([]*model.BidReviewV2Evidence, 0, 4)
	for _, p := range emptyPages {
		if len(evidences) >= 4 {
			break
		}
		name := ""
		if f, ok := files[p.FileID]; ok && f != nil {
			name = f.FileName
		}
		evidences = append(evidences, &model.BidReviewV2Evidence{
			ProjectID: projectID, Side: "bid", FileID: p.FileID, FileName: name,
			PageNo: p.PageNo, Quote: "（该页无文本层，可能为扫描页或图片页）", MatchScore: 1,
		})
	}
	if ratio >= 0.2 {
		return verdictResult{
			Status: FindingWarning, Severity: "low",
			Reason:     fmt.Sprintf("%.0f%% 的页面无文本层（可能为扫描件/图片页），暗标评审下版式特征需人工确认。", ratio*100),
			Suggestion: "对无文本层页面做人工版式检查，确认不含可识别投标人身份的图片或水印。",
			Confidence: "medium",
		}, evidences
	}
	return verdictResult{
		Status: FindingPass, Severity: "low",
		Reason:     "各页文本层完整，未发现异常扫描页。",
		Suggestion: "无需整改。",
		Confidence: "medium",
	}, nil
}

type repeatedLineResult struct {
	line  string
	ratio float64
}

// repeatedLineRatio 统计跨页重复行（长度 >= 6 的同一行出现在 50% 以上页面）
func repeatedLineRatio(pages []*model.BidReviewV2DocumentPage) repeatedLineResult {
	if len(pages) == 0 {
		return repeatedLineResult{}
	}
	counter := map[string]int{}
	for _, p := range pages {
		seen := map[string]struct{}{}
		for _, line := range strings.Split(p.Content, "\n") {
			line = strings.TrimSpace(line)
			if len([]rune(line)) < 6 {
				continue
			}
			if _, dup := seen[line]; dup {
				continue
			}
			seen[line] = struct{}{}
			counter[line]++
		}
	}
	best := ""
	bestCount := 0
	for line, count := range counter {
		if count > bestCount {
			best, bestCount = line, count
		}
	}
	if best == "" {
		return repeatedLineResult{}
	}
	return repeatedLineResult{line: best, ratio: float64(bestCount) / float64(len(pages))}
}

func maxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
