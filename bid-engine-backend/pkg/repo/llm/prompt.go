package llm

import (
	"bytes"
	"text/template"
)

func FormatPrompt(tpl string, data any) (string, error) {
	t, err := template.New("prompt").Option("missingkey=error").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
