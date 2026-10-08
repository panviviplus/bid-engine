// Package confoverride 提供本地覆盖配置文件的合并能力。
//
// 背景：仓库中跟踪的配置文件（conf-local.yml / conf-container.yml）不包含内网地址、
// 自定义域名等环境相关信息；这些内容放在与主配置同目录、同名的覆盖文件里，例如：
//
//	conf-local.yml      -> conf-local.override.yml
//	conf-container.yml  -> conf-container.override.yml
//	conf.yml            -> conf.override.yml
//
// 覆盖文件不纳入版本管理（见 .gitignore），也不随仓库分发；存在时按“覆盖优先”深合并，
// 不存在时完全等同原行为。也可以用环境变量 BID_ENGINE_CONF_OVERRIDE_FILE 显式指定路径。
package confoverride

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Resolve 返回主配置对应的覆盖文件路径；未配置或文件不存在时返回空字符串。
func Resolve(mainPath string) string {
	if p := strings.TrimSpace(os.Getenv("BID_ENGINE_CONF_OVERRIDE_FILE")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
		return ""
	}
	if mainPath == "" {
		return ""
	}
	ext := filepath.Ext(mainPath)
	candidate := strings.TrimSuffix(mainPath, ext) + ".override" + ext
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	return ""
}

// MergeMap 深合并覆盖文件到主配置 map，返回合并结果；无覆盖文件或解析失败时原样返回。
func MergeMap(mainPath string, main map[string]any) map[string]any {
	path := Resolve(mainPath)
	if path == "" || main == nil {
		return main
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return main
	}
	var over map[string]any
	if err := yaml.Unmarshal(data, &over); err != nil {
		fmt.Fprintf(os.Stderr, "本地覆盖配置解析失败，已忽略：%s: %v\n", path, err)
		return main
	}
	return DeepMerge(main, over)
}

// MergeBytes 深合并覆盖文件到主配置内容，返回合并后的 YAML 字节；失败时原样返回。
func MergeBytes(mainPath string, main []byte) []byte {
	if Resolve(mainPath) == "" {
		return main
	}
	var base map[string]any
	if err := yaml.Unmarshal(main, &base); err != nil {
		return main
	}
	merged := MergeMap(mainPath, base)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return main
	}
	return out
}

// DeepMerge 递归合并两个 map，覆盖方的值优先；map 逐层合并，其余类型直接覆盖。
func DeepMerge(base, over map[string]any) map[string]any {
	for k, v := range over {
		if vm, ok := v.(map[string]any); ok {
			if bm, ok := base[k].(map[string]any); ok {
				base[k] = DeepMerge(bm, vm)
				continue
			}
		}
		base[k] = v
	}
	return base
}
