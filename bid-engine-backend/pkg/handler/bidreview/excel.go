package bidreview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
)

var findingStatusLabels = map[string]string{
	FindingPass:     "通过",
	FindingWarning:  "需整改",
	FindingError:    "高风险",
	FindingNA:       "不适用",
	FindingNotFound: "未找到依据",
}

var dimensionSheetNames = map[string]string{
	DimensionCompliance:      "合规性",
	DimensionCompleteness:    "完整性",
	DimensionCompetitiveness: "竞争力",
	DimensionFormat:          "暗标版式",
}

// ExportReport 导出审核报告（多 sheet Excel）
func (s *svcImpl) ExportReport(c *gin.Context) {
	projectID := parseInt64(c.Query("project_id"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id 不能为空"})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.GetProjectForUser(ctx, userID, projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	items, err := s.repo.GetChecklistItems(ctx, projectID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询检查项失败: " + err.Error()})
		return
	}
	findings, _ := s.repo.GetLatestFindings(ctx, projectID)
	evidences, _ := s.repo.GetEvidencesByProject(ctx, projectID)
	remediations, _ := s.repo.GetRemediations(ctx, projectID)
	stats, scoreTotal, scoreMax, _ := s.buildScorecard(ctx, projectID)

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1E3A5F"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// ── 概览 ──
	overview := "审核概览"
	_ = f.SetSheetName("Sheet1", overview)
	overviewRows := [][]interface{}{
		{"项目名称", proj.Name},
		{"审核状态", projectStatusLabel(proj.Status)},
		{"暗标评审", boolLabel(proj.IsAnonymous)},
		{"检查项总数", proj.TotalItems},
		{"通过 / 需整改 / 高风险", fmt.Sprintf("%d / %d / %d", proj.PassedItems, proj.WarningItems, proj.ErrorItems)},
		{"未闭环整改", proj.TodoItems},
		{"竞争力预估得分", fmt.Sprintf("%.1f / %.1f", scoreTotal, scoreMax)},
		{"报告生成时间", time.Now().Format("2006-01-02 15:04:05")},
		{},
		{"维度", "检查项", "通过", "需整改", "高风险", "未找到依据", "不适用", "预估得分", "满分"},
	}
	for _, st := range stats {
		overviewRows = append(overviewRows, []interface{}{
			st.Label, st.Total, st.Passed, st.Warning, st.Error, st.NotFound, st.NA, st.ScoreTotal, st.ScoreMax,
		})
	}
	writeRows(f, overview, overviewRows)
	_ = f.SetColWidth(overview, "A", "A", 24)
	_ = f.SetColWidth(overview, "B", "I", 18)

	findingByItem := make(map[int64]*model.BidReviewV2Finding, len(findings))
	for _, fd := range findings {
		findingByItem[fd.ChecklistItemID] = fd
	}
	evidencesByItem := make(map[int64][]*model.BidReviewV2Evidence, len(items))
	for _, e := range evidences {
		evidencesByItem[e.ChecklistItemID] = append(evidencesByItem[e.ChecklistItemID], e)
	}

	// ── 各维度明细 ──
	headers := []string{"序号", "分类", "检查项", "判定口径", "期望证据", "风险级别", "审核结论", "判定理由", "整改建议", "来源", "招标页码", "投标页码", "人工确认"}
	for _, dim := range bidreviewRepo.Dimensions {
		sheet := dimensionSheetNames[dim]
		if sheet == "" {
			sheet = dim
		}
		rows := make([][]interface{}, 0)
		rows = append(rows, stringsToInterfaces(headers))
		idx := 0
		for _, item := range items {
			if item.Dimension != dim {
				continue
			}
			idx++
			finding := findingByItem[item.ID]
			status, severity, reason, suggestion := "待判定", item.Severity, "", ""
			if finding != nil {
				status = firstNonEmpty(findingStatusLabels[finding.Status], finding.Status)
				severity = finding.Severity
				reason = finding.Reason
				suggestion = finding.Suggestion
			}
			bidPages := make([]string, 0, 3)
			for _, e := range evidencesByItem[item.ID] {
				if e.Side == "bid" && e.PageNo > 0 {
					bidPages = append(bidPages, fmt.Sprintf("P%d", e.PageNo))
				}
			}
			rows = append(rows, []interface{}{
				idx, item.Category, item.Title, item.Requirement, item.ExpectedEvidence,
				severityLabel(severity), status, reason, suggestion, sourceLabel(item.Source),
				pageLabel(item.TenderPage), strings.Join(bidPages, "、"), reviewStatusLabel(item.ReviewStatus),
			})
		}
		if idx == 0 {
			rows = append(rows, []interface{}{"-", "-", "本维度无检查项", "", "", "", "", "", "", "", "", "", ""})
		}
		writeRows(f, sheet, rows)
		_ = f.SetCellStyle(sheet, "A1", mustCellName(len(headers), 1), headerStyle)
		widths := []float64{6, 12, 34, 44, 26, 10, 10, 40, 34, 10, 10, 12, 10}
		for i, w := range widths {
			colName, _ := excelize.ColumnNumberToName(i + 1)
			_ = f.SetColWidth(sheet, colName, colName, w)
		}
	}

	// ── 整改清单 ──
	remediationSheet := "整改清单"
	remediationRows := [][]interface{}{stringsToInterfaces([]string{"序号", "维度", "整改项", "风险级别", "整改建议", "状态", "责任人ID", "备注", "完成时间"})}
	for i, r := range remediations {
		resolved := ""
		if r.ResolvedAt != nil {
			resolved = r.ResolvedAt.Format("2006-01-02 15:04:05")
		}
		remediationRows = append(remediationRows, []interface{}{
			i + 1, dimensionSheetNames[r.Dimension], r.Title, severityLabel(r.Severity),
			r.Suggestion, remediationStatusLabel(r.Status), r.OwnerUserID, r.Note, resolved,
		})
	}
	writeRows(f, remediationSheet, remediationRows)
	_ = f.SetCellStyle(remediationSheet, "A1", "I1", headerStyle)

	// ── 证据索引 ──
	evidenceSheet := "证据索引"
	evidenceRows := [][]interface{}{stringsToInterfaces([]string{"序号", "检查项", "侧", "文件", "页码", "证据引文"})}
	itemTitle := make(map[int64]string, len(items))
	for _, it := range items {
		itemTitle[it.ID] = it.Title
	}
	for i, e := range evidences {
		side := "投标文件"
		if e.Side == "tender" {
			side = "招标文件"
		}
		evidenceRows = append(evidenceRows, []interface{}{
			i + 1, itemTitle[e.ChecklistItemID], side, e.FileName, e.PageNo, e.Quote,
		})
	}
	writeRows(f, evidenceSheet, evidenceRows)
	_ = f.SetCellStyle(evidenceSheet, "A1", "F1", headerStyle)

	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "生成Excel失败: " + err.Error()})
		return
	}
	fileName := fmt.Sprintf("%s-审核报告-%s.xlsx", proj.Name, time.Now().Format("20060102-150405"))
	objectKey := fmt.Sprintf("bid-review/%d/export/%d-report.xlsx", projectID, time.Now().UnixNano())
	_ = os.MkdirAll("./tmp", 0o777)
	tmpPath := filepath.Join("./tmp", fmt.Sprintf("bid-review-export-%d-%d.xlsx", projectID, time.Now().UnixNano()))
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "写入导出文件失败: " + err.Error()})
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()
	if err := s.oss.Put(ctx, objectKey, tmpPath); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "上传导出文件失败: " + err.Error()})
		return
	}
	_ = s.repo.AddExportRecord(ctx, &model.BidReviewV2ExportRecord{
		ProjectID: projectID, Kind: "report", FileName: fileName,
		FileBucket: s.oss.GetDefaultBucketName(), FileObject: objectKey, FileURL: objectKey, UserID: userID,
	})
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: projectID, Action: "export", OperatorID: userID, Detail: "导出审核报告 Excel",
	})
	downloadURL := objectKey
	if u, e := s.oss.GetPresignedURL(entity.ConvertContext(c), objectKey, 24*time.Hour); e == nil && u != nil {
		downloadURL = u.String()
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"file_name": fileName, "file_url": downloadURL}})
}

func writeRows(f *excelize.File, sheet string, rows [][]interface{}) {
	if _, err := f.NewSheet(sheet); err != nil && !strings.Contains(err.Error(), "already exists") {
		return
	}
	for i, row := range rows {
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
}

func stringsToInterfaces(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

func projectStatusLabel(status string) string {
	switch status {
	case bidreviewRepo.ProjectStatusRunning:
		return "审核中"
	case bidreviewRepo.ProjectStatusSucceed:
		return "已完成"
	case bidreviewRepo.ProjectStatusFailed:
		return "审核失败"
	default:
		return status
	}
}

func boolLabel(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

func severityLabel(severity string) string {
	switch severity {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	default:
		return ""
	}
}

func sourceLabel(source string) string {
	switch source {
	case bidreviewRepo.SourcePreset:
		return "通用清单"
	case bidreviewRepo.SourceAnalysis:
		return "解析依据"
	case bidreviewRepo.SourceRule:
		return "企业规则"
	case bidreviewRepo.SourceUser:
		return "自定义"
	case bidreviewRepo.SourceFormat:
		return "暗标版式"
	default:
		return source
	}
}

func reviewStatusLabel(status string) string {
	switch status {
	case bidreviewRepo.CheckConfirmed:
		return "已确认"
	case bidreviewRepo.CheckRejected:
		return "已驳回"
	default:
		return "待处理"
	}
}

func remediationStatusLabel(status string) string {
	switch status {
	case bidreviewRepo.RemediationDoing:
		return "整改中"
	case bidreviewRepo.RemediationDone:
		return "已完成"
	case bidreviewRepo.RemediationIgnored:
		return "已忽略"
	default:
		return "待整改"
	}
}

func pageLabel(page int32) string {
	if page <= 0 {
		return ""
	}
	return fmt.Sprintf("P%d", page)
}

func mustCellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
