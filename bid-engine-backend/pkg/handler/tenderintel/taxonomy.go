package tenderintel

import (
	"fmt"
	"strings"
)

// IndustryCase 行业枚举项，需与 tender_intel_industry 表的种子数据保持一致。
type IndustryCase struct {
	Code     string
	Name     string
	Keywords []string
}

// IndustryCatalog 行业枚举目录（顺序即打标提示词与兜底规则的顺序）。
var IndustryCatalog = []IndustryCase{
	{"it_informatization", "IT/信息化", []string{"软件", "系统", "平台", "信息化", "数字化", "数据", "人工智能", "大模型", "智能", "云", "网络", "信息", "计算机", "算法", "模型"}},
	{"government", "政务", []string{"政务", "政府", "公共服务", "一网通办", "行政审批"}},
	{"healthcare", "医疗", []string{"医疗", "医院", "卫生", "药", "诊疗", "健康"}},
	{"education", "教育", []string{"教育", "学校", "学院", "大学", "培训", "教学"}},
	{"finance", "金融", []string{"金融", "银行", "保险", "证券", "基金", "结算", "财务"}},
	{"energy", "能源", []string{"电力", "能源", "电网", "石油", "石化", "燃气", "新能源", "光伏", "风电"}},
	{"construction", "建筑", []string{"建筑", "施工", "装修", "市政", "监理", "勘察", "设计院", "工程"}},
	{"transportation", "交通", []string{"交通", "公路", "铁路", "地铁", "机场", "港口", "航运", "道路"}},
	{"manufacturing", "制造", []string{"制造", "设备", "机械", "生产线", "加工", "材料", "钢铁", "化工"}},
	{"telecom", "通信", []string{"通信", "运营商", "基站", "光纤", "5G", "网络建设"}},
	{"water", "水利", []string{"水利", "水务", "供水", "排水", "防汛", "水库"}},
	{"environment", "环保", []string{"环保", "环境", "污水", "固废", "监测", "生态"}},
	{"military", "军队", []string{"部队", "军队", "军工", "武器装备", "国防"}},
	{"other", "其他", nil},
}

// IndustryCodes 返回全部行业编码。
func IndustryCodes() []string {
	out := make([]string, 0, len(IndustryCatalog))
	for _, item := range IndustryCatalog {
		out = append(out, item.Code)
	}
	return out
}

// IsKnownIndustry 判断行业编码是否在枚举内。
func IsKnownIndustry(code string) bool {
	code = strings.TrimSpace(code)
	for _, item := range IndustryCatalog {
		if item.Code == code {
			return true
		}
	}
	return false
}

// IndustryName 返回行业中文名。
func IndustryName(code string) string {
	for _, item := range IndustryCatalog {
		if item.Code == code {
			return item.Name
		}
	}
	return "其他"
}

// IndustryLabel 返回行业展示名：已知编码返回中文名，自定义行业原样返回。
func IndustryLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if IsKnownIndustry(value) {
		return IndustryName(value)
	}
	return value
}

// NormalizeIndustries 规整订阅中的行业条件。
//
// 与“打标用的行业枚举”不同，订阅行业同时支持：
//   - 枚举编码（it_informatization）或枚举名称（IT/信息化、医疗…）
//   - 用户自定义行业文本（如“智慧水务”“气象服务”）
//
// 行业名称不可能全量枚举，因此这里只做去空、去重与长度限制，不做白名单过滤。
func NormalizeIndustries(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if runes := []rune(value); len(runes) > 32 {
			value = string(runes[:32])
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

// IndustryCodeByName 按行业中文名（或包含关系）反查编码。
func IndustryCodeByName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	for _, item := range IndustryCatalog {
		if item.Code == "other" {
			continue
		}
		if item.Name == name || strings.Contains(item.Name, name) || strings.Contains(name, item.Name) {
			return item.Code, true
		}
	}
	return "", false
}

// industryCatalogPrompt 生成打标提示词中的行业枚举说明。
func industryCatalogPrompt() string {
	lines := make([]string, 0, len(IndustryCatalog))
	for _, item := range IndustryCatalog {
		if len(item.Keywords) == 0 {
			lines = append(lines, fmt.Sprintf("- %s（%s）", item.Code, item.Name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s（%s）：%s", item.Code, item.Name, strings.Join(item.Keywords, "、")))
	}
	return strings.Join(lines, "\n")
}

// RankIndustriesByKeyword 关键词兜底打标：命中即计入，未命中任何行业时归为“其他”。
func RankIndustriesByKeyword(title, body string) []string {
	text := title + "\n" + body
	hits := make([]string, 0, 3)
	for _, item := range IndustryCatalog {
		if item.Code == "other" {
			continue
		}
		for _, kw := range item.Keywords {
			if strings.Contains(text, kw) {
				hits = append(hits, item.Code)
				break
			}
		}
		if len(hits) >= 3 {
			break
		}
	}
	if len(hits) == 0 {
		return []string{"other"}
	}
	return hits
}
