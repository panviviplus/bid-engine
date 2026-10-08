package llm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"bid-engine/lib/common/logtool"
	"go.uber.org/zap"
)

type Provider string

const (
	ProviderQwen     Provider = "qwen"
	ProviderDeepSeek Provider = "deepseek"
)

// healthCheckTimeout LlmConfigAvailable 连通性测试使用的短超时
const healthCheckTimeout = 10 * time.Second

type Service interface {
	ChatOnce(ctx context.Context, provider Provider, req *ChatRequest) (*ChatResult, error)
	ChatOnceStream(ctx context.Context, provider Provider, req *ChatRequest) (<-chan *StreamDelta, <-chan error)
	GetConfig(provider Provider) *ProviderConfig
	ChatOnceByFeature(ctx context.Context, feature string, req *ChatRequest) (*ChatResult, error)
	ChatOnceStreamByFeature(ctx context.Context, feature string, req *ChatRequest) (<-chan *StreamDelta, <-chan error)
	// LlmConfigExist 用户 LLM 配置存在性检查（模块维度，模块→全局策略；无 LLM 调用）
	LlmConfigExist(ctx context.Context, module string) error
	// LlmConfigAvailable 用户 LLM 配置可用性检查（指定模块，极简请求执行一次 LLM 调用；module 为空仅查 module=all）
	LlmConfigAvailable(ctx context.Context, module string) error
	// TestConnection 使用提交的配置做一次真实连通性测试（不落库、不写用量记录；测试专用错误分类）
	TestConnection(ctx context.Context, cfg *ProviderConfig, prompt string, timeout time.Duration) (*TestResult, error)
	// ChatOnceWithConfig 使用外部传入的配置（如 system_llm_config）直接发起一次对话补全。
	// 与用户配置无关，供平台级后台任务使用。
	ChatOnceWithConfig(ctx context.Context, cfg *ProviderConfig, req *ChatRequest) (*ChatResult, error)
}

var (
	instance Service
	once     sync.Once
)

func GetInstance() Service {
	once.Do(func() {
		instance = newService(logtool.GetLogger().Sugar())
	})
	return instance
}

type svcImpl struct {
	logger *zap.SugaredLogger
	mu     sync.RWMutex
	cliMap map[string]*openAIClient
	cfg    *Config
}

func newService(logger *zap.SugaredLogger) Service {
	cfg := LoadConfig()
	return &svcImpl{
		logger: logger,
		cliMap: map[string]*openAIClient{},
		cfg:    cfg,
	}
}

func (s *svcImpl) GetConfig(provider Provider) *ProviderConfig {
	key := strings.TrimSpace(string(provider))
	if key == "" {
		return nil
	}
	s.mu.RLock()
	if c, ok := s.cfg.Providers[key]; ok && c != nil {
		s.mu.RUnlock()
		return c
	}
	s.mu.RUnlock()
	c := loadProviderConfig(key)
	s.mu.Lock()
	s.cfg.Providers[key] = c
	s.mu.Unlock()
	return c
}

func (s *svcImpl) ChatOnce(ctx context.Context, provider Provider, req *ChatRequest) (*ChatResult, error) {
	cli, err := s.client(provider)
	if err != nil {
		return nil, err
	}
	return cli.ChatOnce(ctx, req)
}

// ChatOnceWithConfig 使用传入配置发起一次非流式对话补全。
func (s *svcImpl) ChatOnceWithConfig(ctx context.Context, cfg *ProviderConfig, req *ChatRequest) (*ChatResult, error) {
	if cfg == nil {
		return nil, ErrLLMNotConfigured
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("%w: 配置不完整（base_url/api_key/model 不能为空）", ErrLLMNotConfigured)
	}
	return s.chatOnceWithUserConfig(ctx, cfg, req)
}

func (s *svcImpl) ChatOnceStream(ctx context.Context, provider Provider, req *ChatRequest) (<-chan *StreamDelta, <-chan error) {
	cli, err := s.client(provider)
	if err != nil {
		return newErrChannels(err)
	}
	return cli.ChatOnceStream(ctx, req)
}

// ── feature 级调用（仅认用户配置，无系统兜底）────────────────────

func (s *svcImpl) ChatOnceByFeature(ctx context.Context, feature string, req *ChatRequest) (*ChatResult, error) {
	userCfg, err := s.userConfigOrErr(ctx, feature)
	if err != nil {
		return nil, err
	}
	s.logger.Infow("llm: 使用用户自定义配置",
		"feature", feature,
		"model", userCfg.Model,
		"base_url", userCfg.BaseURL,
	)
	if req == nil {
		return nil, ErrInvalidRequest
	}
	result, err := s.chatOnceWithUserConfig(ctx, userCfg, req)
	if err != nil {
		return nil, classifyLLMError(err)
	}
	// 补充 Provider/Model，确保上游能正确记录
	if result != nil {
		result.Provider = "user"
		if result.Model == "" {
			result.Model = userCfg.Model
		}
	}
	return result, nil
}

func (s *svcImpl) ChatOnceStreamByFeature(ctx context.Context, feature string, req *ChatRequest) (<-chan *StreamDelta, <-chan error) {
	userCfg, err := s.userConfigOrErr(ctx, feature)
	if err != nil {
		return newErrChannels(err)
	}
	s.logger.Infow("llm: 使用用户自定义配置(流式)",
		"feature", feature,
		"model", userCfg.Model,
		"base_url", userCfg.BaseURL,
	)
	if req == nil {
		return newErrChannels(ErrInvalidRequest)
	}
	deltaC, errC := s.chatOnceStreamWithUserConfig(ctx, userCfg, req)
	// 转发并分类错误
	outDelta := make(chan *StreamDelta)
	outErr := make(chan error, 1)
	go func() {
		defer close(outDelta)
		defer close(outErr)
		deltaOpen, errOpen := true, true
		for deltaOpen || errOpen {
			select {
			case d, ok := <-deltaC:
				if !ok {
					deltaOpen = false
					continue
				}
				outDelta <- d
			case err, ok := <-errC:
				if !ok {
					errOpen = false
					continue
				}
				outErr <- classifyLLMError(err)
			}
		}
	}()
	return outDelta, outErr
}

// newErrChannels 构造仅携带一个错误的流式返回通道
func newErrChannels(err error) (<-chan *StreamDelta, <-chan error) {
	deltaC := make(chan *StreamDelta)
	errC := make(chan error, 1)
	close(deltaC)
	errC <- err
	close(errC)
	return deltaC, errC
}

func (s *svcImpl) client(provider Provider) (*openAIClient, error) {
	key := strings.TrimSpace(string(provider))
	if key == "" {
		return nil, ErrProviderNotConfig
	}
	s.mu.RLock()
	if cli, ok := s.cliMap[key]; ok && cli != nil {
		s.mu.RUnlock()
		return cli, nil
	}
	s.mu.RUnlock()

	cfg := s.GetConfig(provider)
	if cfg == nil {
		return nil, ErrProviderNotConfig
	}
	cli := newOpenAIClient(s.logger, provider, cfg)

	s.mu.Lock()
	s.cliMap[key] = cli
	s.mu.Unlock()
	return cli, nil
}

// chatOnceWithUserConfig 使用用户配置创建临时客户端发请求（非流式）
func (s *svcImpl) chatOnceWithUserConfig(ctx context.Context, cfg *ProviderConfig, req *ChatRequest) (*ChatResult, error) {
	var result *ChatResult
	var err error
	if cfg.EndpointPath == "/responses" {
		result, err = chatOnceResponses(ctx, s.logger, cfg, req)
	} else {
		cli := s.userClientForConfig(cfg)
		result, err = cli.ChatOnce(ctx, req)
	}
	// 补充 Provider/Model，确保上游 writeLLM 能正确记录
	if result != nil {
		result.Provider = "user"
		if result.Model == "" {
			result.Model = cfg.Model
		}
	}
	return result, err
}

func (s *svcImpl) userClientForConfig(cfg *ProviderConfig) *openAIClient {
	if cfg == nil {
		return newOpenAIClient(s.logger, "user", &ProviderConfig{})
	}
	fingerprint := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%t\x00%g\x00%g\x00%d\x00%d", cfg.BaseURL, cfg.EndpointPath, cfg.APIKey, cfg.Model, cfg.TimeoutSeconds, cfg.InsecureSkipVerify, cfg.DefaultTemperature, cfg.DefaultTopP, cfg.DefaultMaxTokens, cfg.ContextWindowTokens)
	sum := sha256.Sum256([]byte(fingerprint))
	key := fmt.Sprintf("user:%x", sum[:])
	s.mu.RLock()
	if client := s.cliMap[key]; client != nil {
		s.mu.RUnlock()
		return client
	}
	s.mu.RUnlock()
	copied := *cfg
	client := newOpenAIClient(s.logger, "user", &copied)
	s.mu.Lock()
	if existing := s.cliMap[key]; existing != nil {
		s.mu.Unlock()
		return existing
	}
	s.cliMap[key] = client
	s.mu.Unlock()
	return client
}

// chatOnceStreamWithUserConfig 使用用户配置创建临时客户端发请求（流式）
func (s *svcImpl) chatOnceStreamWithUserConfig(ctx context.Context, cfg *ProviderConfig, req *ChatRequest) (<-chan *StreamDelta, <-chan error) {
	if cfg.EndpointPath == "/responses" {
		return chatOnceStreamResponses(ctx, s.logger, cfg, req)
	}
	cli := s.userClientForConfig(cfg)
	return cli.ChatOnceStream(ctx, req)
}
