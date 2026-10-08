package bidanalysisv3

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	repollm "bid-engine/pkg/repo/llm"
)

type structuredMode uint8

const (
	structuredModeNone structuredMode = iota
	structuredModeStrict
	structuredModeNonStrict
	structuredModeJSONObject
)

type structuredModeCache struct{ values sync.Map }

func newStructuredModeCache() *structuredModeCache { return &structuredModeCache{} }

func (c *structuredModeCache) load(key string) structuredMode {
	if key != "" {
		if value, ok := c.values.Load(key); ok {
			if mode, valid := value.(structuredMode); valid {
				return mode
			}
		}
	}
	return structuredModeStrict
}

func (c *structuredModeCache) store(key string, mode structuredMode) {
	if key != "" && mode != structuredModeNone {
		c.values.Store(key, mode)
	}
}

func nextStructuredMode(current structuredMode, rejection repollm.ResponseFormatRejection) structuredMode {
	if rejection == repollm.ResponseFormatRejectionNone || current == structuredModeJSONObject {
		return structuredModeNone
	}
	if current == structuredModeStrict && rejection == repollm.ResponseFormatStrictUnsupported {
		return structuredModeNonStrict
	}
	return structuredModeJSONObject
}

func structuredFormatForMode(original any, mode structuredMode) map[string]any {
	switch mode {
	case structuredModeStrict:
		format, _ := original.(map[string]any)
		return format
	case structuredModeNonStrict:
		return nonStrictJSONSchemaFormat(original)
	case structuredModeJSONObject:
		return map[string]any{"type": "json_object"}
	default:
		return nil
	}
}

func structuredModeKey(feature string, cfg *repollm.ProviderConfig) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s|%s", feature, cfg.BaseURL, cfg.EndpointPath, cfg.Model)
}

var processStructuredModes = newStructuredModeCache()

func decodeLocalJSON(content string, target any) error {
	if err := decodeStrictJSON(content, target); err == nil {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(stripJSONFence(content)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON 包含额外内容")
	}
	return nil
}
