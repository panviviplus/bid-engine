// Package bidparse 提供标书相关共享解析/后处理能力（招标解析与投标文件生成共用）。
package bidparse

import (
	"regexp"
	"strconv"
	"strings"
)

// OutlineItem 后处理输入项：文档顺序的标题（含层级，1-4）。
type OutlineItem struct {
	Title string
	Level int32
}

// 标题开头的编号前缀样式（按优先级匹配，循环剥除直到稳定）
var (
	reChapterCN = regexp.MustCompile(`^第[一二三四五六七八九十百千万零〇]+(?:章|节|篇|部分)\s*`) // 第一章 / 第一篇 / 第一部分
	reSectionD  = regexp.MustCompile(`^第\d+(\.\d+)*节\s*`)                   // 第1节 / 第1.1节 / 第1.1.1节
	reSectionCN = regexp.MustCompile(`^第[一二三四五六七八九十百千万零〇]+节\s*`)            // 第一节
	reDottedNum = regexp.MustCompile(`^\d+(\.\d+)*[\.、．]?\s*`)              // 1. / 1.1 / 1、
	reCNNum     = regexp.MustCompile(`^[一二三四五六七八九十百千万]+[、．.]\s*`)           // 一、 / 二．
	reBracket   = regexp.MustCompile(`^[（(][一二三四五六七八九十\d]+[)）]\s*`)         // （一）
	rePrefixes  = []*regexp.Regexp{reChapterCN, reSectionD, reSectionCN, reDottedNum, reCNNum, reBracket}
)

// applyOutlineNumbers 按层级重新生成规范章节序号并拼接到标题（文档顺序、幂等重算）。
// chapterLevel 指定“章”对应的层级：
//   - NormalizeOutlineTitles 用 chapterLevel=1（L1 即章）；
//   - NormalizeBlueprintTitles 用 chapterLevel=2（L1 为文档标题根，L2 起为章）。
//
// 编号规则：章用“第{中文序数}章”，章下各级用点分序号（1.1、1.1.1、…），
// 层级低于 chapterLevel 的节点视为文档标题根，不参与编号。
//
// 标题开头的旧编号前缀会被统一剥除。
func applyOutlineNumbers(items []OutlineItem, chapterLevel int32) []OutlineItem {
	out := make([]OutlineItem, len(items))
	copy(out, items)
	counters := make([]int, 0, 6)
	for i := range out {
		lvl := int(out[i].Level)
		if lvl < int(chapterLevel) {
			continue // 文档标题根不编号
		}
		depth := lvl - int(chapterLevel) // 0=章
		for len(counters) <= depth {
			counters = append(counters, 0)
		}
		counters[depth]++
		for d := depth + 1; d < len(counters); d++ {
			counters[d] = 0
		}
		title := stripLeadingNumberPrefix(out[i].Title)
		if depth == 0 {
			out[i].Title = "第" + toChineseNumber(counters[0]) + "章 " + title
			continue
		}
		parts := make([]string, depth+1)
		for d := 0; d <= depth; d++ {
			parts[d] = strconv.Itoa(counters[d])
		}
		out[i].Title = strings.Join(parts, ".") + " " + title
	}
	return out
}

// NormalizeOutlineTitles 通用大纲标题规范化（模板提取路径）：
// L1 即“章”，L2/L3/L4 依次为 1.1、1.1.1、1.1.1.1。
func NormalizeOutlineTitles(items []OutlineItem) []OutlineItem {
	return applyOutlineNumbers(items, 1)
}

// NormalizeBlueprintTitles 标书蓝图标题规范化：
// L1 为文档标题根（如“投标书大纲目录”，不编号），L2 起为“第一章/1.1/1.1.1…”。
func NormalizeBlueprintTitles(items []OutlineItem) []OutlineItem {
	return applyOutlineNumbers(items, 2)
}

// stripLeadingNumberPrefix 循环剥除标题开头的编号前缀，直到不再匹配。
func stripLeadingNumberPrefix(title string) string {
	t := strings.TrimSpace(title)
	for i := 0; i < 4; i++ {
		before := t
		for _, re := range rePrefixes {
			if loc := re.FindStringIndex(t); loc != nil && loc[0] == 0 {
				t = strings.TrimSpace(t[loc[1]:])
				break
			}
		}
		if t == before {
			break
		}
	}
	return t
}

// toChineseNumber 阿拉伯数字 → 中文序数（1-99；>=100 用阿拉伯数字兜底）。
func toChineseNumber(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if n <= 10 {
		if n == 10 {
			return "十"
		}
		return digits[n]
	}
	if n >= 100 {
		return strconv.Itoa(n)
	}
	var sb strings.Builder
	tens, ones := n/10, n%10
	if tens > 1 {
		sb.WriteString(digits[tens])
	}
	sb.WriteString("十")
	if ones > 0 {
		sb.WriteString(digits[ones])
	}
	return sb.String()
}
