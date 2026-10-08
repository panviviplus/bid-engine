package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ── Responses API 请求结构 ─────────────────────────────────────

type responsesRequest struct {
	Model           string         `json:"model"`
	Input           string         `json:"input"`
	Instructions    string         `json:"instructions,omitempty"`
	Stream          bool           `json:"stream,omitempty"`
	MaxOutputTokens int            `json:"max_output_tokens,omitempty"`
	Text            map[string]any `json:"text,omitempty"`
}

// ── Responses API 响应结构 ─────────────────────────────────────

type responsesResponse struct {
	ID                string                      `json:"id"`
	Object            string                      `json:"object"`
	Model             string                      `json:"model"`
	Status            string                      `json:"status"`
	IncompleteDetails *responsesIncompleteDetails `json:"incomplete_details"`
	Output            []responsesOutput           `json:"output"`
	Usage             *Usage                      `json:"usage"`
}

type responsesIncompleteDetails struct {
	Reason string `json:"reason"`
}

type responsesOutput struct {
	Type    string             `json:"type"`
	Content []responsesContent `json:"content"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (r *responsesResponse) firstText() string {
	if r == nil || len(r.Output) == 0 {
		return ""
	}
	for _, out := range r.Output {
		for _, c := range out.Content {
			if c.Text != "" {
				return c.Text
			}
		}
	}
	return ""
}

func (r *responsesResponse) finishReason() string {
	if r == nil || r.IncompleteDetails == nil {
		return ""
	}
	if r.IncompleteDetails.Reason == "max_output_tokens" {
		return "length"
	}
	return r.IncompleteDetails.Reason
}

// ── 流式 chunk ─────────────────────────────────────────────────

type responsesStreamChunk struct {
	Type   string           `json:"type"`
	Output *responsesOutput `json:"output,omitempty"`
	Usage  *Usage           `json:"usage,omitempty"`
}

// ── ChatRequest → Responses 请求转换 ────────────────────────────

func buildResponsesRequest(req *ChatRequest, stream bool) (*responsesRequest, string, error) {
	if req == nil {
		return nil, "", ErrInvalidRequest
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, "", ErrMissingModel
	}

	// 将 system prompt 作为 instructions，messages/普通 prompt 合并为 input
	instructions := strings.TrimSpace(req.System)
	var inputParts []string

	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			if m.Role == RoleSystem && instructions == "" {
				instructions = m.Content
			} else {
				inputParts = append(inputParts, fmt.Sprintf("[%s]: %s", m.Role, m.Content))
			}
		}
	} else if strings.TrimSpace(req.Prompt) != "" {
		inputParts = append(inputParts, req.Prompt)
	}

	input := strings.Join(inputParts, "\n")
	if input == "" {
		return nil, "", ErrInvalidRequest
	}

	out := &responsesRequest{
		Model:        model,
		Input:        input,
		Instructions: instructions,
		Stream:       stream,
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		out.MaxOutputTokens = *req.MaxTokens
	}
	if req.ResponseFormat != nil {
		// Chat Completions 的 response_format.json_schema 与 Responses 的
		// text.format 结构不同，这里统一转换，业务层只维护一份 Schema。
		var raw map[string]any
		encoded, err := json.Marshal(req.ResponseFormat)
		if err != nil || json.Unmarshal(encoded, &raw) != nil {
			return nil, "", ErrInvalidRequest
		}
		if schema, ok := raw["json_schema"].(map[string]any); ok {
			format := map[string]any{"type": "json_schema"}
			for _, key := range []string{"name", "strict", "schema"} {
				if value, exists := schema[key]; exists {
					format[key] = value
				}
			}
			out.Text = map[string]any{"format": format}
		} else if raw["type"] == "json_object" {
			out.Text = map[string]any{"format": map[string]any{"type": "json_object"}}
		}
	}
	return out, model, nil
}

// ── 非流式调用 ──────────────────────────────────────────────────

func chatOnceResponses(ctx context.Context, logger *zap.SugaredLogger, cfg *ProviderConfig, req *ChatRequest) (*ChatResult, error) {
	rreq, model, err := buildResponsesRequest(req, false)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rreq)
	if err != nil {
		return nil, err
	}

	hc := responsesHTTPClient(cfg)
	endpoint := responsesEndpoint(cfg)

	ctx2, cancel := responsesTimeout(ctx, cfg)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx2, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{Status: resp.StatusCode, Body: string(respBody)}
	}

	var parsed responsesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal responses response failed: %w", err)
	}
	content := strings.TrimSpace(parsed.firstText())
	if content == "" {
		return nil, ErrEmptyLLMResponse
	}

	return &ChatResult{
		Provider:     Provider("user"),
		Model:        model,
		Content:      content,
		FinishReason: parsed.finishReason(),
		Usage:        parsed.Usage,
		Raw:          json.RawMessage(respBody),
	}, nil
}

// ── 流式调用 ────────────────────────────────────────────────────

func chatOnceStreamResponses(ctx context.Context, logger *zap.SugaredLogger, cfg *ProviderConfig, req *ChatRequest) (<-chan *StreamDelta, <-chan error) {
	deltaC := make(chan *StreamDelta, 32)
	errC := make(chan error, 1)

	go func() {
		defer close(deltaC)
		defer close(errC)

		rreq, model, err := buildResponsesRequest(req, true)
		if err != nil {
			errC <- err
			return
		}

		body, err := json.Marshal(rreq)
		if err != nil {
			errC <- err
			return
		}

		hc := responsesHTTPClient(cfg)
		endpoint := responsesEndpoint(cfg)

		ctx2, cancel := responsesTimeout(ctx, cfg)
		defer cancel()

		httpReq, err := http.NewRequestWithContext(ctx2, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			errC <- err
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := hc.Do(httpReq)
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
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var chunk responsesStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				logger.Warnw("解析Responses流式chunk失败", "err", err)
				continue
			}

			var contentDelta string
			if chunk.Output != nil {
				for _, c := range chunk.Output.Content {
					if c.Text != "" {
						contentDelta += c.Text
					}
				}
			}

			d := &StreamDelta{
				Provider:     Provider("user"),
				Model:        model,
				ContentDelta: contentDelta,
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
			errC <- err
		}
	}()

	return deltaC, errC
}

// ── helpers ─────────────────────────────────────────────────────

func responsesEndpoint(cfg *ProviderConfig) string {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return base + "/responses"
}

func responsesHTTPClient(cfg *ProviderConfig) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	return &http.Client{Transport: tr}
}

func responsesTimeout(ctx context.Context, cfg *ProviderConfig) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	sec := cfg.TimeoutSeconds
	if sec <= 0 {
		sec = 300
	}
	return context.WithTimeout(ctx, time.Duration(sec)*time.Second)
}
