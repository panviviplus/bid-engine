package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// LlmConfigExist 用户 LLM 配置存在性检查（模块维度，遵循“模块 → 全局”策略）：
// 先查指定模块的独立配置是否完整（四项均非空），不完整则兜底查 module=all；
// module=all 配置完整即通过；否则返回 ErrLLMNotConfigured。
// 不做任何 LLM 调用；纯 DB 查询。
func (s *svcImpl) LlmConfigExist(ctx context.Context, module string) error {
	cfg := ResolveModuleConfig(ctx, module)
	if cfg == nil {
		return ErrLLMNotConfigured
	}
	return nil
}

// LlmConfigAvailable 用户 LLM 配置可用性检查：校验指定模块（module 参数）的配置，
// 并用一次极简请求（minimal tokens）执行真实 LLM 调用判断是否可用。
//   - module 为空 → 仅检查 module=all（全局）配置；
//   - module 非空 → 整条配置级回退（模块行完整优先，否则回退 module=all）；
//   - 未配置 → ErrLLMNotConfigured；调用失败按超时/Key无权限/不可用分类。
func (s *svcImpl) LlmConfigAvailable(ctx context.Context, module string) error {
	cfg := ResolveModuleConfig(ctx, module)
	if cfg == nil {
		return ErrLLMNotConfigured
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("%w: 配置不完整（base_url/api_key/model 不能为空）", ErrLLMNotConfigured)
	}

	// 极简连通性测试：单 token、短超时，尽可能节省 token
	one := 1
	req := &ChatRequest{
		Prompt:    "ping",
		MaxTokens: &one,
	}
	testCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if _, err := s.chatOnceWithUserConfig(testCtx, cfg, req); err != nil {
		if errors.Is(err, ErrEmptyLLMResponse) {
			// 端点可达、鉴权通过即视为可用（部分模型对空输出返回空 content）
			return nil
		}
		return classifyLLMError(err)
	}
	return nil
}

// userConfigOrErr 解析 feature 对应的用户配置并校验完整性；未配置/不完整返回 ErrLLMNotConfigured。
// 供 ChatOnceByFeature / ChatOnceStreamByFeature 使用（仅认用户配置，无系统兜底）。
func (s *svcImpl) userConfigOrErr(ctx context.Context, feature string) (*ProviderConfig, error) {
	cfg := ResolveConfig(ctx, feature)
	if cfg == nil {
		return nil, ErrLLMNotConfigured
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("%w: 配置不完整（base_url/api_key/model 不能为空）", ErrLLMNotConfigured)
	}
	return cfg, nil
}
