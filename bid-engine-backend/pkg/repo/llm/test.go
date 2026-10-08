package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TestResult 测试连接结果（供“系统管理-模型配置”测试可用性展示）
type TestResult struct {
	Model   string
	Latency time.Duration
	Reply   string
}

// testDefaultPrompt 极简连通性测试 prompt
const testDefaultPrompt = "Say 'pong'"

// testDefaultTimeout 测试连接默认超时
const testDefaultTimeout = 30 * time.Second

// TestConnection 使用提交的配置做一次真实连通性测试（不落库、不写 LLM 用量记录）。
//   - cfg 为空或 base_url/api_key/model 不完整 → ErrLLMNotConfigured；
//   - prompt 为空时使用极简默认 prompt；timeout<=0 时默认 30s；
//   - ErrEmptyLLMResponse 视为成功（端点可达、鉴权通过，部分模型对极简输入返回空 content）；
//   - 失败按测试专用分类返回友好错误（不含原始细节，避免泄露）。
func (s *svcImpl) TestConnection(ctx context.Context, cfg *ProviderConfig, prompt string, timeout time.Duration) (*TestResult, error) {
	if cfg == nil {
		return nil, ErrLLMNotConfigured
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("%w: 配置不完整（base_url/api_key/model 不能为空）", ErrLLMNotConfigured)
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = testDefaultPrompt
	}
	if timeout <= 0 {
		timeout = testDefaultTimeout
	}

	maxTokens := 8
	req := &ChatRequest{
		Prompt:    prompt,
		MaxTokens: &maxTokens,
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, err := s.chatOnceWithUserConfig(testCtx, cfg, req)
	latency := time.Since(start)
	if err != nil {
		if errors.Is(err, ErrEmptyLLMResponse) {
			return &TestResult{Model: cfg.Model, Latency: latency}, nil
		}
		return nil, classifyLLMError(err)
	}
	if result == nil {
		return &TestResult{Model: cfg.Model, Latency: latency}, nil
	}
	model := strings.TrimSpace(result.Model)
	if model == "" {
		model = cfg.Model
	}
	return &TestResult{Model: model, Latency: latency, Reply: result.Content}, nil
}
