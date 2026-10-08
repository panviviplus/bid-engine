package bidgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/docling"
	repoLLM "bid-engine/pkg/repo/llm"
)

const llmFeatureTemplateOutline = "bid_gen_template_outline"

type permanentParseError struct{ err error }

func (e *permanentParseError) Error() string      { return e.err.Error() }
func (e *permanentParseError) Unwrap() error      { return e.err }
func (e *permanentParseError) NonRetryable() bool { return true }

type templateHeading struct {
	CandidateID string
	RawTitle    string
	Title       string
	Level       int32
	Label       string
	OutlineID   int64
}

type templateOutlineCandidate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Label string `json:"label"`
	Order int    `json:"order"`
}

type templateOutlineLLMOutput struct {
	Nodes []struct {
		CandidateID string `json:"candidate_id"`
		Level       int32  `json:"level"`
	} `json:"nodes"`
}

var (
	templatePartPattern        = regexp.MustCompile(`^(?:\d+\s+)?第[一二三四五六七八九十百千零〇\d]+(?:部分|章|篇)\s*`)
	templateSectionPattern     = regexp.MustCompile(`^(\d+(?:\.\d+)+)[、.．]?\s*`)
	templateTOCLinePattern     = regexp.MustCompile(`\s+\d+\s*$`)
	templateHTMLHeadingPattern = regexp.MustCompile(`^h[1-6]$`)
)

func (s *svcImpl) parseTemplateSource(ctx context.Context, proj *model.BidGenProject, localPath, ext string) (*docling.ParseResult, error) {
	parsePath := localPath
	var generated []string
	defer func() {
		for _, path := range generated {
			_ = os.Remove(path)
		}
	}()
	if ext == ".doc" {
		docxPath := strings.TrimSuffix(localPath, ext) + ".docx"
		if err := s.pdf.ConvertDocToDocx(newGinCtx(ctx, proj.UserID), localPath, docxPath); err != nil {
			s.logger.Warnw("模板 DOC 转 DOCX 失败", "project_id", proj.ID, "err", err)
		} else {
			parsePath = docxPath
			generated = append(generated, docxPath)
		}
	}

	result, firstErr := s.docling.Parse(ctx, parsePath, &docling.ParseOptions{ToFormats: []string{"json", "md", "html", "text"}})
	hadServiceError := firstErr != nil
	if usableTemplateParseResult(result) {
		return result, nil
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("Docling 返回状态 %s 且内容为空", templateParseStatus(result))
	}
	if ext != ".pdf" {
		pdfPath := strings.TrimSuffix(localPath, ext) + ".pdf"
		if err := s.pdf.Convert2PdfBySoffice(newGinCtx(ctx, proj.UserID), localPath, pdfPath); err == nil {
			generated = append(generated, pdfPath)
			fallback, fallbackErr := s.docling.Parse(ctx, pdfPath, &docling.ParseOptions{ToFormats: []string{"json", "md", "html", "text"}})
			if usableTemplateParseResult(fallback) {
				return fallback, nil
			}
			if fallbackErr != nil {
				hadServiceError = true
				firstErr = errors.Join(firstErr, fallbackErr)
			}
		}
	}
	if !hadServiceError {
		return nil, &permanentParseError{err: fmt.Errorf("模板解析结果为空或格式不受支持")}
	}
	return nil, firstErr
}

func templateParseStatus(result *docling.ParseResult) string {
	if result == nil {
		return "empty"
	}
	return result.Status
}

func usableTemplateParseResult(result *docling.ParseResult) bool {
	if result == nil || (result.Status != "success" && result.Status != "partial_success") {
		return false
	}
	jsonText := strings.TrimSpace(string(result.JSON))
	jsonUsable := json.Valid(result.JSON) && jsonText != "" && jsonText != "null" && jsonText != "{}" && jsonText != "[]"
	return jsonUsable || strings.TrimSpace(result.HTML) != "" || strings.TrimSpace(result.Markdown) != "" || strings.TrimSpace(result.Text) != ""
}

func extractTemplateHeadings(result *docling.ParseResult) ([]templateHeading, []templateOutlineCandidate) {
	if result == nil {
		return nil, nil
	}
	doc, err := result.ParseDocument()
	if err != nil || doc == nil {
		var headings []templateHeading
		var candidates []templateOutlineCandidate
		for index, line := range strings.Split(firstNonEmptyTemplateText(result), "\n") {
			raw := normalizeTemplateText(line)
			if raw == "" || isTemplateNoise(raw) {
				continue
			}
			candidate := templateOutlineCandidate{ID: fmt.Sprintf("line_%d", index), Title: raw, Label: "text", Order: index}
			if utf8.RuneCountInString(raw) <= 120 {
				candidates = append(candidates, candidate)
			}
			level := templateHeadingLevel(docling.TextItem{Label: "text"}, raw, 1)
			if level > 0 {
				headings = append(headings, templateHeading{CandidateID: candidate.ID, RawTitle: raw, Title: raw, Level: level, Label: "text"})
			}
		}
		if len(headings) == 0 {
			for index, node := range extractOutlineFromHTML(result.HTML) {
				if node == nil || isTemplateNoise(node.Title) {
					continue
				}
				headings = append(headings, templateHeading{CandidateID: fmt.Sprintf("html_%d", index), RawTitle: normalizeTemplateText(node.Title), Title: normalizeTemplateText(node.Title), Level: node.Level, Label: "heading"})
			}
		}
		return dedupeTemplateHeadings(headings), candidates
	}
	orderedTexts := orderedDoclingTemplateTexts(doc)
	headings := make([]templateHeading, 0, 32)
	candidates := make([]templateOutlineCandidate, 0, len(orderedTexts))
	inTOC := false
	currentLevel := int32(1)
	for index, item := range orderedTexts {
		if item.ContentLayer == "furniture" {
			continue
		}
		raw := normalizeTemplateText(item.Text)
		if raw == "" {
			continue
		}
		id := strings.TrimSpace(item.SelfRef)
		if id == "" {
			id = fmt.Sprintf("text_%d", index)
		}
		if utf8.RuneCountInString(raw) <= 120 && !isTemplateNoise(raw) {
			candidates = append(candidates, templateOutlineCandidate{ID: id, Title: raw, Label: item.Label, Order: index})
		}
		compact := strings.ReplaceAll(raw, " ", "")
		if compact == "目录" || compact == "1目录" {
			inTOC = true
			continue
		}
		level := templateHeadingLevel(item, raw, currentLevel)
		if inTOC {
			if level == 0 || templateTOCLinePattern.MatchString(raw) {
				continue
			}
			inTOC = false
		}
		if level == 0 || isTemplateNoise(raw) {
			continue
		}
		title := strings.TrimSpace(regexp.MustCompile(`^\d+\s+(第.+)$`).ReplaceAllString(raw, "$1"))
		headings = append(headings, templateHeading{CandidateID: id, RawTitle: raw, Title: title, Level: level, Label: item.Label})
		if strings.ToLower(strings.TrimSpace(item.Label)) != "list_item" {
			currentLevel = level
		}
	}
	return dedupeTemplateHeadings(headings), candidates
}

func orderedDoclingTemplateTexts(doc *docling.DoclingDocument) []docling.TextItem {
	if doc == nil {
		return nil
	}
	texts := make(map[string]docling.TextItem, len(doc.Texts))
	for _, item := range doc.Texts {
		texts[item.SelfRef] = item
	}
	groups := make(map[string]docling.GroupItem, len(doc.Groups))
	for _, group := range doc.Groups {
		groups[group.SelfRef] = group
	}
	result := make([]docling.TextItem, 0, len(doc.Texts))
	visitedText := make(map[string]bool, len(doc.Texts))
	visitedGroup := make(map[string]bool, len(doc.Groups))
	var walkRef func(string)
	walkRef = func(ref string) {
		if item, ok := texts[ref]; ok {
			if !visitedText[ref] {
				visitedText[ref] = true
				result = append(result, item)
			}
			return
		}
		group, ok := groups[ref]
		if !ok || visitedGroup[ref] {
			return
		}
		visitedGroup[ref] = true
		for _, child := range group.Children {
			walkRef(child.Ref)
		}
	}
	for _, child := range doc.Body.Children {
		walkRef(child.Ref)
	}
	// 兼容没有 body.children 或部分节点未挂树的 Docling 版本。
	for _, item := range doc.Texts {
		if !visitedText[item.SelfRef] {
			result = append(result, item)
		}
	}
	return result
}

func templateHeadingLevel(item docling.TextItem, title string, currentLevel int32) int32 {
	if templatePartPattern.MatchString(title) {
		return 1
	}
	if match := templateSectionPattern.FindStringSubmatch(title); len(match) == 2 {
		level := int32(strings.Count(match[1], ".") + 2)
		if level > 4 {
			return 4
		}
		return level
	}
	switch strings.ToLower(strings.TrimSpace(item.Label)) {
	case "title":
		return 1
	case "section_header", "section_title", "chapter_title":
		return 1
	case "subsection_header":
		return 2
	case "sub_subsection_header":
		return 3
	case "list_item":
		if item.Enumerated || item.Formatting != nil && item.Formatting.Bold != nil && *item.Formatting.Bold {
			if currentLevel < 1 {
				return 1
			}
			if currentLevel >= 4 {
				return 4
			}
			return currentLevel + 1
		}
	}
	return 0
}

func normalizeTemplateText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func isTemplateNoise(value string) bool {
	compact := strings.ReplaceAll(value, " ", "")
	if compact == "" || compact == "目录" || compact == "投标文件" || compact == "投标书" {
		return true
	}
	if templateTOCLinePattern.MatchString(value) || utf8.RuneCountInString(value) > 120 {
		return true
	}
	return strings.HasSuffix(value, "。") || strings.HasSuffix(value, "；")
}

func dedupeTemplateHeadings(input []templateHeading) []templateHeading {
	result := make([]templateHeading, 0, len(input))
	for _, heading := range input {
		if len(result) > 0 && result[len(result)-1].Title == heading.Title && result[len(result)-1].Level == heading.Level {
			continue
		}
		if heading.Level < 1 {
			heading.Level = 1
		}
		if heading.Level > 4 {
			heading.Level = 4
		}
		result = append(result, heading)
	}
	return result
}

func (s *svcImpl) llmTemplateOutline(ctx context.Context, result *docling.ParseResult, candidates []templateOutlineCandidate) ([]templateHeading, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("模板中没有可用的大纲候选文本")
	}
	payload, _ := json.Marshal(map[string]any{"document_text": firstNonEmptyTemplateText(result), "candidates": candidates})
	maxTokens := 2048
	if cfg := repoLLM.ResolveConfig(ctx, llmFeatureTemplateOutline); cfg != nil {
		if cfg.DefaultMaxTokens > 0 && cfg.DefaultMaxTokens < maxTokens {
			maxTokens = cfg.DefaultMaxTokens
		}
	} else {
		return nil, repoLLM.ErrLLMNotConfigured
	}
	temperature := 0.0
	req := &repoLLM.ChatRequest{
		System: "你是投标模板大纲提取器。只能从 candidates 中选择真实标题，保持原始顺序，不得编造或改写标题。返回层级 1-4 的 JSON。",
		Prompt: "TEMPLATE_DOCUMENT:\n" + string(payload), Temperature: &temperature, MaxTokens: &maxTokens,
		ResponseFormat: templateOutlineResponseFormat(),
	}
	response, err := s.llm.ChatOnceByFeature(ctx, llmFeatureTemplateOutline, req)
	if err != nil {
		return nil, err
	}
	var output templateOutlineLLMOutput
	content := strings.TrimSpace(response.Content)
	content = strings.TrimPrefix(strings.TrimSuffix(content, "```"), "```json")
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &output); err != nil {
		return nil, fmt.Errorf("模板大纲模型结果无效: %w", err)
	}
	byID := make(map[string]templateOutlineCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	headings := make([]templateHeading, 0, len(output.Nodes))
	lastOrder := -1
	for _, node := range output.Nodes {
		candidate, ok := byID[node.CandidateID]
		if !ok || candidate.Order < lastOrder || node.Level < 1 || node.Level > 4 {
			return nil, fmt.Errorf("模板大纲模型引用了无效候选")
		}
		lastOrder = candidate.Order
		headings = append(headings, templateHeading{CandidateID: candidate.ID, RawTitle: candidate.Title, Title: candidate.Title, Label: candidate.Label, Level: node.Level})
	}
	if len(headings) == 0 {
		return nil, fmt.Errorf("模板大纲模型没有返回有效节点")
	}
	return dedupeTemplateHeadings(headings), nil
}

func templateOutlineResponseFormat() map[string]any {
	node := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"candidate_id", "level"}, "properties": map[string]any{
		"candidate_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
		"level":        map[string]any{"type": "integer", "minimum": 1, "maximum": 4},
	}}
	schema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"nodes"}, "properties": map[string]any{
		"nodes": map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": node},
	}}
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "bid_gen_template_outline", "strict": true, "schema": schema}}
}

func firstNonEmptyTemplateText(result *docling.ParseResult) string {
	if result == nil {
		return ""
	}
	if strings.TrimSpace(result.Text) != "" {
		return result.Text
	}
	return result.Markdown
}

func templateSourceHTML(result *docling.ParseResult) string {
	if result == nil {
		return ""
	}
	if strings.TrimSpace(result.HTML) != "" {
		return result.HTML
	}
	var body strings.Builder
	body.WriteString("<html><body>")
	for _, line := range strings.Split(firstNonEmptyTemplateText(result), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			body.WriteString("<p>" + stdhtml.EscapeString(line) + "</p>")
		}
	}
	body.WriteString("</body></html>")
	return body.String()
}

func (s *svcImpl) persistTemplateOutline(ctx context.Context, projectID int64, headings []templateHeading, sourceHTML string) error {
	if len(headings) == 0 || strings.TrimSpace(sourceHTML) == "" {
		return fmt.Errorf("模板正文或大纲为空")
	}
	return s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.BidGenProject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, projectID).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id=?", projectID).Delete(&model.BidGenOutline{}).Error; err != nil {
			return err
		}
		stack := make([]int, 0, 6)
		children := map[int64]int32{}
		for index := range headings {
			heading := &headings[index]
			for len(stack) > 0 && headings[stack[len(stack)-1]].Level >= heading.Level {
				stack = stack[:len(stack)-1]
			}
			parentID := int64(0)
			if len(stack) > 0 {
				parentID = headings[stack[len(stack)-1]].OutlineID
			}
			children[parentID]++
			node := &model.BidGenOutline{
				ProjectID: projectID, ParentID: parentID, Level: heading.Level,
				SortOrder: children[parentID] * 100, Title: heading.Title,
				ClauseIds: "[]", MaterialIds: "[]", GenStatus: OutlineGenPending, Source: "user",
			}
			if err := tx.Create(node).Error; err != nil {
				return err
			}
			heading.OutlineID = node.ID
			stack = append(stack, index)
		}
		anchoredHTML, err := anchorTemplateHTML(sourceHTML, headings)
		if err != nil {
			return err
		}
		if countHTMLOutlineAnchors(anchoredHTML) != len(headings) {
			return fmt.Errorf("模板正文大纲锚点不完整")
		}
		doc := &model.BidGenDocContent{ProjectID: projectID, DocJSON: "", DocHTML: anchoredHTML}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.Assignments(map[string]any{"doc_json": "", "doc_html": anchoredHTML, "version": gorm.Expr("version + 1")}),
		}).Create(doc).Error; err != nil {
			return err
		}
		stages := parseStageStatusJSON(project.StageStatus)
		stages[GenStageTemplateParse] = parseStageSucceeded
		stages[GenStageTemplateOutline] = parseStageSucceeded
		stageJSON, _ := json.Marshal(stages)
		return tx.Model(&project).Updates(map[string]any{
			"status": ProjectStatusOutlineReview, "stage": "", "stage_status": string(stageJSON),
			"progress": 100, "last_error": "",
		}).Error
	})
}

func anchorTemplateHTML(source string, headings []templateHeading) (string, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("解析模板 HTML: %w", err)
	}
	blocks := make([]*html.Node, 0, 128)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		isDirectListStrong := node.Type == html.ElementNode && node.Data == "strong" && node.Parent != nil && node.Parent.Type == html.ElementNode && node.Parent.Data == "li"
		if node.Type == html.ElementNode && (node.Data == "p" || node.Data == "h1" || node.Data == "h2" || node.Data == "h3" || node.Data == "h4" || node.Data == "h5" || node.Data == "h6" || isDirectListStrong) {
			blocks = append(blocks, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	start := 0
	for _, heading := range headings {
		matched := -1
		for index := start; index < len(blocks); index++ {
			text := normalizeTemplateText(extractText(blocks[index]))
			if text == heading.RawTitle || text == heading.Title {
				matched = index
				break
			}
		}
		if matched < 0 {
			return "", fmt.Errorf("模板标题无法定位到正文: %s", heading.Title)
		}
		node := blocks[matched]
		if node.Parent != nil && node.Parent.Type == html.ElementNode && node.Parent.Data == "strong" {
			node.Parent.Data = "div"
		}
		if node.Parent != nil && node.Parent.Type == html.ElementNode && node.Parent.Data == "li" && node.Parent.Parent != nil {
			listItem := node.Parent
			listItem.Data = "div"
			if listItem.Parent.Data == "ol" || listItem.Parent.Data == "ul" {
				listItem.Parent.Data = "div"
			}
		}
		node.Data = "h" + strconv.Itoa(int(heading.Level))
		node.Attr = setHTMLAttr(node.Attr, "data-outline-id", strconv.FormatInt(heading.OutlineID, 10))
		start = matched + 1
	}
	// 未纳入大纲的原始 h 标签（最常见是“目录”）降为普通段落，避免前端把它误同步成新章节。
	var demote func(*html.Node)
	demote = func(node *html.Node) {
		if node.Type == html.ElementNode && templateHTMLHeadingPattern.MatchString(node.Data) {
			hasAnchor := false
			for _, attr := range node.Attr {
				if attr.Key == "data-outline-id" && attr.Val != "" {
					hasAnchor = true
					break
				}
			}
			if !hasAnchor {
				node.Data = "p"
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			demote(child)
		}
	}
	demote(doc)
	var output bytes.Buffer
	if err := html.Render(&output, doc); err != nil {
		return "", err
	}
	return output.String(), nil
}

func setHTMLAttr(attrs []html.Attribute, key, value string) []html.Attribute {
	for index := range attrs {
		if attrs[index].Key == key {
			attrs[index].Val = value
			return attrs
		}
	}
	return append(attrs, html.Attribute{Key: key, Val: value})
}

func countHTMLOutlineAnchors(source string) int {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return 0
	}
	count := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.HasPrefix(node.Data, "h") {
			for _, attr := range node.Attr {
				if attr.Key == "data-outline-id" && attr.Val != "" {
					count++
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return count
}

func orderedTemplateCandidates(candidates []templateOutlineCandidate) []templateOutlineCandidate {
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Order < candidates[j].Order })
	return candidates
}
