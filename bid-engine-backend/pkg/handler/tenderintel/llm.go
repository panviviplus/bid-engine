package tenderintel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bid-engine/pkg/db/model"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/sysllm"
)

// 情报站 LLM 环节统一走 system_llm_config：按候选顺序调用，失败自动降级到下一个候选。
const (
	tagBatchSize          = 20
	tagBodyExcerptRunes   = 1200
	insightBodyExcerptRun = 6000
)

// callSystemLLM 依次尝试全局模型配置候选，返回首个成功的响应内容。
func (s *svcImpl) callSystemLLM(ctx context.Context, system, prompt string, maxTokens int) (string, *model.SystemLlmConfig, error) {
	candidates, err := s.sysllm.Candidates(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("读取全局模型配置失败: %w", err)
	}
	llmSvc := repoLLM.GetInstance()

	if len(candidates) == 0 {
		// DB 无候选：回退配置文件中的默认 provider
		cfg, item, err := s.sysllm.Resolve(ctx)
		if err != nil {
			return "", nil, err
		}
		content, err := s.chatWithConfig(ctx, llmSvc, cfg, system, prompt, maxTokens)
		return content, item, err
	}

	var lastErr error
	for _, candidate := range candidates {
		cfg := s.sysllm.ToProviderConfig(candidate)
		content, err := s.chatWithConfig(ctx, llmSvc, cfg, system, prompt, maxTokens)
		if err == nil {
			return content, candidate, nil
		}
		lastErr = err
		s.logger.Warnw("情报站 LLM 调用失败，降级到下一个候选配置",
			"config_id", candidate.ID, "model", candidate.Model, "err", err)
	}
	if lastErr == nil {
		lastErr = sysllm.ErrNoCandidate
	}
	return "", nil, lastErr
}

func (s *svcImpl) chatWithConfig(ctx context.Context, llmSvc repoLLM.Service, cfg *repoLLM.ProviderConfig, system, prompt string, maxTokens int) (string, error) {
	req := &repoLLM.ChatRequest{
		System: system,
		Prompt: prompt,
	}
	if maxTokens > 0 {
		req.MaxTokens = &maxTokens
	}
	result, err := llmSvc.ChatOnceWithConfig(ctx, cfg, req)
	if err != nil {
		return "", err
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		return "", fmt.Errorf("模型返回为空")
	}
	return strings.TrimSpace(result.Content), nil
}

// ── 批量打标 ────────────────────────────────────────────────────

// TagResult 单条公告的打标结果。
type TagResult struct {
	Industries   []string `json:"industries"`
	NoticeType   string   `json:"notice_type"`
	Publisher    string   `json:"publisher"`
	Agency       string   `json:"agency"`
	ProjectCode  string   `json:"project_code"`
	BudgetAmount *float64 `json:"budget_amount"`
	Province     string   `json:"province"`
}

type tagPayload struct {
	Index        int    `json:"index"`
	Title        string `json:"title"`
	SourceName   string `json:"source_name"`
	RegionText   string `json:"region_text"`
	NoticeType   string `json:"notice_type_text"`
	BudgetText   string `json:"budget_text"`
	BodyExcerpt  string `json:"body_excerpt"`
	RawPublisher string `json:"publisher"`
}

// TagNotices 对一批公告做行业分类与字段补全，返回 notice_id → 结果。
// 单批超过 tagBatchSize 时由调用方分批。
func (s *svcImpl) TagNotices(ctx context.Context, notices []*model.TenderIntelNotice) (map[int64]*TagResult, error) {
	if len(notices) == 0 {
		return map[int64]*TagResult{}, nil
	}
	payload := make([]tagPayload, 0, len(notices))
	for idx, notice := range notices {
		payload = append(payload, tagPayload{
			Index:        idx,
			Title:        notice.Title,
			SourceName:   notice.SourceName,
			RegionText:   notice.RegionText,
			NoticeType:   notice.NoticeType,
			BudgetText:   notice.BudgetText,
			BodyExcerpt:  truncateRunes(notice.BodyText, tagBodyExcerptRunes),
			RawPublisher: notice.Publisher,
		})
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	system := "你是招标公告结构化分析助手，只输出 JSON，不输出任何解释。"
	prompt := fmt.Sprintf(`请为下面每一条招标公告做行业分类与字段补全。

## 行业枚举（industries 只能取以下 code，可多选，最多 3 个）
%s

## 公告类型枚举（notice_type 只能取以下 code 之一）
open_tender 公开招标 / invite_tender 邀请招标 / negotiation 竞争性磋商 / tender_negotiation 竞争性谈判 /
inquiry 询价 / prequalification 资格预审 / single_source 单一来源 / change 变更澄清 / terminate 终止废标 / other 其他

## 输出格式
只输出 JSON 数组，每个元素对应输入中的一条，字段如下：
[{"index": 0, "industries": ["it_informatization"], "notice_type": "open_tender", "publisher": "采购人全称或空串", "agency": "代理机构或空串", "project_code": "项目编号或空串", "budget_amount": 1234567.0 或 null, "province": "省份全称或空串"}]

## 注意
1. industries 必须来自行业枚举；无法判断时使用 ["other"]
2. budget_amount 单位为元，无预算信息时输出 null
3. province 使用省份全称（如 广东省、北京市），无地区信息时输出空串
4. 不要编造输入中不存在的信息

## 待处理公告（JSON）
%s`, industryCatalogPrompt(), string(payloadJSON))

	content, _, err := s.callSystemLLM(ctx, system, prompt, 0)
	if err != nil {
		return nil, err
	}
	rawItems, err := extractJSONArray(content)
	if err != nil {
		return nil, err
	}

	var parsed []struct {
		Index        int      `json:"index"`
		Industries   []string `json:"industries"`
		NoticeType   string   `json:"notice_type"`
		Publisher    string   `json:"publisher"`
		Agency       string   `json:"agency"`
		ProjectCode  string   `json:"project_code"`
		BudgetAmount *float64 `json:"budget_amount"`
		Province     string   `json:"province"`
	}
	if err := json.Unmarshal(rawItems, &parsed); err != nil {
		return nil, fmt.Errorf("解析打标结果失败: %w", err)
	}

	out := make(map[int64]*TagResult, len(notices))
	for _, item := range parsed {
		if item.Index < 0 || item.Index >= len(notices) {
			continue
		}
		result := &TagResult{
			NoticeType:   item.NoticeType,
			Publisher:    strings.TrimSpace(item.Publisher),
			Agency:       strings.TrimSpace(item.Agency),
			ProjectCode:  strings.TrimSpace(item.ProjectCode),
			BudgetAmount: item.BudgetAmount,
			Province:     strings.TrimSpace(item.Province),
		}
		for _, code := range item.Industries {
			code = strings.TrimSpace(code)
			if IsKnownIndustry(code) {
				result.Industries = append(result.Industries, code)
			}
		}
		if len(result.Industries) == 0 {
			result.Industries = []string{"other"}
		}
		out[notices[item.Index].ID] = result
	}
	return out, nil
}

// ── AI 解读 ─────────────────────────────────────────────────────

// BuildInsight 生成公告的 AI 解读（Markdown），返回内容与使用的模型名。
func (s *svcImpl) BuildInsight(ctx context.Context, notice *model.TenderIntelNotice) (string, string, error) {
	system := "你是资深招投标顾问，输出结构化的中文分析，不输出与任务无关的内容。"
	prompt := fmt.Sprintf(`请基于下面的招标公告内容，输出一份简明的投标决策解读。

## 公告标题
%s

## 采购人
%s

## 预算
%s

## 公告正文（可能被截断）
%s

## 输出要求
使用 Markdown，包含以下四个小节，每节 2-4 条要点，不要编造公告中不存在的信息：
### 是否值得跟进
### 关键资质要求
### 风险点
### 建议动作`, notice.Title, notice.Publisher, notice.BudgetText, truncateRunes(notice.BodyText, insightBodyExcerptRun))

	content, cfg, err := s.callSystemLLM(ctx, system, prompt, 0)
	if err != nil {
		return "", "", err
	}
	modelName := ""
	if cfg != nil {
		modelName = cfg.Model
	}
	return content, modelName, nil
}

// ── 自然语言订阅解析 ────────────────────────────────────────────

// SubscriptionDraft 自然语言订阅解析结果。
type SubscriptionDraft struct {
	Name        string   `json:"name"`
	Keywords    []string `json:"keywords"`
	MatchMode   string   `json:"match_mode"`
	Industries  []string `json:"industries"`
	Regions     []string `json:"regions"`
	NoticeTypes []string `json:"notice_types"`
	BudgetMin   *float64 `json:"budget_min"`
	BudgetMax   *float64 `json:"budget_max"`
	Summary     string   `json:"summary"`
}

// ParseSubscriptionText 将自然语言描述解析为结构化订阅草稿。
func (s *svcImpl) ParseSubscriptionText(ctx context.Context, text string) (*SubscriptionDraft, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("描述不能为空")
	}
	system := "你是招标情报订阅助手，只输出 JSON，不输出解释。"
	prompt := fmt.Sprintf(`请把用户的自然语言订阅需求解析为结构化条件。

## 行业枚举（industries 只能取以下 code）
%s

## 公告类型枚举（notice_types 只能取以下 code）
open_tender / invite_tender / negotiation / tender_negotiation / inquiry / prequalification / single_source / change / terminate

## 输出格式（只输出一个 JSON 对象）
{"name": "订阅名称", "keywords": ["关键词"], "match_mode": "any 或 all", "industries": ["code"], "regions": ["省份全称"], "notice_types": ["code"], "budget_min": 数字或 null, "budget_max": 数字或 null, "summary": "一句话说明该订阅在盯什么"}

## 注意
1. keywords 至少包含一个词，用于匹配公告标题与正文
2. 金额单位为元（例如“100 万以上”→ budget_min = 1000000）
3. 无法确定的可选条件留空数组或 null
4. 与招标采购无关的输入，请输出 {"error": "原因"}

## 用户需求
%s`, industryCatalogPrompt(), text)

	content, _, err := s.callSystemLLM(ctx, system, prompt, 0)
	if err != nil {
		return nil, err
	}
	jsonText, err := extractJSONObject(content)
	if err != nil {
		return nil, err
	}
	var probe struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(jsonText, &probe)
	if strings.TrimSpace(probe.Error) != "" {
		return nil, fmt.Errorf("%s", probe.Error)
	}
	var draft SubscriptionDraft
	if err := json.Unmarshal(jsonText, &draft); err != nil {
		return nil, fmt.Errorf("解析订阅草稿失败: %w", err)
	}
	if strings.TrimSpace(draft.MatchMode) != "all" {
		draft.MatchMode = "any"
	}
	draft.Industries = filterKnownIndustries(draft.Industries)
	draft.NoticeTypes = filterKnownNoticeTypes(draft.NoticeTypes)
	draft.Keywords = trimNonEmpty(draft.Keywords)
	draft.Regions = trimNonEmpty(draft.Regions)
	if len(draft.Keywords) == 0 && len(draft.Industries) == 0 && len(draft.Regions) == 0 && len(draft.NoticeTypes) == 0 {
		return nil, fmt.Errorf("未能从描述中识别出可用的订阅条件")
	}
	return &draft, nil
}

func filterKnownIndustries(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if IsKnownIndustry(item) {
			out = append(out, item)
		}
	}
	return out
}

func filterKnownNoticeTypes(items []string) []string {
	known := map[string]struct{}{
		NoticeTypeOpenTender: {}, NoticeTypeInviteTender: {}, NoticeTypeNegotiation: {},
		NoticeTypeTenderNegotiation: {}, NoticeTypeInquiry: {}, NoticeTypePrequalification: {},
		NoticeTypeSingleSource: {}, NoticeTypeChange: {}, NoticeTypeTerminate: {},
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if _, ok := known[item]; ok {
			out = append(out, item)
		}
	}
	return out
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// truncateRunes 按字符截断文本，避免按字节切断中文。
func truncateRunes(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "……"
}

// extractJSONArray 从模型响应中提取第一个 JSON 数组。
func extractJSONArray(content string) ([]byte, error) {
	return extractJSON(content, '[', ']')
}

// extractJSONObject 从模型响应中提取第一个 JSON 对象。
func extractJSONObject(content string) ([]byte, error) {
	return extractJSON(content, '{', '}')
}

// extractJSON 提取首个平衡的 JSON 片段，容忍模型输出中的 ```json 围栏与前后缀文本。
func extractJSON(content string, open, close byte) ([]byte, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("模型返回为空")
	}
	start := strings.IndexByte(content, open)
	if start < 0 {
		return nil, fmt.Errorf("模型返回中未找到 JSON")
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(content); i++ {
		ch := content[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return []byte(content[start : i+1]), nil
			}
		}
	}
	return nil, fmt.Errorf("模型返回的 JSON 不完整")
}
