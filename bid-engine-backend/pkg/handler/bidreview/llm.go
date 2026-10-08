package bidreview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	repoLLM "bid-engine/pkg/repo/llm"
)

// LLM feature 标识（与 pkg/repo/llm/user_config.go featureToModule 对应）
const (
	llmFeatureChecklist     = "bid_review_checklist"
	llmFeatureTenderExtract = "bid_review_tender_extract"
	llmFeatureVerdict       = "bid_review_verdict"
	llmFeatureVerdictBatch  = "bid_review_verdict_batch"
	llmFeatureScoring       = "bid_review_scoring"
)

// 各功能输出上限（token）与下限：审核链路的输出规模可预期，绝不直接信任用户配置。
const (
	reviewCapChecklist    = 8192
	reviewCapVerdict      = 6144
	reviewCapVerdictBatch = 6144
	reviewCapScoring      = 6144
	reviewMinOutput       = 512
)

// nonRetryableError 标记不可重试错误（Worker 识别后不再退避重试，直接进终态）。
type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string { return e.err.Error() }
func (e *nonRetryableError) Unwrap() error { return e.err }
func (e *nonRetryableError) NonRetryable() bool {
	return true
}

// asNonRetryable 包成不可重试错误（nil 原样返回）
func asNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &nonRetryableError{err: err}
}

// reviewFeatureCap 功能级输出上限
func reviewFeatureCap(feature string) int {
	switch feature {
	case llmFeatureChecklist:
		return reviewCapChecklist
	case llmFeatureTenderExtract:
		return reviewCapChecklist
	case llmFeatureVerdict:
		return reviewCapVerdict
	case llmFeatureVerdictBatch:
		return reviewCapVerdictBatch
	case llmFeatureScoring:
		return reviewCapScoring
	default:
		return reviewCapVerdict
	}
}

// reviewMaxOutput 计算本次调用的 max_tokens：
// 取 min(用户配置, 功能上限, 审核模块硬顶)，并保证不低于下限。
// 这样即使模型配置里写了 900000（厂商上限 65536），也不会把整个流水线打死。
func (s *svcImpl) reviewMaxOutput(ctx context.Context, feature string) int {
	capTokens := reviewFeatureCap(feature)
	if s.maxOutputCeiling > 0 && s.maxOutputCeiling < capTokens {
		capTokens = s.maxOutputCeiling
	}
	configured := 0
	if cfg := repoLLM.ResolveConfig(ctx, feature); cfg != nil {
		configured = cfg.DefaultMaxTokens
	}
	out := capTokens
	if configured > 0 && configured < capTokens {
		out = configured
	}
	if out < reviewMinOutput {
		out = reviewMinOutput
	}
	if out > reviewMinOutput && out > capTokens {
		out = capTokens
	}
	return out
}

// reviewModelName 当前 feature 使用的模型名（用于判定留痕）
func (s *svcImpl) reviewModelName(ctx context.Context, feature string) string {
	if cfg := repoLLM.ResolveConfig(ctx, feature); cfg != nil {
		return cfg.Model
	}
	return ""
}

// cleanLLMJSONResponse 提取并净化 LLM 返回的 JSON（去除代码围栏与前后缀）
func cleanLLMJSONResponse(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx > 0 {
		content = content[idx:]
	}
	if last := strings.LastIndex(content, "}"); last >= 0 && last < len(content)-1 {
		content = content[:last+1]
	}
	return strings.TrimSpace(content)
}

// chatJSON 审核链路统一 LLM 调用：并发受控 + 显式输出上限 + 错误分类。
// 返回的“提供方拒绝请求”类错误会被标记为不可重试。
func (s *svcImpl) chatJSON(ctx context.Context, feature, system, prompt string, timeout time.Duration, out any) error {
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	if s.llmSem != nil {
		select {
		case s.llmSem <- struct{}{}:
			defer func() { <-s.llmSem }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	maxTokens := s.reviewMaxOutput(ctx, feature)
	temp := 0.1
	req := &repoLLM.ChatRequest{
		System:      system,
		Prompt:      prompt,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := s.llm.ChatOnceByFeature(callCtx, feature, req)
	if err != nil {
		return s.classifyReviewLLMError(err)
	}
	if res == nil || strings.TrimSpace(res.Content) == "" {
		return fmt.Errorf("LLM 返回空响应")
	}
	clean := cleanLLMJSONResponse(res.Content)
	if err := json.Unmarshal([]byte(clean), out); err != nil {
		// JSON 结构问题属于内容质量，重试没有意义，但换一次容错解析仍可能有救。
		if ferr := json.Unmarshal([]byte(cleanLLMJSONResponse(repairJSON(clean))), out); ferr != nil {
			return asNonRetryable(fmt.Errorf("LLM JSON 解析失败: %w", err))
		}
	}
	return nil
}

// classifyReviewLLMError 错误分类：提供方确定性拒绝 → 不可重试 + 精准文案；其余保持原样。
func (s *svcImpl) classifyReviewLLMError(err error) error {
	if err == nil {
		return nil
	}
	if repoLLM.IsProviderRejection(err) {
		detail := strings.TrimSpace(repoLLM.FriendlyMessage(err))
		if !strings.Contains(err.Error(), "max_tokens") && !strings.Contains(err.Error(), "context") {
			return asNonRetryable(fmt.Errorf("%w（%s）", repoLLM.ErrLLMRequestRejected, detail))
		}
		return asNonRetryable(fmt.Errorf("%w（%s）", repoLLM.ErrLLMRequestRejected, detail))
	}
	return err
}

// repairJSON 轻量修复（尾逗号、单引号包裹的键）后重试一次
func repairJSON(raw string) string {
	raw = strings.ReplaceAll(raw, ",}", "}")
	raw = strings.ReplaceAll(raw, ",]", "]")
	raw = strings.ReplaceAll(raw, "，", ",")
	raw = strings.ReplaceAll(raw, "\n", " ")
	return raw
}
