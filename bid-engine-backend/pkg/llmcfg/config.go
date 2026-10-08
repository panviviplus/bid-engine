package llmcfg

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"bid-engine/lib/common/logtool"
	"gopkg.in/yaml.v3"
)

type store struct {
	root any
}

var (
	once  sync.Once
	s     *store
	loadE error
)

func Get(key string) string {
	if key == "" {
		return ""
	}
	st := getStore()
	if st == nil {
		return ""
	}
	if v, ok := st.getByPath(key); ok {
		return toString(v)
	}
	if idx := strings.LastIndex(key, "."); idx >= 0 && idx+1 < len(key) {
		leaf := key[idx+1:]
		if v, ok := st.searchLeaf(leaf); ok {
			return toString(v)
		}
	}
	if v, ok := st.searchLeaf(key); ok {
		return toString(v)
	}
	return ""
}

func getStore() *store {
	once.Do(func() {
		s, loadE = load()
		if loadE != nil {
			logtool.GetLogger().Sugar().Warnw("加载llm-config.yml失败", "err", loadE)
		}
	})
	if loadE != nil {
		return nil
	}
	return s
}

func load() (*store, error) {
	path := os.Getenv("SKB_LLM_CONF_FILE")
	if path == "" {
		if dir := os.Getenv("SKB_CONF_DIR"); dir != "" {
			p := strings.TrimRight(dir, "/") + "/llm-config.yml"
			if _, err := os.Stat(p); err == nil {
				path = p
			}
		}
	}
	if path == "" {
		candidates := []string{"/service/conf/llm-config.yml", "./conf/llm-config.yml", "/conf/llm-config.yml"}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("未找到LLM配置文件: 设置环境变量SKB_LLM_CONF_FILE，或把文件llm-config.yml挂载到如下路径：./conf/llm-config.yml, /service/conf/llm-config.yml, 或/conf/llm-config.yml")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("未找到LLM配置文件：%s: 设置环境变量SKB_LLM_CONF_FILE，或把文件llm-config.yml挂载到如下路径：./conf/llm-config.yml, /service/conf/llm-config.yml, 或/conf/llm-config.yml", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &store{root: m}, nil
}

func (s *store) getByPath(path string) (any, bool) {
	segs := strings.Split(path, ".")
	cur := s.root
	for _, seg := range segs {
		switch v := cur.(type) {
		case map[string]any:
			nxt, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = nxt
		default:
			return nil, false
		}
	}
	return cur, true
}

func (s *store) searchLeaf(name string) (any, bool) {
	return searchLeaf(name, s.root)
}

func searchLeaf(name string, node any) (any, bool) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if k == name {
				return child, true
			}
			if got, ok := searchLeaf(name, child); ok {
				return got, true
			}
		}
	case []any:
		for _, child := range v {
			if got, ok := searchLeaf(name, child); ok {
				return got, true
			}
		}
	}
	return nil, false
}

func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}
