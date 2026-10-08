package llm

import "strings"

func BuildSingleTurnMessages(system, prompt string) ([]Message, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, ErrInvalidRequest
	}
	var out []Message
	if strings.TrimSpace(system) != "" {
		out = append(out, Message{Role: RoleSystem, Content: system})
	}
	out = append(out, Message{Role: RoleUser, Content: prompt})
	return out, nil
}
