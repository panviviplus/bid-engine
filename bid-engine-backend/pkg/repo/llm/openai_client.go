package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type openAIClient struct {
	logger   *zap.SugaredLogger
	provider Provider
	cfg      *ProviderConfig
	hc       *http.Client
}

func newOpenAIClient(logger *zap.SugaredLogger, provider Provider, cfg *ProviderConfig) *openAIClient {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	// 优先 IPv4、失败再回退 IPv6：规避 VPN/运营商 IPv6 路由损坏导致的
	// “连接假成功、写入即断”（write: socket is not connected），与 curl 默认行为一致。
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, "tcp4", addr)
		if err == nil {
			return conn, nil
		}
		return dialer.DialContext(ctx, "tcp6", addr)
	}
	return &openAIClient{
		logger:   logger,
		cfg:      cfg,
		provider: provider,
		hc: &http.Client{
			Transport: tr,
		},
	}
}

// llmRetryAttempts 单次 LLM 调用对瞬时故障的最大尝试次数（首次 + 退避重试）。
const llmRetryAttempts = 3

// llmRetryBackoff 每次重试前的等待时长，覆盖短时网络抖动、VPN 切换、服务限流与 5xx。
var llmRetryBackoff = []time.Duration{2 * time.Second, 8 * time.Second}

func (c *openAIClient) ChatOnce(ctx context.Context, req *ChatRequest) (*ChatResult, error) {
	client := c
	provider := client.provider

	if provider == "" {
		return nil, ErrProviderNotConfig
	}
	if err := client.validate(); err != nil {
		return nil, err
	}
	creq, model, err := client.buildChatCompletionRequest(req, false)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(creq)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= llmRetryAttempts; attempt++ {
		result, retryable, callErr := client.chatOnceAttempt(ctx, body)
		if callErr == nil {
			if result != nil && result.Model == "" {
				result.Model = model
			}
			return result, nil
		}
		lastErr = callErr
		if !retryable || attempt == llmRetryAttempts {
			break
		}
		if isIntermittentIPRestriction(callErr) {
			c.hc.CloseIdleConnections()
		}
		delay := llmRetryBackoff[attempt-1]
		client.logger.Warnw("LLM 瞬时故障，退避重试",
			"provider", provider, "model", model,
			"attempt", attempt, "max_attempts", llmRetryAttempts, "delay_ms", delay.Milliseconds(),
			"err", callErr)
		select {
		case <-ctx.Done():
			return nil, callErr
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

// chatOnceAttempt 执行一次 LLM 请求，返回结果、该错误是否值得退避重试与错误。
func (c *openAIClient) chatOnceAttempt(ctx context.Context, body []byte) (*ChatResult, bool, error) {
	ctx2, cancel := c.applyTimeout(ctx)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx2, "POST", c.endpointURL(), bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, retryableTransportError(err), err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, retryableTransportError(err), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := &httpStatusError{Status: resp.StatusCode, Body: string(respBody)}
		return nil, retryableLLMResponse(resp.StatusCode, string(respBody)), httpErr
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, false, fmt.Errorf("unmarshal llm response failed: %w", err)
	}
	content := strings.TrimSpace(parsed.firstContent())
	if content == "" {
		return nil, false, ErrEmptyLLMResponse
	}

	return &ChatResult{
		Provider:     c.provider,
		Content:      content,
		FinishReason: parsed.firstFinishReason(),
		Usage:        parsed.Usage,
		Raw:          json.RawMessage(respBody),
	}, false, nil
}

// retryableLLMStatus HTTP 状态码是否属于瞬时故障（限流/服务端错误），可退避重试。
func retryableLLMStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func retryableLLMResponse(status int, body string) bool {
	if retryableLLMStatus(status) {
		return true
	}
	return status == http.StatusForbidden && strings.Contains(strings.ToLower(body), "ip access denied")
}

func isIntermittentIPRestriction(err error) bool {
	var httpErr *httpStatusError
	return errors.As(err, &httpErr) && httpErr.Status == http.StatusForbidden && strings.Contains(strings.ToLower(httpErr.Body), "ip access denied")
}

// retryableTransportError 网络层错误是否值得退避重试：
//   - 上下文取消/请求级超时属于确定性终止，不重试（长模型请求重试只会更慢）；
//   - 连接/DNS/写入失败（如 VPN 抖动、socket 断开）与响应体中断属于瞬时故障，重试。
func retryableTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return !netErr.Timeout()
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *openAIClient) ChatOnceStream(ctx context.Context, req *ChatRequest) (<-chan *StreamDelta, <-chan error) {
	deltaC := make(chan *StreamDelta, 32)
	errC := make(chan error, 1)

	go func() {
		defer close(deltaC)
		defer close(errC)

		client := c
		provider := client.provider
		if provider == "" {
			errC <- ErrProviderNotConfig
			return
		}
		if err := client.validate(); err != nil {
			errC <- err
			return
		}
		creq, model, err := client.buildChatCompletionRequest(req, true)
		if err != nil {
			errC <- err
			return
		}
		body, err := json.Marshal(creq)
		if err != nil {
			errC <- err
			return
		}

		ctx2, cancel := client.applyTimeout(ctx)
		defer cancel()

		httpReq, err := http.NewRequestWithContext(ctx2, "POST", client.endpointURL(), bytes.NewReader(body))
		if err != nil {
			errC <- err
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+client.cfg.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Cache-Control", "no-cache")

		resp, err := client.hc.Do(httpReq)
		if err != nil {
			errC <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			errC <- &httpStatusError{Status: resp.StatusCode, Body: string(b)}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx2.Done():
				errC <- ctx2.Err()
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var chunk chatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				client.logger.Warnw("解析LLM流式chunk失败", "provider", provider, "err", err)
				continue
			}

			contentDelta, finishReason := chunk.firstDelta()
			d := &StreamDelta{
				Provider:     provider,
				Model:        model,
				ContentDelta: contentDelta,
				FinishReason: finishReason,
				Usage:        chunk.Usage,
				Raw:          json.RawMessage([]byte(data)),
			}
			select {
			case deltaC <- d:
			case <-ctx2.Done():
				errC <- ctx2.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				errC <- err
			}
		}
	}()

	return deltaC, errC
}

func (c *openAIClient) validate() error {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return fmt.Errorf("provider=%s: %w", c.provider, ErrMissingBaseURL)
	}
	if strings.TrimSpace(c.cfg.EndpointPath) == "" {
		return fmt.Errorf("provider=%s: %w", c.provider, ErrMissingEndpoint)
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return fmt.Errorf("provider=%s: %w", c.provider, ErrMissingAPIKey)
	}
	return nil
}

func (c *openAIClient) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	sec := c.cfg.TimeoutSeconds
	if sec <= 0 {
		sec = 60
	}
	return context.WithTimeout(ctx, time.Duration(sec)*time.Second)
}

func (c *openAIClient) endpointURL() string {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	path := strings.TrimSpace(c.cfg.EndpointPath)
	if path == "" {
		path = "/chat/completions"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (c *openAIClient) buildChatCompletionRequest(req *ChatRequest, stream bool) (*chatCompletionRequest, string, error) {
	if req == nil {
		return nil, "", ErrInvalidRequest
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(c.cfg.Model)
	}
	if model == "" {
		return nil, "", ErrMissingModel
	}

	messages := req.Messages
	if len(messages) == 0 {
		if strings.TrimSpace(req.System) != "" {
			messages = append(messages, Message{Role: RoleSystem, Content: req.System})
		}
		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" {
			return nil, "", ErrInvalidRequest
		}
		messages = append(messages, Message{Role: RoleUser, Content: prompt})
	}

	temp := c.cfg.DefaultTemperature
	topP := c.cfg.DefaultTopP
	maxTokens := c.cfg.DefaultMaxTokens
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	if req.TopP != nil {
		topP = *req.TopP
	}
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	out := &chatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   stream,
	}
	// 仅下发取值大于 0 的显式参数；0 视为未配置交给服务端默认值。
	// 否则 top_p=0 会触发部分厂商（如 DeepSeek）参数校验失败（合法范围 (0,1]），
	// 也会让“测试连接”这类未设置默认值的请求带上非法 top_p=0。
	if temp > 0 {
		out.Temperature = &temp
	}
	if topP > 0 {
		out.TopP = &topP
	}
	if maxTokens > 0 {
		out.MaxTokens = &maxTokens
	}
	if req.ResponseFormat != nil {
		out.ResponseFormat = req.ResponseFormat
	}
	if len(req.Extra) > 0 {
		out.Extra = req.Extra
	}
	return out, model, nil
}

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	Stream         bool           `json:"stream"`
	Temperature    *float64       `json:"temperature,omitempty"`
	TopP           *float64       `json:"top_p,omitempty"`
	MaxTokens      *int           `json:"max_tokens,omitempty"`
	ResponseFormat any            `json:"response_format,omitempty"`
	Extra          map[string]any `json:"-"`
}

func (r chatCompletionRequest) MarshalJSON() ([]byte, error) {
	type Alias chatCompletionRequest
	base := map[string]any{}
	b, err := json.Marshal(Alias(r))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &base); err != nil {
		return nil, err
	}
	for k, v := range r.Extra {
		if _, exists := base[k]; exists {
			continue
		}
		base[k] = v
	}
	return json.Marshal(base)
}

type chatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *Usage                 `json:"usage"`
}

type chatCompletionChoice struct {
	Index        int                  `json:"index"`
	Message      chatCompletionMsg    `json:"message"`
	FinishReason string               `json:"finish_reason"`
	LogProbs     any                  `json:"logprobs"`
	Delta        *chatCompletionDelta `json:"delta,omitempty"`
}

type chatCompletionMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func (r *chatCompletionResponse) firstContent() string {
	if r == nil || len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

func (r *chatCompletionResponse) firstFinishReason() string {
	if r == nil || len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].FinishReason
}

type chatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []chatCompletionChunkChoice `json:"choices"`
	Usage   *Usage                      `json:"usage"`
}

type chatCompletionChunkChoice struct {
	Index        int                 `json:"index"`
	Delta        chatCompletionDelta `json:"delta"`
	FinishReason string              `json:"finish_reason"`
}

func (c *chatCompletionChunk) firstDelta() (string, string) {
	if c == nil || len(c.Choices) == 0 {
		return "", ""
	}
	return c.Choices[0].Delta.Content, c.Choices[0].FinishReason
}
