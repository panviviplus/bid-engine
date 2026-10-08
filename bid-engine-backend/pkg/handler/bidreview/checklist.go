package bidreview

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
)

// ================================================================
// 审核清单生成：固定清单 + 解析依据映射 + 企业规则库 + 材料齐套(LLM)
// ================================================================

// originMeta 清单项来源元数据（评分项分值等）
type originMeta struct {
	FileID    int64   `json:"file_id,omitempty"`
	FullScore float64 `json:"full_score,omitempty"`
	FieldKey  string  `json:"field_key,omitempty"`
	RuleID    int64   `json:"rule_id,omitempty"`
	Source    string  `json:"source,omitempty"`
	Page      int     `json:"page,omitempty"`
}

type presetChecklistDef struct {
	Key              string
	Dimension        string
	Category         string
	Title            string
	Requirement      string
	ExpectedEvidence string
	Severity         string
}

// presetChecklist 通用审核清单（行业通用口径；招标侧细节由解析依据补强）
var presetChecklist = []presetChecklistDef{
	// ── 合规性 ──
	{"preset:project_name", DimensionCompliance, "一致性", "项目名称一致性", "投标文件中的项目名称应与招标文件完全一致，不得张冠李戴或沿用模板名称。", "投标文件封面/正文中的项目名称", "high"},
	{"preset:tenderer_name", DimensionCompliance, "一致性", "招标人与代理机构一致性", "投标文件引用的招标人/代理机构名称应与招标文件一致。", "投标文件正文中的招标人名称", "medium"},
	{"preset:bid_price", DimensionCompliance, "报价", "投标报价一致性与限价合规", "投标报价大小写金额一致，且不得超过招标最高限价/预算。", "开标一览表、报价表、投标函", "high"},
	{"preset:bid_security", DimensionCompliance, "保证金", "投标保证金合规", "投标保证金金额、形式与缴纳时限满足招标要求，并附缴纳凭证。", "保证金缴纳凭证、银行回单", "high"},
	{"preset:bid_deadline", DimensionCompliance, "时效", "投标截止/开标时间时效", "投标文件签署日期与投标有效期应满足招标要求，不得晚于投标截止时间。", "投标函签署日期、投标有效期声明", "high"},
	{"preset:bidder_name", DimensionCompliance, "主体", "投标人名称与主体材料一致", "投标人名称应与营业执照、资质证书、公章名称完全一致。", "营业执照、资质证书、公章", "high"},
	{"preset:signature_seal", DimensionCompliance, "签章", "签字盖章与授权合规", "法定代表人（或授权代表）签字、单位公章、授权委托书齐备且有效。", "法定代表人身份证明、授权委托书、签章页", "high"},
	{"preset:star_clause", DimensionCompliance, "响应性", "★/▲星号条款逐条响应", "对招标文件标注为实质性要求的星号条款逐条响应，不得出现负偏离。", "技术/商务响应偏离表", "high"},
	{"preset:deviation_table", DimensionCompliance, "响应性", "技术商务偏离表完整性", "技术、商务偏离表按招标要求格式逐条填写，无漏项、无负偏离未说明。", "技术偏离表、商务偏离表", "medium"},
	// ── 完整性 ──
	{"preset:qualification", DimensionCompleteness, "资格材料", "资格审查材料齐套", "营业执照、资质证书、财务报告、业绩证明、社保与信用证明等按招标要求齐套提供且在有效期内。", "资格证明文件清单及附件", "high"},
	{"preset:performance", DimensionCompleteness, "业绩", "类似项目业绩材料完整", "按招标要求的数量与时间范围提供合同/中标通知书/验收证明等业绩材料。", "业绩一览表与证明材料", "medium"},
	{"preset:format_completeness", DimensionCompleteness, "编制要求", "投标文件组成与份数符合要求", "投标文件按招标要求的组成、顺序、份数、装订与封装方式编制。", "目录、装订与封装说明", "medium"},
	{"preset:technical_response", DimensionCompleteness, "技术方案", "技术方案对招标需求全覆盖", "技术方案覆盖招标文件的全部技术规范与服务要求条目，无遗漏章节。", "技术方案正文", "high"},
	{"preset:commercial_response", DimensionCompleteness, "商务条款", "商务条款响应完整", "付款方式、交付期、质保与售后等商务条款均明确响应。", "商务响应表", "medium"},
}

// 暗标（版式）检查项：仅暗标项目生成
var anonymousChecklist = []presetChecklistDef{
	{"format:identity_terms", DimensionFormat, "暗标", "投标人身份信息泄露检查", "暗标评审下，投标文件正文不得出现投标人名称、简称、LOGO、联系人、电话、邮箱、统一社会信用代码等身份信息。", "正文文本层关键词扫描结果", "high"},
	{"format:header_footer", DimensionFormat, "暗标", "页眉页脚与重复版式线索检查", "暗标评审下，页眉、页脚（含解析出的装饰层文本）不得包含投标人身份信息或反复出现的可识别标识。", "PDF 页眉页脚文本（Docling furniture 层）", "medium"},
	{"format:layout_consistency", DimensionFormat, "暗标", "版式一致性检查（扫描页/重复行线索）", "暗标评审下，版式特征应统一、无异常扫描页或贯穿全文的重复标识行，避免暴露投标人身份。", "逐页文本密度与跨页重复行统计", "low"},
}

// BuildChecklist 生成审核清单（重跑时保留人工项）
func (s *svcImpl) buildChecklist(ctx context.Context, proj *model.BidReviewV2Project) (int32, error) {
	snap, err := s.loadFrozenSnapshot(ctx, proj.ID)
	if err != nil {
		return 0, err
	}

	if err := s.repo.DeleteChecklistItemsBySource(ctx, proj.ID,
		[]string{SourcePreset, SourceAnalysis, SourceRule, SourceFormat}); err != nil {
		return 0, err
	}

	rows := make([]*model.BidReviewV2ChecklistItem, 0, 80)
	order := int32(0)
	add := func(item *model.BidReviewV2ChecklistItem) {
		order++
		item.ProjectID = proj.ID
		item.SortOrder = order
		if item.ReviewStatus == "" {
			item.ReviewStatus = CheckPending
		}
		rows = append(rows, item)
	}

	fieldByKey := make(map[string]snapshotField, len(snap.Fields))
	for _, f := range snap.Fields {
		fieldByKey[f.Key] = f
	}

	// 1. 通用清单
	for _, def := range presetChecklist {
		requirement := def.Requirement
		page, quote, fileID := 0, "", int64(0)
		switch def.Key {
		case "preset:bid_price":
			if f, ok := fieldByKey["total_ceiling_amount"]; ok && f.Value != "" {
				requirement = fmt.Sprintf("招标最高限价为 %s；投标报价不得超过该限价，且大小写金额一致。", f.Value)
				page, quote, fileID = f.Page, f.Quote, f.FileID
			}
		case "preset:bid_security":
			if f, ok := fieldByKey["bid_security"]; ok && f.Value != "" {
				requirement = fmt.Sprintf("招标要求的投标保证金：%s；须按要求金额、形式与时限缴纳并附凭证。", f.Value)
				page, quote, fileID = f.Page, f.Quote, f.FileID
			}
		case "preset:bid_deadline":
			if f, ok := fieldByKey["bid_deadline"]; ok && f.Value != "" {
				requirement = fmt.Sprintf("投标截止时间为 %s；投标文件签署日期与投标有效期须满足该时点要求。", f.Value)
				page, quote, fileID = f.Page, f.Quote, f.FileID
			}
		}
		meta, _ := json.Marshal(originMeta{FileID: fileID, Source: SourcePreset, Page: page})
		add(&model.BidReviewV2ChecklistItem{
			ItemKey: def.Key, Dimension: def.Dimension, Category: def.Category, Title: def.Title,
			Requirement: requirement, ExpectedEvidence: def.ExpectedEvidence, Severity: def.Severity,
			Source: SourcePreset, TenderPage: int32(page), TenderQuote: quote, OriginJSON: string(meta),
		})
	}

	// 2. 解析依据映射：否决/高风险条款 → 合规性
	highClauses := make([]snapshotClause, 0, 24)
	for _, c := range snap.Clauses {
		if c.Importance == "high" || containsAny(c.Content+c.Title, []string{"否决", "废标", "无效投标", "不予受理", "取消投标资格"}) {
			highClauses = append(highClauses, c)
		}
	}
	if len(highClauses) > 24 {
		highClauses = highClauses[:24]
	}
	for _, c := range highClauses {
		title := strings.TrimSpace(c.Title)
		if title == "" {
			title = truncateRunes(strings.TrimSpace(c.Content), 40)
		}
		meta, _ := json.Marshal(originMeta{FileID: c.FileID, Source: SourceAnalysis, Page: c.Page})
		add(&model.BidReviewV2ChecklistItem{
			ItemKey:   "clause:" + shortHash(title+c.Content),
			Dimension: DimensionCompliance, Category: "否决项",
			Title:            "否决/实质性条款响应：" + truncateRunes(title, 60),
			Requirement:      firstNonEmpty(strings.TrimSpace(c.Content), strings.TrimSpace(c.Quote), title),
			ExpectedEvidence: "投标文件中对该条款的直接响应内容",
			Severity:         severityFromImportance(c.Importance),
			Source:           SourceAnalysis, TenderPage: int32(c.Page), TenderQuote: c.Quote, OriginJSON: string(meta),
		})
	}

	// 3. 解析依据映射：资格与业绩字段 → 完整性（同一字段多值时合并为一条）
	fieldOrder := make([]string, 0, 8)
	fieldValues := make(map[string][]string, 8)
	fieldRef := make(map[string]snapshotField, 8)
	for _, f := range snap.Fields {
		if f.Category != "qualification_performance" || strings.TrimSpace(f.Value) == "" {
			continue
		}
		identity := f.Key
		if f.FileID > 0 {
			identity = fmt.Sprintf("%d:%s", f.FileID, f.Key)
		}
		if _, ok := fieldValues[identity]; !ok {
			fieldOrder = append(fieldOrder, identity)
			fieldRef[identity] = f
		}
		dup := false
		for _, exist := range fieldValues[identity] {
			if exist == f.Value {
				dup = true
				break
			}
		}
		if !dup {
			fieldValues[identity] = append(fieldValues[identity], f.Value)
		}
	}
	for _, key := range fieldOrder {
		f := fieldRef[key]
		values := fieldValues[key]
		if len(values) > 3 {
			values = values[:3]
		}
		meta, _ := json.Marshal(originMeta{FileID: f.FileID, FieldKey: f.Key, Source: SourceAnalysis, Page: f.Page})
		add(&model.BidReviewV2ChecklistItem{
			ItemKey:   "field:" + key,
			Dimension: DimensionCompleteness, Category: "资格与业绩",
			Title:            "资格/业绩要求核对：" + f.DisplayName,
			Requirement:      fmt.Sprintf("招标要求：%s。请核对投标文件是否提供满足该要求的证明材料。", strings.Join(values, "；")),
			ExpectedEvidence: "对应资格/业绩证明材料",
			Severity:         "high",
			Source:           SourceAnalysis, TenderPage: int32(f.Page), TenderQuote: f.Quote, OriginJSON: string(meta),
		})
	}

	// 4. 评分办法 → 竞争力（逐条对标）
	for idx, row := range snap.ScoringRows {
		full := parseScoreValue(row.Score)
		title := strings.TrimSpace(row.Item)
		if title == "" {
			title = fmt.Sprintf("评分项 %d", idx+1)
		}
		requirement := strings.TrimSpace(row.Criteria)
		if requirement == "" {
			requirement = "按招标评分办法提供对应支撑材料并说明响应。"
		}
		if strings.TrimSpace(row.Response) != "" {
			requirement += "\n响应材料要求：" + strings.TrimSpace(row.Response)
		}
		meta, _ := json.Marshal(originMeta{FileID: row.FileID, FullScore: full, Source: SourceAnalysis, Page: row.Page})
		add(&model.BidReviewV2ChecklistItem{
			ItemKey:   "score:" + shortHash(fmt.Sprintf("%d-%s", idx, title)),
			Dimension: DimensionCompetitiveness, Category: "评分对标",
			Title:            "评分对标：" + truncateRunes(title, 80),
			Requirement:      requirement,
			ExpectedEvidence: firstNonEmpty(strings.TrimSpace(row.Response), "对应的证明材料与说明"),
			Severity:         "medium",
			Source:           SourceAnalysis, TenderPage: int32(row.Page), TenderQuote: row.Quote, OriginJSON: string(meta),
		})
	}

	// 5. 暗标/版式清单（暗标项目）
	if proj.IsAnonymous {
		for _, def := range anonymousChecklist {
			meta, _ := json.Marshal(originMeta{Source: SourceFormat})
			add(&model.BidReviewV2ChecklistItem{
				ItemKey: def.Key, Dimension: def.Dimension, Category: def.Category, Title: def.Title,
				Requirement: def.Requirement, ExpectedEvidence: def.ExpectedEvidence, Severity: def.Severity,
				Source: SourceFormat, OriginJSON: string(meta),
			})
		}
	}

	// 6. 企业规则库
	rules, err := s.repo.ListRules(ctx, proj.UserID, "", true, "")
	if err != nil {
		s.logger.Warnw("读取审核规则库失败", "project_id", proj.ID, "err", err)
	}
	hitRuleIDs := make([]int64, 0, len(rules))
	for _, rule := range rules {
		meta, _ := json.Marshal(originMeta{RuleID: rule.ID, Source: SourceRule})
		add(&model.BidReviewV2ChecklistItem{
			ItemKey:   fmt.Sprintf("rule:%d", rule.ID),
			Dimension: firstNonEmpty(rule.Dimension, DimensionCompliance), Category: firstNonEmpty(rule.Category, "企业规则"),
			Title: rule.Title, Requirement: rule.Requirement, ExpectedEvidence: firstNonEmpty(rule.ExpectedEvidence, "对应证明材料"),
			Severity: firstNonEmpty(rule.Severity, "medium"), Source: SourceRule, RuleID: rule.ID, OriginJSON: string(meta),
		})
		hitRuleIDs = append(hitRuleIDs, rule.ID)
	}
	if len(hitRuleIDs) > 0 {
		if err := s.repo.IncrRuleHit(ctx, hitRuleIDs); err != nil {
			s.logger.Warnw("更新规则命中次数失败", "err", err)
		}
	}

	// 7. 材料齐套清单（LLM，失败降级不阻塞）
	if extra, llmErr := s.generateMaterialChecklist(ctx, snap); llmErr != nil {
		s.logger.Warnw("材料齐套清单生成降级（使用通用清单继续）", "project_id", proj.ID, "err", llmErr)
	} else {
		for _, item := range extra {
			add(item)
		}
	}

	if len(rows) == 0 {
		return 0, fmt.Errorf("未生成任何审核清单项")
	}
	// item_key 在库上唯一：同一批次出现重复键会让多行 INSERT 命中重复，
	// MySQL 8.0.20+/9.x 会直接报 1869（详见 repo 层 INSERT IGNORE 说明），这里先统一去重。
	rows = dedupeChecklistItems(rows)
	if err := s.repo.BatchCreateChecklistItems(ctx, rows); err != nil {
		return 0, fmt.Errorf("保存审核清单失败: %w", err)
	}
	return int32(len(rows)), nil
}

// ── 材料齐套清单（LLM）────────────────────────────────────────────

type materialItem struct {
	Name             string `json:"name"`
	Requirement      string `json:"requirement"`
	ExpectedEvidence string `json:"expected_evidence"`
	Severity         string `json:"severity"`
}

type materialResp struct {
	Items []*materialItem `json:"items"`
}

const materialSystemPrompt = `你是招投标评审专家。请从招标文件中提取“投标人必须随投标文件提交的材料/证明清单”，用于审核投标文件是否齐套。

输出规则：
- 只提取招标文件明确要求的材料、证明、表格、附件（含格式要求、份数、盖章要求）
- name 用简短中文概括（≤ 30 字）
- requirement 写清楚要求（含数量、时间范围、盖章等约束）
- expected_evidence 写清楚应提供的证明材料
- severity：缺项会导致否决/废标的填 high，否则 medium 或 low
- 最多输出 18 项，只输出 JSON，不要解释文字`

const materialUserPrompt = `以下是招标文件的解析要点，请提取必须提交的材料清单。

关键字段：
%s

关键条款：
%s

JSON Schema：
{
  "items": [
    {"name": "材料名称", "requirement": "招标要求", "expected_evidence": "应提供的证明", "severity": "high"}
  ]
}`

func (s *svcImpl) generateMaterialChecklist(ctx context.Context, snap *analysisSnapshot) ([]*model.BidReviewV2ChecklistItem, error) {
	var fieldLines, clauseLines strings.Builder
	for _, f := range snap.Fields {
		if strings.TrimSpace(f.Value) == "" {
			continue
		}
		fmt.Fprintf(&fieldLines, "- %s：%s\n", f.DisplayName, truncateRunes(f.Value, 160))
		if fieldLines.Len() > 6000 {
			break
		}
	}
	for _, c := range snap.Clauses {
		line := strings.TrimSpace(c.Title + " " + truncateRunes(c.Content, 200))
		if line == "" {
			continue
		}
		fmt.Fprintf(&clauseLines, "- %s\n", line)
		if clauseLines.Len() > 12000 {
			break
		}
	}
	if fieldLines.Len() == 0 && clauseLines.Len() == 0 {
		return nil, fmt.Errorf("解析依据为空")
	}

	prompt := fmt.Sprintf(materialUserPrompt, fieldLines.String(), clauseLines.String())
	var resp materialResp
	if err := s.chatJSON(ctx, llmFeatureChecklist, materialSystemPrompt, prompt, 6*time.Minute, &resp); err != nil {
		return nil, err
	}

	out := make([]*model.BidReviewV2ChecklistItem, 0, len(resp.Items))
	seen := map[string]struct{}{}
	for _, it := range resp.Items {
		if it == nil || strings.TrimSpace(it.Name) == "" {
			continue
		}
		key := "material:" + shortHash(it.Name)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		meta, _ := json.Marshal(originMeta{Source: SourceAnalysis})
		out = append(out, &model.BidReviewV2ChecklistItem{
			ItemKey: key, Dimension: DimensionCompleteness, Category: "材料齐套",
			Title:       "材料齐套：" + truncateRunes(strings.TrimSpace(it.Name), 80),
			Requirement: strings.TrimSpace(it.Requirement), ExpectedEvidence: strings.TrimSpace(it.ExpectedEvidence),
			Severity: normalizeSeverity(it.Severity, "medium"), Source: SourceAnalysis, OriginJSON: string(meta),
		})
	}
	return out, nil
}

// ── 小工具 ──────────────────────────────────────────────────────

var scoreNumberRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

// parseScoreValue 从"3分 / 3.5 分 / 合计 20分"中提取分值
func parseScoreValue(raw string) float64 {
	m := scoreNumberRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}

func severityFromImportance(importance string) string {
	switch strings.ToLower(strings.TrimSpace(importance)) {
	case "high", "高":
		return "high"
	case "low", "低":
		return "low"
	default:
		return "medium"
	}
}

func containsAny(text string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

func shortHash(text string) string {
	sum := sha1.Sum([]byte(text))
	return hex.EncodeToString(sum[:])[:10]
}

func truncateRunes(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// dedupeChecklistItems 按 item_key 去重（保留首次出现，顺序稳定）。
// 复用解析依据时（例如同一条款有多条证据、同一字段有多个取值）会出现同名 key，
// 若原样入库，多行 INSERT 会命中同一唯一键，MySQL 8.0.20+/9.x 直接报 1869。
func dedupeChecklistItems(items []*model.BidReviewV2ChecklistItem) []*model.BidReviewV2ChecklistItem {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]*model.BidReviewV2ChecklistItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, dup := seen[item.ItemKey]; dup {
			continue
		}
		seen[item.ItemKey] = struct{}{}
		out = append(out, item)
	}
	return out
}
