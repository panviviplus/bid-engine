package llm

import (
	"encoding/json"
	"strings"
)

func ParseJSON[T any](text string) (*T, error) {
	jsonText, err := ExtractJSON(text)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func ExtractJSON(text string) (string, error) {
	clean := strings.TrimSpace(stripCodeFences(text))
	if clean == "" {
		return "", ErrJSONNotFound
	}
	if json.Valid([]byte(clean)) {
		return clean, nil
	}

	obj := extractBalanced(clean, '{', '}')
	if obj != "" && json.Valid([]byte(obj)) {
		return obj, nil
	}
	arr := extractBalanced(clean, '[', ']')
	if arr != "" && json.Valid([]byte(arr)) {
		return arr, nil
	}
	return "", ErrNonJSONContent
}

func stripCodeFences(s string) string {
	ss := strings.TrimSpace(s)
	if !strings.HasPrefix(ss, "```") {
		return s
	}
	ss = strings.TrimPrefix(ss, "```")
	ss = strings.TrimLeft(ss, " \t\r\n")
	if idx := strings.IndexByte(ss, '\n'); idx >= 0 {
		head := strings.TrimSpace(ss[:idx])
		if head == "" || isFenceLang(head) {
			ss = ss[idx+1:]
		}
	}
	ss = strings.TrimSpace(ss)
	ss = strings.TrimSuffix(ss, "```")
	return strings.TrimSpace(ss)
}

func isFenceLang(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "json" || s == "javascript" || s == "js" || s == "" || s == "text"
}

func extractBalanced(s string, open, close byte) string {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inStr = false
			}
			continue
		}

		switch ch {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1])
			}
		}
	}
	return ""
}
