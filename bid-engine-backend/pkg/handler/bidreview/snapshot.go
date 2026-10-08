package bidreview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
)

// ================================================================
// 招标解析依据：绑定 → 等待完成 → 冻结快照
// ================================================================

type snapshotField struct {
	FileID      int64  `json:"file_id,omitempty"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	ValueType   string `json:"value_type"`
	Value       string `json:"value"`
	Page        int    `json:"page"`
	Quote       string `json:"quote"`
}

type snapshotScoringRow struct {
	FileID   int64  `json:"file_id,omitempty"`
	Page     int    `json:"page,omitempty"`
	Quote    string `json:"quote,omitempty"`
	Item     string `json:"item"`
	Score    string `json:"score"`
	Criteria string `json:"criteria"`
	Response string `json:"response"`
}

type snapshotClause struct {
	FileID     int64  `json:"file_id,omitempty"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Importance string `json:"importance"`
	Page       int    `json:"page"`
	Quote      string `json:"quote"`
}

type snapshotWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// analysisSnapshot 招标解析依据快照（审核结论的可追溯基准）
type analysisSnapshot struct {
	AnalysisProjectID int64                `json:"analysis_project_id"`
	AnalysisRunID     int64                `json:"analysis_run_id"`
	ProjectName       string               `json:"project_name"`
	Fields            []snapshotField      `json:"fields"`
	ScoringRows       []snapshotScoringRow `json:"scoring_rows"`
	Clauses           []snapshotClause     `json:"clauses"`
	Warnings          []snapshotWarning    `json:"warnings"`
	FrozenAt          time.Time            `json:"frozen_at"`
}

// ensureAnalysisBinding 等待已有来源的招标解析运行；未绑定来源的审核项目由专用流程处理。
func (s *svcImpl) ensureAnalysisBinding(ctx context.Context, proj *model.BidReviewV2Project) (int64, int64, error) {
	analysisProjectID := proj.AnalysisProjectID
	analysisRunID := proj.AnalysisRunID
	if analysisProjectID <= 0 {
		return 0, 0, fmt.Errorf("审核项目没有可复用的招标解析来源")
	}

	effectiveRunID, err := s.ensureAnalysisRun(ctx, proj, analysisProjectID, analysisRunID)
	if err != nil {
		return analysisProjectID, effectiveRunID, err
	}
	if err := s.waitAnalysisReady(ctx, analysisProjectID, effectiveRunID); err != nil {
		return analysisProjectID, effectiveRunID, err
	}
	if effectiveRunID != proj.AnalysisRunID {
		if err := s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{"analysis_run_id": effectiveRunID}); err != nil {
			return analysisProjectID, effectiveRunID, err
		}
		proj.AnalysisRunID = effectiveRunID
	}
	return analysisProjectID, effectiveRunID, nil
}

// ensureAnalysisRun 使用已有来源的解析运行：
//   - 已成功：复用当前运行
//   - 运行中/暂停：等待既有运行
//   - 已失败：返回错误，由审核侧按来源决定是否改走专用解析
func (s *svcImpl) ensureAnalysisRun(ctx context.Context, proj *model.BidReviewV2Project, analysisProjectID, boundRunID int64) (int64, error) {
	analysisProj, err := s.analysisRepo.Project(ctx, analysisProjectID, 0)
	if err != nil {
		return boundRunID, fmt.Errorf("查询招标解析项目失败: %w", err)
	}

	switch analysisProj.Status {
	case repov3.ProjectSucceeded, repov3.ProjectSucceededWithWarnings:
		if run, rErr := s.analysisRepo.CurrentRun(ctx, analysisProjectID); rErr == nil && run != nil {
			return run.ID, nil
		}
		return boundRunID, nil
	case repov3.ProjectRunning, repov3.ProjectPaused:
		if run, rErr := s.analysisRepo.CurrentRun(ctx, analysisProjectID); rErr == nil && run != nil {
			s.logger.Infow("招标解析进行中，审核等待其完成", "project_id", proj.ID, "analysis_project_id", analysisProjectID, "run_id", run.ID)
			return run.ID, nil
		}
		return boundRunID, nil
	}

	// 解析已失败（或状态异常）
	if !analysisProj.IsInternal {
		return boundRunID, asNonRetryable(fmt.Errorf(
			"招标解析项目“%s”解析未成功（%s），请先在招标解析模块重新解析后再重试审核",
			analysisProj.Name, firstNonEmpty(strings.TrimSpace(analysisProj.LastError), analysisProj.Status)))
	}

	return boundRunID, fmt.Errorf("来源招标解析结果不可用（%s）", firstNonEmpty(strings.TrimSpace(analysisProj.LastError), analysisProj.Status))
}

// waitAnalysisReady 轮询等待解析完成（超时返回可重试错误，交回 Worker 退避后继续等待）
func (s *svcImpl) waitAnalysisReady(ctx context.Context, analysisProjectID, runID int64) error {
	const (
		pollInterval = 10 * time.Second
		maxWait      = 30 * time.Minute
	)
	deadline := time.Now().Add(maxWait)
	for {
		proj, err := s.analysisRepo.Project(ctx, analysisProjectID, 0)
		if err != nil {
			return fmt.Errorf("查询招标解析项目失败: %w", err)
		}
		switch proj.Status {
		case repov3.ProjectSucceeded, repov3.ProjectSucceededWithWarnings:
			return nil
		case repov3.ProjectFailed:
			reason := strings.TrimSpace(proj.LastError)
			if runID > 0 {
				if run, rErr := s.analysisRepo.Run(ctx, runID); rErr == nil && run != nil && strings.TrimSpace(run.LastError) != "" {
					reason = strings.TrimSpace(run.LastError)
				}
			}
			return asNonRetryable(fmt.Errorf(
				"招标文件解析未成功（%s）；请检查“系统管理-模型配置”后，在审核详情页重试“招标文件解析”阶段",
				firstNonEmpty(reason, "解析失败")))
		}
		if runID > 0 {
			if run, rErr := s.analysisRepo.Run(ctx, runID); rErr == nil && run != nil && run.Status == repov3.ProjectFailed {
				return asNonRetryable(fmt.Errorf("招标解析运行失败，请先在招标解析模块修复后重试审核"))
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待招标解析完成超时，稍后将自动重试")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// loadAnalysisSnapshot 从解析库读取审核依据（字段 / 评分表 / 条款 / 告警）
func (s *svcImpl) loadAnalysisSnapshot(ctx context.Context, projectID int64) (*analysisSnapshot, error) {
	snap := &analysisSnapshot{FrozenAt: time.Now()}
	db := s.analysisRepo.DB()

	type fieldRow struct {
		FieldKey    string
		DisplayName string
		CategoryKey string
		ValueType   string
		DisplayVal  string
	}
	fieldSQL := `SELECT f.field_key AS field_key, f.display_name AS display_name, f.category_key AS category_key,
			f.value_type AS value_type, COALESCE(v.display_value,'') AS display_val
		FROM bid_analysis_v3_field_value v
		JOIN bid_analysis_v3_field f ON f.id = v.field_id
		WHERE v.project_id = ? AND v.value_status = 'active'
		ORDER BY f.category_key, f.id`
	rows := make([]fieldRow, 0, 40)
	if err := db.WithContext(ctx).Raw(fieldSQL, projectID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取解析字段失败: %w", err)
	}
	// 证据页码：按 field_value 取首条证据
	evidencePage := make(map[string]int, len(rows))
	evidenceQuote := make(map[string]string, len(rows))
	type evRow struct {
		FieldKey string
		PageNo   int32
		Quote    string
	}
	evSQL := `SELECT f.field_key AS field_key, e.page_no AS page_no, COALESCE(e.quote,'') AS quote
		FROM bid_analysis_v3_field_value_evidence e
		JOIN bid_analysis_v3_field_value v ON v.id = e.field_value_id
		JOIN bid_analysis_v3_field f ON f.id = v.field_id
		WHERE e.project_id = ? AND v.value_status = 'active'
		ORDER BY e.sort_order`
	evs := make([]evRow, 0, 40)
	if err := db.WithContext(ctx).Raw(evSQL, projectID).Scan(&evs).Error; err != nil {
		return nil, fmt.Errorf("读取解析证据失败: %w", err)
	}
	for _, e := range evs {
		if _, ok := evidencePage[e.FieldKey]; !ok {
			evidencePage[e.FieldKey] = int(e.PageNo)
			evidenceQuote[e.FieldKey] = strings.TrimSpace(e.Quote)
		}
	}
	for _, r := range rows {
		snap.Fields = append(snap.Fields, snapshotField{
			Key: r.FieldKey, DisplayName: r.DisplayName, Category: r.CategoryKey, ValueType: r.ValueType,
			Value: strings.TrimSpace(r.DisplayVal), Page: evidencePage[r.FieldKey], Quote: evidenceQuote[r.FieldKey],
		})
		if r.FieldKey == "scoring_criteria_table" {
			snap.ScoringRows = parseScoringTable(r.DisplayVal)
		}
	}

	type clauseRow struct {
		Title      string
		Content    string
		Importance string
		PageNo     int32
		Quote      string
	}
	// 取“每条条款一条证据”：直接 JOIN 证据表会按证据条数放大条款行，
	// 导致清单 item_key 重复（同一语句内重复键会触发 MySQL 1869）。
	clauseSQL := `SELECT c.title AS title, COALESCE(c.content,'') AS content, c.importance AS importance,
			COALESCE((SELECT e.page_no FROM bid_analysis_v3_clause_evidence e
				WHERE e.clause_id = c.id ORDER BY e.sort_order, e.id LIMIT 1), 0) AS page_no,
			COALESCE((SELECT e.quote FROM bid_analysis_v3_clause_evidence e
				WHERE e.clause_id = c.id ORDER BY e.sort_order, e.id LIMIT 1), '') AS quote
		FROM bid_analysis_v3_clause c
		WHERE c.project_id = ?
		ORDER BY c.sort_order, c.id`
	clauses := make([]clauseRow, 0, 80)
	if err := db.WithContext(ctx).Raw(clauseSQL, projectID).Scan(&clauses).Error; err != nil {
		return nil, fmt.Errorf("读取解析条款失败: %w", err)
	}
	for _, c := range clauses {
		snap.Clauses = append(snap.Clauses, snapshotClause{
			Title: strings.TrimSpace(c.Title), Content: strings.TrimSpace(c.Content),
			Importance: c.Importance, Page: int(c.PageNo), Quote: strings.TrimSpace(c.Quote),
		})
	}

	warns := make([]*model.BidAnalysisV3Warning, 0, 20)
	if err := db.WithContext(ctx).Where("project_id = ? AND resolved = 0", projectID).
		Order("id").Find(&warns).Error; err != nil {
		return nil, fmt.Errorf("读取解析告警失败: %w", err)
	}
	for _, w := range warns {
		snap.Warnings = append(snap.Warnings, snapshotWarning{Code: w.Code, Severity: w.Severity, Message: w.Message})
	}
	return snap, nil
}

// parseScoringTable 解析评分办法 markdown 表格为结构化行
func parseScoringTable(markdown string) []snapshotScoringRow {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return nil
	}
	// 解析结果常被压成一行（换行丢失），此时按 | 切分并逐 5 列成行；
	// 仍是标准多行 markdown 时沿用按行解析，二者都支持。
	if !strings.Contains(text, "\n") {
		return parseScoringRowsFlat(text)
	}
	lines := strings.Split(text, "\n")
	return parseScoringRowsByLine(lines)
}

// parseScoringRowsByLine 标准多行 markdown 表格解析
func parseScoringRowsByLine(lines []string) []snapshotScoringRow {
	rows := make([]snapshotScoringRow, 0, 24)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitMarkdownRow(line)
		if len(cells) == 0 {
			continue
		}
		joined := strings.Join(cells, "")
		if strings.Contains(joined, "---") || joined == "" {
			continue
		}
		if strings.Contains(cells[0], "序号") && strings.Contains(joined, "评分") {
			continue
		}
		row := snapshotScoringRow{}
		switch {
		case len(cells) >= 5:
			row.Item = cells[1]
			row.Score = cells[2]
			row.Criteria = cells[3]
			row.Response = cells[4]
		case len(cells) == 4:
			row.Item = cells[1]
			row.Score = cells[2]
			row.Criteria = cells[3]
		default:
			row.Item = cells[0]
			if len(cells) > 1 {
				row.Criteria = strings.Join(cells[1:], " ")
			}
		}
		if strings.TrimSpace(row.Item) == "" && strings.TrimSpace(row.Criteria) == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func splitMarkdownRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	// 去掉尾部空单元格（markdown 表格常见尾竖线）
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// parseScoringRowsFlat 解析被压成一行的评分表（解析结果常丢失换行）。
// 结构形如：| 序号 | 评分项 | 分值 | 评分标准 | 响应要求 | ...（每行 5 列，行间以空单元分隔）
func parseScoringRowsFlat(text string) []snapshotScoringRow {
	cells := strings.Split(text, "|")
	pending := make([]string, 0, 5)
	rows := make([]snapshotScoringRow, 0, 24)

	emit := func(cols []string) {
		// 列顺序：0=序号 1=评分项 2=分值 3=评分标准 4=响应要求
		seq := strings.TrimSpace(cols[0])
		item := strings.TrimSpace(cols[1])
		score := strings.TrimSpace(cols[2])
		criteria := strings.TrimSpace(cols[3])
		response := strings.TrimSpace(cols[4])
		// 表头 / 分隔行跳过
		if seq == "序号" || strings.Contains(seq, "序号") || strings.Contains(item, "评分项") {
			return
		}
		if strings.HasPrefix(seq, "---") || strings.HasPrefix(item, "---") {
			return
		}
		if item == "" && criteria == "" {
			return
		}
		rows = append(rows, snapshotScoringRow{Item: item, Score: score, Criteria: criteria, Response: response})
	}

	for _, raw := range cells {
		cell := strings.TrimSpace(raw)
		if cell == "" {
			// 行边界：已凑满 5 列时空单元是行分隔符，直接跳过
			if len(pending) == 5 {
				emit(pending)
				pending = pending[:0]
			} else if len(pending) > 0 {
				// 行内空单元（如缺"响应要求"）保留占位，避免列错位
				pending = append(pending, "")
			}
			continue
		}
		// 整行分隔符（|---|---|）按原顺序收集，交由 emit 过滤
		pending = append(pending, cell)
		if len(pending) == 5 {
			emit(pending)
			pending = pending[:0]
		}
	}
	if len(pending) == 5 {
		emit(pending)
	}
	return rows
}

// freezeAnalysisSnapshot 生成并落库审核依据快照
func (s *svcImpl) freezeAnalysisSnapshot(ctx context.Context, proj *model.BidReviewV2Project) (*analysisSnapshot, error) {
	snap, err := s.loadAnalysisSnapshot(ctx, proj.AnalysisProjectID)
	if err != nil {
		return nil, err
	}
	snap.AnalysisProjectID = proj.AnalysisProjectID
	snap.AnalysisRunID = proj.AnalysisRunID
	analysisProj, pErr := s.analysisRepo.Project(ctx, proj.AnalysisProjectID, 0)
	if pErr == nil && analysisProj != nil {
		snap.ProjectName = analysisProj.Name
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSnapshot(ctx, &model.BidReviewV2AnalysisSnapshot{
		ProjectID:         proj.ID,
		AnalysisProjectID: proj.AnalysisProjectID,
		AnalysisRunID:     proj.AnalysisRunID,
		SnapshotJSON:      string(raw),
	}); err != nil {
		return nil, err
	}
	return snap, nil
}

// loadFrozenSnapshot 读取已冻结快照（清单生成使用）
func (s *svcImpl) loadFrozenSnapshot(ctx context.Context, projectID int64) (*analysisSnapshot, error) {
	record, err := s.repo.GetSnapshot(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var snap analysisSnapshot
	if err := json.Unmarshal([]byte(record.SnapshotJSON), &snap); err != nil {
		return nil, fmt.Errorf("解析审核依据快照失败: %w", err)
	}
	return &snap, nil
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}
