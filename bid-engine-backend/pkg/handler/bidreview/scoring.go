package bidreview

import (
	"context"
	"encoding/json"

	"bid-engine/pkg/db/model"
)

// ================================================================
// 竞争力评分对标：按评分办法逐条预估得分
// ================================================================

// dimensionStat 维度结论汇总
type dimensionStat struct {
	Dimension  string  `json:"dimension"`
	Label      string  `json:"label"`
	Total      int32   `json:"total"`
	Passed     int32   `json:"passed"`
	Warning    int32   `json:"warning"`
	Error      int32   `json:"error"`
	NotFound   int32   `json:"not_found"`
	NA         int32   `json:"na"`
	Pending    int32   `json:"pending"`
	ScoreTotal float64 `json:"score_total"`
	ScoreMax   float64 `json:"score_max"`
}

// summarizeDimensions 汇总各维度结论与评分
func summarizeDimensions(items []*model.BidReviewV2ChecklistItem, findings []*model.BidReviewV2Finding) []dimensionStat {
	findingByItem := make(map[int64]*model.BidReviewV2Finding, len(findings))
	for _, f := range findings {
		findingByItem[f.ChecklistItemID] = f
	}
	stats := make(map[string]*dimensionStat, len(Dimensions))
	order := make([]string, 0, len(Dimensions))
	for _, d := range Dimensions {
		stats[d] = &dimensionStat{Dimension: d, Label: dimensionLabelZH(d)}
		order = append(order, d)
	}
	for _, item := range items {
		st, ok := stats[item.Dimension]
		if !ok {
			st = &dimensionStat{Dimension: item.Dimension, Label: dimensionLabelZH(item.Dimension)}
			stats[item.Dimension] = st
			order = append(order, item.Dimension)
		}
		st.Total++
		finding := findingByItem[item.ID]
		status := ""
		if finding != nil {
			status = finding.Status
		}
		switch status {
		case FindingPass:
			st.Passed++
		case FindingWarning:
			st.Warning++
		case FindingError:
			st.Error++
		case FindingNA:
			st.NA++
		case "":
			// 尚无判定结论（人工自定义项 / 待复检项）
			st.Pending++
		default:
			st.NotFound++
		}
		if item.Dimension != DimensionCompetitiveness {
			continue
		}
		full := itemFullScore(item)
		if status == FindingNA || full <= 0 {
			continue
		}
		st.ScoreMax += full
		switch status {
		case FindingPass:
			st.ScoreTotal += full
		case FindingWarning:
			st.ScoreTotal += full * 0.5
		}
	}
	out := make([]dimensionStat, 0, len(order))
	for _, d := range order {
		if st, ok := stats[d]; ok {
			out = append(out, *st)
		}
	}
	return out
}

// itemFullScore 读取清单项满分（评分对标项）
func itemFullScore(item *model.BidReviewV2ChecklistItem) float64 {
	if item == nil || item.OriginJSON == "" {
		return 0
	}
	var meta originMeta
	if err := json.Unmarshal([]byte(item.OriginJSON), &meta); err != nil {
		return 0
	}
	return meta.FullScore
}

// computeScorecard 计算总分与满分
func computeScorecard(stats []dimensionStat) (float64, float64) {
	total, max := 0.0, 0.0
	for _, st := range stats {
		total += st.ScoreTotal
		max += st.ScoreMax
	}
	return total, max
}

func dimensionLabelZH(dimension string) string {
	switch dimension {
	case DimensionCompliance:
		return "合规性"
	case DimensionCompleteness:
		return "完整性"
	case DimensionCompetitiveness:
		return "竞争力"
	case DimensionFormat:
		return "暗标版式"
	default:
		return dimension
	}
}

// buildScorecard 读取清单与判定，产出维度汇总（供 finalize 与详情接口复用）
func (s *svcImpl) buildScorecard(ctx context.Context, projectID int64) ([]dimensionStat, float64, float64, error) {
	items, err := s.repo.GetChecklistItems(ctx, projectID)
	if err != nil {
		return nil, 0, 0, err
	}
	findings, err := s.repo.GetLatestFindings(ctx, projectID)
	if err != nil {
		return nil, 0, 0, err
	}
	stats := summarizeDimensions(items, findings)
	total, max := computeScorecard(stats)
	return stats, total, max, nil
}
