package llm

import "encoding/json"

type MessageRole string

const (
	RoleSystem MessageRole = "system"
	RoleUser   MessageRole = "user"
	RoleAssist MessageRole = "assistant"
)

type Message struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
}

type ChatRequest struct {
	System         string
	Prompt         string
	Messages       []Message
	Model          string
	Temperature    *float64
	TopP           *float64
	MaxTokens      *int
	ResponseFormat any
	Extra          map[string]any
}

type ChatResult struct {
	Provider     Provider
	Model        string
	Content      string
	FinishReason string
	Usage        *Usage
	Raw          json.RawMessage
}

type StreamDelta struct {
	Provider     Provider
	Model        string
	ContentDelta string
	FinishReason string
	Usage        *Usage
	Raw          json.RawMessage
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
