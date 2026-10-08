package bidanalysisv3

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var (
	amountTokenPattern = regexp.MustCompile(`-?[0-9][0-9,]*(?:\.[0-9]+)?`)
	chineseDatePattern = regexp.MustCompile(`^(\d{4})年(\d{1,2})月(\d{1,2})日(?:[\s　]*(\d{1,2})(?:时|:|：)(\d{1,2})分?)?$`)
	// collapsedTableRowSeparator 折叠表格的行边界：两个管道符之间只有空白。
	// 模型常把整张 GFM 表格压缩成一行（单元格之间是单个 |，行之间是 | |）。
	collapsedTableRowSeparator = regexp.MustCompile(`\|\s*\|`)
	tableSeparatorCellPattern  = regexp.MustCompile(`^:?-+:?$`)
)

// canonicalMarkdownTable 把被压缩成一行（或行内空白分隔）的 GFM 表格重新排版为
// 逐行表格，使其能被标准 Markdown 渲染器识别为表格。
// 只有在能确定列数、且每一行列数一致时才改写；任何不确定都返回空串，由调用方保留原值。
func canonicalMarkdownTable(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.Contains(trimmed, "|") {
		return ""
	}
	// 已经是多行表格：保持模型原始排版，不改造
	if strings.Count(trimmed, "\n") >= 2 {
		return ""
	}
	chunks := collapsedTableRowSeparator.Split(strings.ReplaceAll(trimmed, "\n", ""), -1)
	if len(chunks) < 3 {
		return ""
	}
	rows := make([][]string, 0, len(chunks))
	for _, chunk := range chunks {
		cells := splitMarkdownTableRow(chunk)
		if len(cells) == 0 {
			return ""
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}

	separatorIndex := -1
	for i, row := range rows {
		if isMarkdownTableSeparator(row) {
			separatorIndex = i
			break
		}
	}
	if separatorIndex <= 0 {
		return ""
	}
	columns := len(rows[separatorIndex])
	if columns < 2 || len(rows[separatorIndex-1]) != columns {
		return ""
	}

	body := make([][]string, 0, len(rows))
	for _, row := range rows[separatorIndex+1:] {
		if len(row) < columns {
			// 行尾说明文字等非表格内容：到此为止
			break
		}
		body = append(body, row[:columns])
	}
	if len(body) == 0 {
		return ""
	}

	var builder strings.Builder
	writeRow := func(cells []string) {
		builder.WriteString("| ")
		builder.WriteString(strings.Join(cells, " | "))
		builder.WriteString(" |\n")
	}
	writeRow(rows[separatorIndex-1])
	builder.WriteString("|")
	builder.WriteString(strings.Repeat(" --- |", columns))
	builder.WriteString("\n")
	for _, row := range body {
		writeRow(row)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func buildNormalizedValue(candidate extractedCandidate, packet extractionPacket) map[string]any {
	value := map[string]any{"amount": nil, "currency": nil, "unit": nil, "original": nil, "iso": nil, "data_json": nil}
	original := strings.TrimSpace(candidate.DisplayValue)
	if original == "" {
		return value
	}
	value["original"] = original
	switch candidate.ValueType {
	case "amount":
		tokens := amountTokenPattern.FindAllString(original, -1)
		if len(tokens) == 1 {
			value["amount"] = strings.ReplaceAll(tokens[0], ",", "")
		}
		if strings.ContainsAny(original, "¥￥") || strings.Contains(original, "人民币") || strings.Contains(strings.ToUpper(original), "RMB") || strings.Contains(strings.ToUpper(original), "CNY") || strings.Contains(original, "元") {
			value["currency"] = "CNY"
		}
		for _, unit := range []string{"亿元", "万元", "元/标段", "元／标段", "元"} {
			if strings.Contains(original, unit) {
				value["unit"] = strings.ReplaceAll(unit, "／", "/")
				break
			}
		}
	case "date", "datetime":
		value["iso"] = normalizedISODate(original)
	case "list":
		value["data_json"] = normalizeListJSON(original)
	case "object":
		value["data_json"] = normalizeObjectJSON(original)
	case "table":
		rows := make([][]string, 0)
		refs := make([]string, 0)
		allowed := map[string]bool{}
		for _, ref := range candidate.EvidenceRefs {
			if ref.Kind == "table" {
				allowed[ref.Ref] = true
			}
		}
		for _, table := range packet.Tables {
			if allowed[table.Ref] {
				rows = append(rows, table.Rows...)
				refs = append(refs, table.Ref)
			}
		}
		// 未引用到源表格（如评分标准表来自正文而非 Docling 表格）时，
		// 回退解析 display_value 中的 GFM Markdown 表格，保证表格类字段
		// 在数据层始终是结构化二维表，而不是被压成单行文本。
		if len(rows) == 0 {
			rows = markdownTableRows(original)
		}
		if len(rows) == 0 {
			rows = [][]string{{original}}
		}
		encoded, _ := json.Marshal(map[string]any{"rows": rows, "source_table_refs": refs})
		value["data_json"] = string(encoded)
	}
	return value
}

// markdownTableRows 解析 GFM Markdown 表格为二维行数组。
// 返回 nil 表示文本不是可解析的表格：缺少表头分隔行，或数据行列数与表头不一致。
func markdownTableRows(text string) [][]string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	rows := make([][]string, 0, len(lines))
	separatorSeen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, "|") {
			continue
		}
		cells := splitMarkdownTableRow(trimmed)
		if len(cells) == 0 {
			continue
		}
		if !separatorSeen && isMarkdownTableSeparator(cells) {
			separatorSeen = true
			continue
		}
		rows = append(rows, cells)
	}
	if !separatorSeen || len(rows) < 2 {
		return nil
	}
	width := len(rows[0])
	if width == 0 {
		return nil
	}
	for _, row := range rows[1:] {
		if len(row) != width {
			return nil
		}
	}
	return rows
}

func splitMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	parts := strings.Split(trimmed, "|")
	cells := make([]string, len(parts))
	for i, part := range parts {
		cells[i] = strings.TrimSpace(part)
	}
	return cells
}

func isMarkdownTableSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		trimmed = strings.TrimPrefix(trimmed, ":")
		trimmed = strings.TrimSuffix(trimmed, ":")
		trimmed = strings.ReplaceAll(trimmed, " ", "")
		if trimmed == "" {
			return false
		}
		for _, r := range trimmed {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func normalizedISODate(original string) any {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, original); err == nil {
			if layout == "2006-01-02" {
				return parsed.Format("2006-01-02")
			}
			return parsed.Format("2006-01-02T15:04:05")
		}
	}
	match := chineseDatePattern.FindStringSubmatch(original)
	if match == nil {
		return nil
	}
	layout, input := "2006年1月2日", original
	outputLayout := "2006-01-02"
	if match[4] != "" {
		layout, input, outputLayout = "2006年1月2日 15时04分", strings.ReplaceAll(strings.ReplaceAll(original, ":", "时"), "：", "时"), "2006-01-02T15:04:05"
	}
	parsed, err := time.ParseInLocation(layout, input, time.Local)
	if err != nil {
		return nil
	}
	return parsed.Format(outputLayout)
}

func normalizeListJSON(original string) string {
	if json.Valid([]byte(original)) {
		var items []any
		if json.Unmarshal([]byte(original), &items) == nil {
			encoded, _ := json.Marshal(items)
			return string(encoded)
		}
	}
	items := strings.FieldsFunc(original, func(r rune) bool { return r == '；' || r == ';' || r == '\n' || r == '、' })
	if len(items) == 0 {
		items = []string{original}
	}
	for index := range items {
		items[index] = strings.TrimSpace(items[index])
	}
	encoded, _ := json.Marshal(items)
	return string(encoded)
}

func normalizeObjectJSON(original string) string {
	if json.Valid([]byte(original)) {
		var object map[string]any
		if json.Unmarshal([]byte(original), &object) == nil {
			encoded, _ := json.Marshal(object)
			return string(encoded)
		}
	}
	encoded, _ := json.Marshal(map[string]string{"text": original})
	return string(encoded)
}
