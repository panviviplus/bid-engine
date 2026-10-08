package bidgen

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"bid-engine/pkg/db/model"
)

// ── 素材/图表渲染 ───────────────────────────────────────────────
//
// 改造前素材只在章节末尾硬贴最多 6 张图片、图注等于素材名；
// 现在由写作规格决定引用哪些素材，模型在正文中用占位标记指明位置，
// 服务端把标记替换为真实图片节点或结构化表格，并统一编号“图 X-Y”“表 X-Y”。

// chapterFigure 已插入正文的图片
type chapterFigure struct {
	Number       string `json:"number"`
	Caption      string `json:"caption"`
	MaterialID   int64  `json:"material_id"`
	MaterialType string `json:"material_type"`
	ImageID      int64  `json:"image_id"`
	FileID       int64  `json:"file_id"`
	ObjectKey    string `json:"object_key"`
}

type chapterMarkerKind string

const (
	markerFigure      chapterMarkerKind = "figure"
	markerScoring     chapterMarkerKind = "scoring"
	markerPerformance chapterMarkerKind = "performance"
)

type chapterMarker struct {
	Kind chapterMarkerKind
	Ref  string
}

// chapterBlock 正文切分单元：普通文本块或图表占位标记
type chapterBlock struct {
	Text   string
	Marker *chapterMarker
}

// renderChapter 把模型输出的 markdown-lite 正文转换为 ProseMirror 节点，
// 并把图片/表格占位标记替换为结构化节点。chapterNo 为所属一级章节序号（图表编号前缀）。
// pendingRefs 为写作规格已选定但正文未放置标记的素材：这些素材的图片会在
// 章节末尾兜底插入，避免“素材已引用却没有出现在文档里”。
func renderChapter(content string, ev *chapterEvidence, chapterNo int, pendingRefs []string) ([]pmNode, []chapterFigure) {
	var nodes []pmNode
	var figures []chapterFigure
	figureNo := 0
	tableNo := 0
	placedRefs := make(map[string]bool, len(pendingRefs))

	appendFigureNodes := func(card materialCard) {
		for _, image := range card.Images {
			figureNo++
			number := formatChartNumber("图", chapterNo, figureNo)
			caption := firstNonEmptyString(image.Name, image.Desc, card.Name)
			nodes = append(nodes, pmNode{
				Type:  "image",
				Attrs: map[string]interface{}{"src": buildDownloadURL(image.ObjectKey), "alt": caption},
			})
			nodes = append(nodes, captionNode(number+"　"+caption))
			figures = append(figures, chapterFigure{
				Number:       number,
				Caption:      caption,
				MaterialID:   card.ID,
				MaterialType: card.Type,
				ImageID:      image.ID,
				ObjectKey:    image.ObjectKey,
			})
		}
	}

	flushText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		nodes = append(nodes, textToPMJSON(text)...)
	}

	var buffer []string
	for _, block := range splitChapterBlocks(content) {
		if block.Marker == nil {
			buffer = append(buffer, block.Text)
			continue
		}
		flushText(strings.Join(buffer, "\n\n"))
		buffer = nil

		switch block.Marker.Kind {
		case markerFigure:
			card, ok := ev.material(block.Marker.Ref)
			if !ok || len(card.Images) == 0 {
				continue
			}
			placedRefs[block.Marker.Ref] = true
			appendFigureNodes(card)
		case markerScoring:
			rows := scoringMatrixRows(ev.Scoring)
			if len(rows) < 2 {
				continue
			}
			tableNo++
			number := formatChartNumber("表", chapterNo, tableNo)
			nodes = append(nodes, captionNode(number+"　本章评分项响应对照表"))
			nodes = append(nodes, tableNode(rows))
		case markerPerformance:
			rows := performanceTableRows(ev.Materials)
			if len(rows) < 2 {
				continue
			}
			tableNo++
			number := formatChartNumber("表", chapterNo, tableNo)
			nodes = append(nodes, captionNode(number+"　类似项目业绩一览表"))
			nodes = append(nodes, tableNode(rows))
		}
	}
	flushText(strings.Join(buffer, "\n\n"))

	// 兜底：规格选定但正文未放置标记的素材，图片在章末插入
	for _, ref := range pendingRefs {
		if placedRefs[ref] {
			continue
		}
		card, ok := ev.material(ref)
		if !ok || len(card.Images) == 0 {
			continue
		}
		placedRefs[ref] = true
		appendFigureNodes(card)
		if figureNo > 0 && figureNo >= 8 {
			break
		}
	}

	if len(nodes) == 0 {
		nodes = []pmNode{paragraphNode()}
	}
	return sanitizePMNodes(nodes), figures
}

// splitChapterBlocks 按空行切分正文，并把图表占位标记识别为独立块。
func splitChapterBlocks(content string) []chapterBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	blocks := make([]chapterBlock, 0, len(lines))
	buffer := make([]string, 0, len(lines))

	flush := func() {
		text := strings.TrimSpace(strings.Join(buffer, "\n"))
		if text != "" {
			blocks = append(blocks, chapterBlock{Text: text})
		}
		buffer = buffer[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if marker, ok := parseChapterMarker(trimmed); ok {
			flush()
			markerCopy := marker
			blocks = append(blocks, chapterBlock{Marker: &markerCopy})
			continue
		}
		if trimmed == "" {
			flush()
			continue
		}
		buffer = append(buffer, line)
	}
	flush()
	return blocks
}

// parseChapterMarker 解析 [[图:M1]]、[[表:评分]]、[[表:业绩]] 三种占位标记。
func parseChapterMarker(line string) (chapterMarker, bool) {
	normalized := strings.ReplaceAll(line, " ", "")
	normalized = strings.ReplaceAll(normalized, "：", ":")
	normalized = strings.ReplaceAll(normalized, "［", "[")
	normalized = strings.ReplaceAll(normalized, "］", "]")
	if !strings.HasPrefix(normalized, "[[") || !strings.HasSuffix(normalized, "]]") {
		return chapterMarker{}, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(normalized, "[["), "]]")
	parts := strings.SplitN(inner, ":", 2)
	if len(parts) != 2 {
		return chapterMarker{}, false
	}
	kind := strings.TrimSpace(parts[0])
	ref := strings.ToUpper(strings.TrimSpace(parts[1]))
	switch kind {
	case "图":
		if !strings.HasPrefix(ref, "M") {
			return chapterMarker{}, false
		}
		return chapterMarker{Kind: markerFigure, Ref: ref}, true
	case "表":
		switch ref {
		case "评分", "评分项", "评分响应":
			return chapterMarker{Kind: markerScoring}, true
		case "业绩", "业绩一览":
			return chapterMarker{Kind: markerPerformance}, true
		}
	}
	return chapterMarker{}, false
}

// extractImageNodes 从已有章节 JSON 中提取图片与图注节点（重写/扩写/缩写时保留原图）。
func extractImageNodes(contentJSON string) []pmNode {
	var nodes []pmNode
	if strings.TrimSpace(contentJSON) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(contentJSON), &nodes); err != nil {
		return nil
	}
	result := make([]pmNode, 0, len(nodes))
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		if node.Type == "image" {
			result = append(result, node)
			continue
		}
		// 图注：图片紧随其后的居中段落
		if node.Type == "paragraph" && node.Attrs["textAlign"] == "center" && len(result) > 0 {
			if result[len(result)-1].Type == "image" {
				result = append(result, node)
			}
		}
	}
	return result
}

func captionNode(text string) pmNode {
	return pmNode{
		Type:    "paragraph",
		Attrs:   map[string]interface{}{"textAlign": "center"},
		Content: []pmNode{textNode(text)},
	}
}

func formatChartNumber(prefix string, chapterNo, index int) string {
	chapter := chapterNo
	if chapter <= 0 {
		chapter = 1
	}
	return fmt.Sprintf("%s %d-%d", prefix, chapter, index)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// scoringMatrixRows 由评分标准行构造“序号 / 评分项 / 分值 / 评分标准”矩阵。
func scoringMatrixRows(rows []scoringRow) [][]string {
	if len(rows) == 0 {
		return nil
	}
	result := [][]string{{"序号", "评分项", "分值", "评分标准"}}
	for i, row := range rows {
		index := strings.TrimSpace(row.Index)
		if index == "" {
			index = fmt.Sprintf("%d", i+1)
		}
		result = append(result, []string{index, row.Item, row.Score, row.Standard})
	}
	return result
}

// performanceTableRows 由业绩类素材卡片的结构化字段构造业绩一览表。
func performanceTableRows(cards []materialCard) [][]string {
	result := [][]string{{"序号", "项目名称", "业主单位", "合同金额", "完工时间"}}
	for _, card := range cards {
		if card.Type != "performance" || len(card.Fields) == 0 {
			continue
		}
		project := card.Fields["项目名称"]
		client := card.Fields["业主单位"]
		amount := card.Fields["合同金额"]
		completed := card.Fields["完工时间"]
		if project == "" && client == "" && amount == "" {
			continue
		}
		if project == "" {
			project = card.Name
		}
		result = append(result, []string{fmt.Sprintf("%d", len(result)), project, client, amount, completed})
		if len(result) > 20 {
			break
		}
	}
	if len(result) < 2 {
		return nil
	}
	return result
}

// buildDownloadURL 构造图片下载 URL（与前端 gallery 一致：/api/material/file/download/{objectKey}）
func buildDownloadURL(objectKey string) string {
	segs := strings.Split(objectKey, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return "/api/material/file/download/" + strings.Join(segs, "/")
}

// materialRefRecords 生成素材引用记录，保证“哪些素材被用在哪一章、是否插入图片”可追溯。
func materialRefRecords(proj *model.BidGenProject, outlineID int64, cards []materialCard, figures []chapterFigure) []*model.BidGenMaterialRef {
	if proj == nil {
		return nil
	}
	type figureInfo struct {
		inserted bool
		imageID  int64
		object   string
	}
	used := make(map[int64]figureInfo, len(figures))
	for _, figure := range figures {
		info := used[figure.MaterialID]
		info.inserted = true
		if info.object == "" {
			info.imageID, info.object = figure.ImageID, figure.ObjectKey
		}
		used[figure.MaterialID] = info
	}
	records := make([]*model.BidGenMaterialRef, 0, len(cards))
	for _, card := range cards {
		if card.ID <= 0 {
			continue
		}
		info := used[card.ID]
		records = append(records, &model.BidGenMaterialRef{
			ProjectID:    proj.ID,
			OutlineID:    outlineID,
			MaterialType: card.Type,
			MaterialID:   card.ID,
			FileID:       info.imageID,
			FileURL:      info.object,
			Inserted:     info.inserted,
		})
	}
	return records
}
