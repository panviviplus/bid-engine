package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"bid-engine/lib/common/confoverride"
)

type store struct {
	root any
}

var (
	once  sync.Once
	s     *store
	loadE error
)

// Get 读取配置，支持点号路径（如 "wps.app_ak"），也支持纯叶子键（如 "app_ak"）
func Get(key string) string {
	if key == "" {
		return ""
	}
	st := getStore()
	if st == nil {
		return ""
	}
	// 先按路径查找
	if v, ok := st.getByPath(key); ok {
		return toString(v)
	}
	// 新增：当 key 为点路径时，按最后一段作为叶子键递归搜索（兼容不同分组布局）
	if idx := strings.LastIndex(key, "."); idx >= 0 && idx+1 < len(key) {
		leaf := key[idx+1:]
		if v, ok := st.searchLeaf(leaf); ok {
			return toString(v)
		}
	}
	// 回退：兼容纯叶子键输入（如直接传 "app_ak"）
	if v, ok := st.searchLeaf(key); ok {
		return toString(v)
	}
	return ""
}

func getStore() *store {
	once.Do(func() {
		s, loadE = load()
	})
	if loadE != nil {
		return nil
	}
	return s
}

func load() (*store, error) {
	path := os.Getenv("BID_ENGINE_CONF_FILE")
	// 允许通过目录级环境变量指定 conf 目录（如 /service/conf 或 /conf）
	if path == "" {
		if dir := os.Getenv("BID_ENGINE_CONF_DIR"); dir != "" {
			p := strings.TrimRight(dir, "/") + "/conf-local.yml"
			if _, err := os.Stat(p); err == nil {
				path = p
			}
		}
	}
	if path == "" {
		// 优先相对路径，兼容 WORKDIR /service；再尝试常见绝对路径
		candidates := []string{"/service/conf/conf.yml", "./conf/conf-local.yml", "/conf/conf.yml"}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	// 到这里仍为空或文件不存在，返回明确的错误
	if path == "" {
		return nil, fmt.Errorf("未找到配置文件: 设置环境变量BID_ENGINE_CONF_FILE，或把配置文件放到如下路径：./conf/conf-local.yml（本地）, /service/conf/conf.yml（容器）, 或/conf/conf.yml")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("未找到配置文件：%s: 设置环境变量BID_ENGINE_CONF_FILE，或把配置文件放到如下路径：./conf/conf-local.yml（本地）, /service/conf/conf.yml（容器）, 或/conf/conf.yml", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	// 合并本地覆盖配置（conf-*.override.yml，不纳入版本管理），用于内网地址等环境相关配置
	m = confoverride.MergeMap(path, m)
	return &store{root: m}, nil
}

// getByPath 按点号路径遍历，支持 map 和 slice（slice 需要数字下标）
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
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(v) {
				return nil, false
			}
			cur = v[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// searchLeaf 在整棵树里按叶子键名递归查找
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
