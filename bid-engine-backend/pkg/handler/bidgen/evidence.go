package bidgen

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/material"
	"bid-engine/pkg/service/biddoc"
)

// ── 证据包：写作时允许引用的全部事实来源 ────────────────────────
//
// 改造前每章都会收到“全部 active 招标事实”，既不精准又稀释注意力；
// 现在按章节组装证据包，并用 C/F/S/M 编号让模型可引用、服务端可校验。

type evidenceClause struct {
	Ref     string
	Title   string
	Content string
}

type evidenceFact struct {
	Ref   string
	Name  string
	Value string
}

// scoringRow 评分标准表的一行（来自招标解析的“评分标准与评分办法”固定字段）
type scoringRow struct {
	Ref      string
	Index    string
	Item     string
	Score    string
	Standard string
	Response string
}

// materialImage 素材图片（含图片自身的名称与描述）
type materialImage struct {
	ID        int64
	ObjectKey string
	Name      string
	Desc      string
}

// materialCard 素材卡片：写作时真正注入的内容，而不只是素材名称
type materialCard struct {
	Ref    string
	ID     int64
	Type   string
	Name   string
	Desc   string
	Fields map[string]string
	Labels []string
	OCR    string
	Images []materialImage
}

// chapterEvidence 单章证据包
type chapterEvidence struct {
	Clauses   []evidenceClause
	Facts     []evidenceFact
	Scoring   []scoringRow
	Materials []materialCard
	AllTitles []string

	clauseBy   map[string]evidenceClause
	scoringBy  map[string]scoringRow
	materialBy map[string]materialCard
}

func (e *chapterEvidence) reindex() {
	e.clauseBy = make(map[string]evidenceClause, len(e.Clauses))
	for _, item := range e.Clauses {
		e.clauseBy[item.Ref] = item
	}
	e.scoringBy = make(map[string]scoringRow, len(e.Scoring))
	for _, item := range e.Scoring {
		e.scoringBy[item.Ref] = item
	}
	e.materialBy = make(map[string]materialCard, len(e.Materials))
	for _, item := range e.Materials {
		e.materialBy[item.Ref] = item
	}
}

func (e *chapterEvidence) material(ref string) (materialCard, bool) {
	if e == nil || e.materialBy == nil {
		return materialCard{}, false
	}
	card, ok := e.materialBy[ref]
	return card, ok
}

func (e *chapterEvidence) scoringRows(refs []string) []scoringRow {
	if e == nil {
		return nil
	}
	if len(refs) == 0 {
		return e.Scoring
	}
	result := make([]scoringRow, 0, len(refs))
	for _, ref := range refs {
		if row, ok := e.scoringBy[ref]; ok {
			result = append(result, row)
		}
	}
	return result
}

// render 生成写入 prompt 的证据清单文本。
func (e *chapterEvidence) render() string {
	if e == nil {
		return ""
	}
	var sections []string

	if len(e.Clauses) > 0 {
		lines := make([]string, 0, len(e.Clauses))
		for _, clause := range e.Clauses {
			lines = append(lines, fmt.Sprintf("[%s] %s：%s", clause.Ref, clause.Title, clause.Content))
		}
		sections = append(sections, "【招标要求证据】\n"+strings.Join(lines, "\n"))
	}
	if len(e.Facts) > 0 {
		lines := make([]string, 0, len(e.Facts))
		for _, fact := range e.Facts {
			lines = append(lines, fmt.Sprintf("[%s] %s：%s", fact.Ref, fact.Name, fact.Value))
		}
		sections = append(sections, "【招标关键事实】\n"+strings.Join(lines, "\n"))
	}
	if len(e.Scoring) > 0 {
		lines := make([]string, 0, len(e.Scoring))
		for _, row := range e.Scoring {
			parts := []string{fmt.Sprintf("[%s] 评分项：%s", row.Ref, row.Item)}
			if row.Score != "" {
				parts = append(parts, "分值："+row.Score)
			}
			if row.Standard != "" {
				parts = append(parts, "评分标准："+row.Standard)
			}
			if row.Response != "" {
				parts = append(parts, "响应要求："+row.Response)
			}
			lines = append(lines, strings.Join(parts, "；"))
		}
		sections = append(sections, "【评分标准表条目】\n"+strings.Join(lines, "\n"))
	}
	if len(e.Materials) > 0 {
		lines := make([]string, 0, len(e.Materials))
		for _, card := range e.Materials {
			lines = append(lines, renderMaterialCard(card))
		}
		sections = append(sections, "【企业素材卡片】\n"+strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

func renderMaterialCard(card materialCard) string {
	parts := []string{fmt.Sprintf("[%s]（%s）%s", card.Ref, materialTypeLabel(card.Type), card.Name)}
	if card.Desc != "" {
		parts = append(parts, "描述："+card.Desc)
	}
	if len(card.Labels) > 0 {
		parts = append(parts, "标签："+strings.Join(card.Labels, "、"))
	}
	if len(card.Fields) > 0 {
		keys := make([]string, 0, len(card.Fields))
		for key := range card.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fieldParts := make([]string, 0, len(keys))
		for _, key := range keys {
			if value := strings.TrimSpace(card.Fields[key]); value != "" {
				fieldParts = append(fieldParts, key+"="+value)
			}
		}
		if len(fieldParts) > 0 {
			parts = append(parts, "结构化信息："+strings.Join(fieldParts, "；"))
		}
	}
	if card.OCR != "" {
		parts = append(parts, "原文摘录："+card.OCR)
	}
	if len(card.Images) > 0 {
		captions := make([]string, 0, len(card.Images))
		for _, image := range card.Images {
			caption := strings.TrimSpace(image.Name)
			if caption == "" {
				caption = strings.TrimSpace(image.Desc)
			}
			if caption == "" {
				caption = "证明材料图片"
			}
			captions = append(captions, caption)
		}
		parts = append(parts, fmt.Sprintf("可用图片 %d 张（%s）；需要在正文中展示时，另起一行输出占位标记 [[图:%s]]",
			len(card.Images), strings.Join(captions, "、"), card.Ref))
	}
	return "· " + strings.Join(parts, " | ")
}

func materialTypeLabel(materialType string) string {
	switch materialType {
	case "qualification":
		return "资质"
	case "performance":
		return "业绩"
	case "template":
		return "模板"
	default:
		return materialType
	}
}

// ── 证据组装 ────────────────────────────────────────────────────

// bidFactWhitelist 进入正文写作的招标事实白名单：只保留写作真正需要、
// 且必须全篇口径一致的关键事实，避免把全部字段广播到每一章。
var bidFactWhitelist = map[string]bool{
	"project_name":         true,
	"project_number":       true,
	"tenderer_name":        true,
	"procurement_scope":    true,
	"total_ceiling_amount": true,
	"bid_deadline":         true,
	"bid_validity_period":  true,
	"performance_security": true,
	"bid_security":         true,
	"procurement_method":   true,
}

// buildChapterEvidence 组装单章证据包。
func (s *svcImpl) buildChapterEvidence(ctx context.Context, proj *model.BidGenProject, node *model.BidGenOutline, ordered []*model.BidGenOutline, spec *chapterSpec) *chapterEvidence {
	ev := &chapterEvidence{}

	// 全篇章节标题：让模型知道别处已经写了什么，避免跨章重复
	for _, item := range ordered {
		if biddoc.IsDocumentRoot(item) {
			continue
		}
		ev.AllTitles = append(ev.AllTitles, fmt.Sprintf("%s %s", levelPrefix(item.Level), item.Title))
	}

	// 1. 关联条款：优先用大纲的 clause_ids，为空则按标题关键词召回
	clauses := s.chapterClauses(ctx, proj, node)
	for i, clause := range clauses {
		if clause == nil {
			continue
		}
		ev.Clauses = append(ev.Clauses, evidenceClause{
			Ref:     fmt.Sprintf("C%d", i+1),
			Title:   strings.TrimSpace(clause.Title),
			Content: truncateRunes(strings.TrimSpace(clause.Content), 600),
		})
	}

	// 2. 关键事实（白名单）
	snapshot := s.projectSnapshot(ctx, proj)
	if snapshot != nil {
		fieldByID := make(map[int64]*model.BidAnalysisV3Field, len(snapshot.Fields))
		for _, field := range snapshot.Fields {
			fieldByID[field.ID] = field
		}
		for _, value := range snapshot.FieldValues {
			if value.ValueStatus != "active" && value.ValueStatus != "suggestion" {
				continue
			}
			field := fieldByID[value.FieldID]
			if field == nil || !bidFactWhitelist[field.FieldKey] {
				continue
			}
			text := truncateRunes(strings.TrimSpace(value.DisplayValue), 300)
			if text == "" {
				continue
			}
			ev.Facts = append(ev.Facts, evidenceFact{
				Ref:   fmt.Sprintf("F%d", len(ev.Facts)+1),
				Name:  strings.TrimSpace(field.DisplayName),
				Value: text,
			})
		}
	}

	// 3. 评分标准表条目：规格已分配评分项时只带分配到的条目；
	//    未分配（或尚无规格）时带全表，保证模型仍能逐条响应。
	allScoring := s.loadScoringRows(ctx, proj)
	if len(allScoring) > 0 {
		if spec != nil && len(spec.ScoringRefs) > 0 {
			ev.Scoring = filterScoringRows(allScoring, spec.ScoringRefs)
		} else {
			ev.Scoring = allScoring
		}
	}

	// 4. 素材卡片
	ev.Materials = s.chapterMaterialCards(ctx, proj, node, spec)
	ev.reindex()

	// 回填规格中素材引用对应的真实素材 ID（供写回 outline.material_ids）
	if spec != nil {
		for _, ref := range spec.MaterialRefs {
			if card, ok := ev.material(ref); ok && !containsInt64(spec.MaterialIDs, card.ID) {
				spec.MaterialIDs = append(spec.MaterialIDs, card.ID)
			}
		}
	}
	return ev
}

func filterScoringRows(all []scoringRow, refs []string) []scoringRow {
	selected := make(map[string]bool, len(refs))
	for _, ref := range refs {
		selected[ref] = true
	}
	result := make([]scoringRow, 0, len(refs))
	for _, row := range all {
		if selected[row.Ref] {
			result = append(result, row)
		}
	}
	return result
}

func levelPrefix(level int32) string {
	switch level {
	case 1:
		return "一级"
	case 2:
		return "二级"
	case 3:
		return "三级"
	default:
		return "四级"
	}
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// chapterClauses 章节关联条款：clause_ids 优先，缺失时按标题关键词召回。
func (s *svcImpl) chapterClauses(ctx context.Context, proj *model.BidGenProject, node *model.BidGenOutline) []*model.BidAnalysisV3Clause {
	var pool []*model.BidAnalysisV3Clause
	if proj.TenderProjectID > 0 {
		if clauses, err := s.analysisSvc.Clauses(ctx, proj.TenderProjectID); err == nil {
			pool = clauses
		}
	}
	if len(pool) == 0 {
		if snapshot := s.projectSnapshot(ctx, proj); snapshot != nil {
			pool = snapshot.Clauses
		}
	}
	if len(pool) == 0 {
		return nil
	}

	var clauseIDs []int64
	if node.ClauseIds != "" {
		_ = json.Unmarshal([]byte(node.ClauseIds), &clauseIDs)
	}
	if len(clauseIDs) > 0 {
		byID := make(map[int64]*model.BidAnalysisV3Clause, len(pool))
		for _, clause := range pool {
			byID[clause.ID] = clause
		}
		result := make([]*model.BidAnalysisV3Clause, 0, len(clauseIDs))
		for _, id := range clauseIDs {
			if clause := byID[id]; clause != nil {
				result = append(result, clause)
			}
		}
		if len(result) > 0 {
			return limitClauses(result, 6)
		}
	}

	keywords := chapterKeywords(node.Title)
	if len(keywords) == 0 {
		return nil
	}
	type scored struct {
		clause *model.BidAnalysisV3Clause
		score  int
	}
	candidates := make([]scored, 0, len(pool))
	for _, clause := range pool {
		if clause == nil {
			continue
		}
		haystack := clause.Title + clause.Content
		score := 0
		for _, keyword := range keywords {
			if strings.Contains(haystack, keyword) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		switch clause.Importance {
		case "critical":
			score += 3
		case "high":
			score += 2
		case "medium":
			score++
		}
		candidates = append(candidates, scored{clause: clause, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].clause.ID < candidates[j].clause.ID
	})
	result := make([]*model.BidAnalysisV3Clause, 0, len(candidates))
	for _, item := range candidates {
		result = append(result, item.clause)
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func limitClauses(clauses []*model.BidAnalysisV3Clause, limit int) []*model.BidAnalysisV3Clause {
	if len(clauses) > limit {
		return clauses[:limit]
	}
	return clauses
}

// projectSnapshot 读取项目来源快照；失败时返回 nil 并降级。
func (s *svcImpl) projectSnapshot(ctx context.Context, proj *model.BidGenProject) *bidSourceSnapshotPayload {
	if proj == nil {
		return nil
	}
	snapshot, err := s.loadSourceSnapshot(ctx, proj.ID)
	if err != nil {
		return nil
	}
	return snapshot
}

// loadScoringRows 读取招标解析产出的“评分标准与评分办法”结构化表格。
func (s *svcImpl) loadScoringRows(ctx context.Context, proj *model.BidGenProject) []scoringRow {
	if proj == nil || proj.TenderProjectID <= 0 {
		return nil
	}
	db := s.repo.DB()
	if db == nil {
		return nil
	}
	var runID int64
	if err := db.WithContext(ctx).Table("bid_analysis_v3_project").
		Select("current_run_id").Where("id = ?", proj.TenderProjectID).Scan(&runID).Error; err != nil {
		return nil
	}
	if runID <= 0 {
		return nil
	}
	var row struct {
		NormalizedValueJSON string
		DisplayValue        string
	}
	err := db.WithContext(ctx).Table("bid_analysis_v3_field f").
		Select("v.normalized_value_json, v.display_value").
		Joins("JOIN bid_analysis_v3_field_value v ON v.field_id = f.id").
		Where("f.project_id = ? AND f.field_key = ? AND (v.run_id = ? OR v.origin = 'user') AND v.value_status IN ('active','suggestion')",
			proj.TenderProjectID, scoringFieldKey, runID).
		Order("v.id DESC").Limit(1).Scan(&row).Error
	if err != nil {
		return nil
	}

	// 优先使用模型整理的 Markdown 表格：它按评标口径归纳过，文本干净且列名明确；
	// 源表解析结果（normalized_value_json）常因 OCR 出现单元格截断与列错位，仅作兜底。
	rows := parsePipeTable(row.DisplayValue)
	if len(rows) < 2 {
		rows = rowsFromNormalizedJSON(row.NormalizedValueJSON)
	}
	return mapScoringRows(rows)
}

func rowsFromNormalizedJSON(raw string) [][]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !json.Valid([]byte(raw)) {
		return nil
	}
	var normalized map[string]any
	if err := json.Unmarshal([]byte(raw), &normalized); err != nil {
		return nil
	}
	dataJSON, _ := normalized["data_json"].(string)
	if strings.TrimSpace(dataJSON) == "" || !json.Valid([]byte(dataJSON)) {
		return nil
	}
	var payload struct {
		Rows [][]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &payload); err != nil {
		return nil
	}
	return payload.Rows
}

// mapScoringRows 把二维表映射为结构化评分行：先按表头列名匹配，匹配不到再按位置兜底。
func mapScoringRows(rows [][]string) []scoringRow {
	if len(rows) < 2 {
		return nil
	}
	header := rows[0]
	index := map[string]int{}
	for i, cell := range header {
		cleaned := strings.TrimSpace(cell)
		switch {
		case strings.Contains(cleaned, "序号"):
			index["index"] = i
		case strings.Contains(cleaned, "评分项") || strings.Contains(cleaned, "评审因素") || strings.Contains(cleaned, "评审内容"):
			if _, exists := index["item"]; !exists {
				index["item"] = i
			}
		case strings.Contains(cleaned, "分值") || strings.Contains(cleaned, "分数") || strings.Contains(cleaned, "权重"):
			index["score"] = i
		case strings.Contains(cleaned, "标准") || strings.Contains(cleaned, "细则"):
			index["standard"] = i
		case strings.Contains(cleaned, "响应"):
			index["response"] = i
		}
	}
	if _, ok := index["item"]; !ok {
		index["index"], index["item"], index["score"], index["standard"] = 0, 1, 2, 3
	}

	at := func(row []string, key string) string {
		position, ok := index[key]
		if !ok || position < 0 || position >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[position])
	}

	result := make([]scoringRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		item := at(row, "item")
		if item == "" {
			continue
		}
		result = append(result, scoringRow{
			Ref:      fmt.Sprintf("S%d", len(result)+1),
			Index:    at(row, "index"),
			Item:     truncateRunes(item, 120),
			Score:    at(row, "score"),
			Standard: truncateRunes(at(row, "standard"), 300),
			Response: truncateRunes(at(row, "response"), 200),
		})
	}
	return result
}

// ── 素材卡片 ────────────────────────────────────────────────────

// chapterMaterialCards 本章候选素材卡片。
// 顺序必须稳定：先按章节标题关键词召回（写作规格生成时模型看到的编号），
// 再追加“规格已选中但未召回”的素材，保证两次组装之间 M1..Mn 与真实素材一一对应。
func (s *svcImpl) chapterMaterialCards(ctx context.Context, proj *model.BidGenProject, node *model.BidGenOutline, spec *chapterSpec) []materialCard {
	ac := newGinCtx(ctx, proj.UserID)
	seen := make(map[int64]bool)
	var cards []materialCard

	add := func(id int64) {
		if id <= 0 || seen[id] || len(cards) >= 10 {
			return
		}
		seen[id] = true
		card := s.loadMaterialCard(ac, proj.UserID, id)
		if card == nil {
			return
		}
		cards = append(cards, *card)
	}

	keywords := chapterKeywords(node.Title)
	if len(keywords) > 0 {
		searchTerm := strings.Join(keywords, " ")
		for _, materialType := range []string{"qualification", "performance"} {
			results, _, err := s.mat.ListMaterials(ctx, material.ListParam{
				Type: materialType, Keyword: searchTerm, PageNum: 1, PageSize: 4, AuthUserID: proj.UserID,
			})
			if err != nil {
				continue
			}
			for _, item := range results {
				if item == nil {
					continue
				}
				add(item.ID)
			}
		}
	}

	// 规格已选中但关键词未召回到的素材，追加在候选之后（编号稳定）
	for _, id := range specMaterialIDList(spec) {
		add(id)
	}

	for i := range cards {
		cards[i].Ref = fmt.Sprintf("M%d", i+1)
	}
	return cards
}

func specMaterialIDList(spec *chapterSpec) []int64 {
	if spec == nil {
		return nil
	}
	return spec.MaterialIDs
}

// loadMaterialCard 聚合单个素材的卡片内容：标题 + 描述 + OCR 结构化信息 + 图片。
func (s *svcImpl) loadMaterialCard(ac *gin.Context, userID, materialID int64) *materialCard {
	summary, err := s.mat.LookupMaterialForUser(ac.Request.Context(), userID, materialID)
	if err != nil || summary == nil {
		return nil
	}
	card := &materialCard{
		ID:   summary.ID,
		Type: summary.Type,
		Name: strings.TrimSpace(summary.Name),
		Desc: truncateRunes(strings.TrimSpace(summary.Description), 300),
	}
	if card.Name == "" {
		return nil
	}

	if ocrResults, ocrErr := s.mat.GetOcrResultByMaterialID(ac, materialID); ocrErr == nil {
		for _, result := range ocrResults {
			if result == nil {
				continue
			}
			if card.Fields == nil && strings.TrimSpace(result.StructuredFields) != "" {
				fields := map[string]any{}
				if json.Unmarshal([]byte(result.StructuredFields), &fields) == nil {
					card.Fields = flattenMaterialFields(fields)
				}
			}
			if len(card.Labels) == 0 && strings.TrimSpace(result.Labels) != "" {
				for _, label := range strings.Split(result.Labels, ",") {
					if trimmed := strings.TrimSpace(label); trimmed != "" {
						card.Labels = append(card.Labels, trimmed)
					}
				}
			}
			if card.OCR == "" && strings.TrimSpace(result.OcrRawText) != "" {
				card.OCR = truncateRunes(compactWhitespace(result.OcrRawText), 400)
			}
		}
	}

	if images, imageErr := s.mat.ListMaterialImages(ac, materialID); imageErr == nil {
		for _, image := range images {
			if image == nil || strings.TrimSpace(image.ObjectKey) == "" {
				continue
			}
			card.Images = append(card.Images, materialImage{
				ID:        image.ID,
				ObjectKey: strings.TrimSpace(image.ObjectKey),
				Name:      strings.TrimSpace(image.Name),
				Desc:      truncateRunes(strings.TrimSpace(image.Description), 120),
			})
			if len(card.Images) >= 4 {
				break
			}
		}
	}
	return card
}

// flattenMaterialFields 把 OCR 结构化字段压平为“可读键 → 文本值”，仅保留有值的标量。
func flattenMaterialFields(fields map[string]any) map[string]string {
	result := make(map[string]string)
	for key, raw := range fields {
		if raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case string:
			if value := strings.TrimSpace(typed); value != "" {
				result[materialFieldLabel(key)] = truncateRunes(value, 200)
			}
		case float64:
			result[materialFieldLabel(key)] = strings.TrimSpace(fmt.Sprintf("%v", typed))
		case bool:
			result[materialFieldLabel(key)] = fmt.Sprintf("%v", typed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

var materialFieldLabels = map[string]string{
	"cert_type":         "证书类型",
	"cert_number":       "证书编号",
	"holder_name":       "持证人/单位",
	"issuing_authority": "发证机关",
	"valid_from":        "有效期起",
	"valid_to":          "有效期止",
	"scope":             "资质范围",
	"project_name":      "项目名称",
	"client":            "业主单位",
	"contract_amount":   "合同金额",
	"completion_date":   "完工时间",
	"summary":           "项目概况",
}

func materialFieldLabel(key string) string {
	if label, ok := materialFieldLabels[key]; ok {
		return label
	}
	return key
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// chapterKeywords 从标题提取检索关键词（去掉章节编号噪声后按词切分）。
func chapterKeywords(title string) []string {
	cleaned := strings.TrimSpace(title)
	replacer := strings.NewReplacer(
		"第", " ", "章", " ", "节", " ",
		"（", " ", "）", " ", "(", " ", ")", " ",
		"、", " ", "，", " ", ",", " ", "：", " ", ":", " ",
	)
	cleaned = replacer.Replace(cleaned)
	parts := strings.Fields(cleaned)
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if len([]rune(part)) < 2 {
			continue
		}
		keywords = append(keywords, part)
	}
	if len(keywords) == 0 {
		return nil
	}
	if len(keywords) > 4 {
		keywords = keywords[:4]
	}
	return keywords
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
