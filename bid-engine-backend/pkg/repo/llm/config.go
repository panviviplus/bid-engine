package llm

import (
	"os"
	"strconv"
	"strings"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/llmcfg"
)

type Config struct {
	Providers map[string]*ProviderConfig
}

type ProviderConfig struct {
	BaseURL            string
	EndpointPath       string
	APIKey             string
	Model              string
	TimeoutSeconds     int
	InsecureSkipVerify bool
	DefaultTemperature float64
	DefaultTopP        float64
	DefaultMaxTokens   int
	// ContextWindowTokens 是模型输入+输出的总上下文预算，用于上游安全分块。
	ContextWindowTokens int
}

func LoadConfig() *Config {
	providers := map[string]*ProviderConfig{}
	providers[string(ProviderQwen)] = loadProviderConfig(string(ProviderQwen))
	providers[string(ProviderDeepSeek)] = loadProviderConfig(string(ProviderDeepSeek))
	return &Config{Providers: providers}
}

func loadProviderConfig(provider string) *ProviderConfig {
	prefixNew := "providers." + provider + "."
	prefixOld := "properties.llm." + provider + "."
	c := &ProviderConfig{}

	c.BaseURL = strings.TrimSpace(llmcfg.Get(prefixNew + "base_url"))
	if c.BaseURL == "" {
		c.BaseURL = strings.TrimSpace(skbcfg.Get(prefixOld + "base_url"))
	}
	c.EndpointPath = strings.TrimSpace(llmcfg.Get(prefixNew + "endpoint_path"))
	if c.EndpointPath == "" {
		c.EndpointPath = strings.TrimSpace(skbcfg.Get(prefixOld + "endpoint_path"))
	}
	c.APIKey = strings.TrimSpace(llmcfg.Get(prefixNew + "api_key"))
	if c.APIKey == "" {
		c.APIKey = strings.TrimSpace(skbcfg.Get(prefixOld + "api_key"))
	}
	c.Model = strings.TrimSpace(llmcfg.Get(prefixNew + "model"))
	if c.Model == "" {
		c.Model = strings.TrimSpace(skbcfg.Get(prefixOld + "model"))
	}

	if v := strings.TrimSpace(os.Getenv("SKB_LLM_" + normalizeProviderEnv(provider) + "_API_KEY")); v != "" {
		c.APIKey = v
	}

	c.TimeoutSeconds = atoi(firstNonEmpty(llmcfg.Get(prefixNew+"timeout_seconds"), skbcfg.Get(prefixOld+"timeout_seconds")))
	c.DefaultMaxTokens = atoi(firstNonEmpty(llmcfg.Get(prefixNew+"default_max_tokens"), skbcfg.Get(prefixOld+"default_max_tokens")))
	c.DefaultTemperature = atof(firstNonEmpty(llmcfg.Get(prefixNew+"default_temperature"), skbcfg.Get(prefixOld+"default_temperature")))
	c.DefaultTopP = atof(firstNonEmpty(llmcfg.Get(prefixNew+"default_top_p"), skbcfg.Get(prefixOld+"default_top_p")))
	c.InsecureSkipVerify = atob(firstNonEmpty(llmcfg.Get(prefixNew+"insecure_skip_verify"), skbcfg.Get(prefixOld+"insecure_skip_verify")))

	applyProviderDefaults(provider, c)

	return c
}

func applyProviderDefaults(provider string, c *ProviderConfig) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if c.BaseURL == "" {
		switch provider {
		case string(ProviderQwen):
			c.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		case string(ProviderDeepSeek):
			c.BaseURL = "https://api.deepseek.com/v1"
		}
	}
	if c.EndpointPath == "" {
		c.EndpointPath = "/chat/completions"
	}
	if c.Model == "" {
		switch provider {
		case string(ProviderQwen):
			c.Model = "qwen-plus"
		case string(ProviderDeepSeek):
			c.Model = "deepseek-chat"
		}
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 60
	}
	if c.DefaultTemperature == 0 {
		c.DefaultTemperature = 0.2
	}
	if c.DefaultTopP == 0 {
		c.DefaultTopP = 0.9
	}
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func normalizeProviderEnv(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range provider {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.ToUpper(b.String())
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func atof(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

func atob(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	return s == "1" || s == "true" || s == "yes" || s == "y"
}
