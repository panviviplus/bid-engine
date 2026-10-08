package bidgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/service/biddoc"
)

var regexpNumberedList = regexp.MustCompile(`^\d+\.\s+`)

// pmNode ProseMirror JSON 节点
type pmNode struct {
	Type    string                 `json:"type"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Content []pmNode               `json:"content,omitempty"`
	Text    string                 `json:"text,omitempty"`
}

func textNode(text string) pmNode {
	return pmNode{Type: "text", Text: text}
}

func paragraphNode(children ...pmNode) pmNode {
	return pmNode{Type: "paragraph", Content: children}
}

// sanitizePMNodes 递归清理会导致 ProseMirror 解析失败的节点。
//
// ProseMirror 的 TextNode.fromJSON 要求 text 必须是非空字符串，否则抛
// RangeError("Invalid text node in JSON")；而标记为 omitempty 的空文本会被序列化成
// {"type":"text"}（缺少 text 字段），前端整篇文档解析失败、编辑器直接空白。
// 空表格单元格、清洗后为空的列表项是常见来源，这里统一兜底。
func sanitizePMNodes(nodes []pmNode) []pmNode {
	if len(nodes) == 0 {
		return nodes
	}
	result := make([]pmNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Type == "text" {
			if strings.TrimSpace(node.Text) == "" {
				continue
			}
			result = append(result, node)
			continue
		}
		if len(node.Content) > 0 {
			node.Content = sanitizePMNodes(node.Content)
		}
		switch node.Type {
		case "tableCell", "tableHeader":
			// 单元格必须至少包含一个块级节点
			if len(node.Content) == 0 {
				node.Content = []pmNode{paragraphNode()}
			}
		case "tableRow", "table", "listItem", "bulletList", "orderedList", "blockquote":
			// 被掏空的容器没有意义，直接丢弃
			if len(node.Content) == 0 {
				continue
			}
		}
		result = append(result, node)
	}
	return result
}

// sanitizePMDoc 清理整篇文档的 content。
func sanitizePMDoc(doc pmNode) pmNode {
	doc.Content = sanitizePMNodes(doc.Content)
	if len(doc.Content) == 0 {
		doc.Content = []pmNode{paragraphNode()}
	}
	return doc
}

func headingNode(level int, outlineID int64, text string) pmNode {
	attrs := map[string]interface{}{"level": level}
	if outlineID > 0 {
		attrs["outlineId"] = outlineID
	}
	return pmNode{Type: "heading", Attrs: attrs, Content: []pmNode{textNode(text)}}
}

// buildDocFromOutline 将大纲节点转为 TipTap 文档 JSON（仅标题层级 + 空段落占位）
// 文档顺序由公共 biddoc 服务按父子关系与兄弟排序确定。
func buildDocFromOutline(nodes []*model.BidGenOutline) (string, error) {
	return biddoc.BuildOutlineDocument(nodes)
}

// outlineIDFromAttrs 兼容从内存（int64）与 JSON 反序列化（float64）两种来源读取 outlineId
func outlineIDFromAttrs(attrs map[string]interface{}) (int64, bool) {
	if attrs == nil {
		return 0, false
	}
	v, ok := attrs["outlineId"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}

// findHeadingIndex 在 doc.content 中定位 outlineId 对应的标题节点索引
func findHeadingIndex(doc *pmNode, outlineID int64) int {
	for i := range doc.Content {
		n := &doc.Content[i]
		if n.Type != "heading" {
			continue
		}
		if v, ok := n.Attrs["outlineId"]; ok {
			if id, ok := v.(float64); ok && int64(id) == outlineID {
				return i
			}
		}
	}
	return -1
}

// bodyRange 返回章节正文区间 [start, end)（含标题节点）。
// 结束边界 = start 之后第一个“带 outlineId 的大纲标题”：只覆盖该章节自身的正文区间，
// 保留子章节标题（子章节是独立章节，由各自 outlineId 单独生成/替换）；
// LLM 生成的内容子标题没有 outlineId，属于本章节正文的一部分，应被包含在替换区间内。
func bodyRange(doc *pmNode, outlineID int64) (int, int) {
	start := findHeadingIndex(doc, outlineID)
	if start < 0 {
		return -1, -1
	}
	end := len(doc.Content)
	for i := start + 1; i < len(doc.Content); i++ {
		n := &doc.Content[i]
		if n.Type != "heading" {
			continue
		}
		if _, ok := outlineIDFromAttrs(n.Attrs); ok {
			end = i
			break
		}
	}
	return start, end
}

// chapterRange 返回章节区间 [start, end)（含标题节点，到下一个同级/更高级标题为止）
func chapterRange(doc *pmNode, outlineID int64) (int, int) {
	start := findHeadingIndex(doc, outlineID)
	if start < 0 {
		return -1, -1
	}
	level := 1
	if v, ok := doc.Content[start].Attrs["level"]; ok {
		if lv, ok := v.(float64); ok {
			level = int(lv)
		}
	}
	end := len(doc.Content)
	for i := start + 1; i < len(doc.Content); i++ {
		if doc.Content[i].Type != "heading" {
			continue
		}
		if _, ok := outlineIDFromAttrs(doc.Content[i].Attrs); !ok {
			continue
		}
		lv := 7
		if v, ok := doc.Content[i].Attrs["level"]; ok {
			if l, ok := v.(float64); ok {
				lv = int(l)
			}
		}
		if lv <= level {
			end = i
			break
		}
	}
	return start, end
}

// mergeChapter 将生成好的章节片段（含标题）合并进文档，按 outlineId 确定性替换
func mergeChapter(docJSON string, chapter []pmNode) (string, error) {
	var doc pmNode
	if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
		return "", fmt.Errorf("解析 doc_json 失败: %w", err)
	}
	if len(chapter) == 0 || chapter[0].Type != "heading" {
		return "", fmt.Errorf("章节片段必须以标题开头")
	}
	outlineID, ok := outlineIDFromAttrs(chapter[0].Attrs)
	if !ok {
		return "", fmt.Errorf("章节标题缺少 outlineId")
	}
	start, end := bodyRange(&doc, outlineID)
	if start < 0 {
		return "", fmt.Errorf("文档中未找到 outlineId=%d 的标题节点", int64(outlineID))
	}
	replaced := append([]pmNode{}, doc.Content[:start]...)
	replaced = append(replaced, sanitizePMNodes(chapter)...)
	replaced = append(replaced, doc.Content[end:]...)
	doc = sanitizePMDoc(doc)
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// textToPMJSON 将 LLM 输出的 markdown-lite 文本转为 ProseMirror 节点数组
// 支持：空行分段、# 标题（2-4级）、- 无序列表、1. 有序列表、| 表格
func textToPMJSON(text string) []pmNode {
	text = strings.TrimSpace(text)
	if text == "" {
		return []pmNode{}
	}
	blocks := strings.Split(text, "\n\n")
	var nodes []pmNode
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		switch {
		case strings.HasPrefix(block, "|"):
			if table := parsePipeTable(block); len(table) > 0 {
				nodes = append(nodes, tableNode(table))
			}
		case isBulletList(block):
			nodes = append(nodes, bulletListNode(block))
		case isOrderedList(block):
			nodes = append(nodes, orderedListNode(block))
		case strings.HasPrefix(block, "#### "), strings.HasPrefix(block, "### "), strings.HasPrefix(block, "## "), strings.HasPrefix(block, "# "):
			// 标题行与正文同块（LLM 常在标题后不空行）：只取首行作标题，
			// 剩余行作为正文递归解析，避免整块正文被吞进标题文本
			marker, level := headingMarker(block)
			lines := strings.SplitN(block, "\n", 2)
			title := strings.TrimSpace(strings.TrimPrefix(lines[0], marker))
			nodes = append(nodes, headingNode(level, 0, title))
			if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
				nodes = append(nodes, textToPMJSON(strings.TrimSpace(lines[1]))...)
			}
		default:
			// 单行内可能有换行，合并为一段
			flat := strings.ReplaceAll(block, "\n", " ")
			nodes = append(nodes, paragraphNode(textNode(cleanInline(flat))))
		}
	}
	if len(nodes) == 0 {
		return []pmNode{paragraphNode()}
	}
	return sanitizePMNodes(nodes)
}

// headingMarker 解析标题块前缀（#### / ### / ## / #）并返回对应层级。
// # 与 ## 均映射为 level 2：作为正文内容小标题，视觉上低于大纲章节标题。
func headingMarker(block string) (string, int) {
	switch {
	case strings.HasPrefix(block, "#### "):
		return "#### ", 4
	case strings.HasPrefix(block, "### "):
		return "### ", 3
	case strings.HasPrefix(block, "## "):
		return "## ", 2
	default:
		return "# ", 2
	}
}

func cleanInline(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func isBulletList(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
			return false
		}
	}
	return true
}

func isOrderedList(block string) bool {
	lines := strings.Split(block, "\n")
	count := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if !regexpNumberedList.MatchString(t) {
			return false
		}
		count++
	}
	return count > 0
}

func bulletListNode(block string) pmNode {
	var items []pmNode
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "- "), "* "))
		items = append(items, pmNode{Type: "listItem", Content: []pmNode{paragraphNode(textNode(cleanInline(t)))}})
	}
	return pmNode{Type: "bulletList", Content: items}
}

func orderedListNode(block string) pmNode {
	var items []pmNode
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		idx := strings.Index(t, ". ")
		if idx < 0 {
			continue
		}
		items = append(items, pmNode{
			Type: "listItem",
			Content: []pmNode{
				paragraphNode(textNode(cleanInline(strings.TrimSpace(t[idx+2:])))),
			},
		})
	}
	return pmNode{Type: "orderedList", Content: items}
}

// parsePipeTable 解析 markdown 管道表格为二维数组
func parsePipeTable(block string) [][]string {
	lines := strings.Split(block, "\n")
	var rows [][]string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.Contains(t, "---") {
			continue
		}
		t = strings.Trim(t, "|")
		cols := strings.Split(t, "|")
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}
		rows = append(rows, cols)
	}
	return rows
}

func tableNode(rows [][]string) pmNode {
	var tableContent []pmNode
	for i, row := range rows {
		var cells []pmNode
		for _, cell := range row {
			cType := "tableCell"
			if i == 0 {
				cType = "tableHeader"
			}
			cells = append(cells, pmNode{Type: cType, Content: []pmNode{paragraphNode(textNode(cell))}})
		}
		tableContent = append(tableContent, pmNode{Type: "tableRow", Content: cells})
	}
	return pmNode{Type: "table", Attrs: map[string]interface{}{"resizable": true}, Content: tableContent}
}

// extractChapterBodyText 从章节内容 JSON（[heading, ...正文, ...素材]）中提取正文纯文本，
// 供重写/扩写/缩写时把现有内容喂给 LLM。跳过标题节点与素材图片节点（含其子内容）。
func extractChapterBodyText(contentJSON string) string {
	var nodes []pmNode
	if err := json.Unmarshal([]byte(contentJSON), &nodes); err != nil {
		return ""
	}
	var parts []string
	var walk func(n pmNode)
	walk = func(n pmNode) {
		if n.Type == "heading" || n.Type == "image" {
			return
		}
		if n.Type == "text" {
			if t := strings.TrimSpace(n.Text); t != "" {
				parts = append(parts, t)
			}
			return
		}
		for _, ch := range n.Content {
			walk(ch)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return strings.Join(parts, "\n")
}
