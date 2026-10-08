package bidgen

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
)

// bidCoverData 封面页字段：由后端从招标解析事实 + 企业信息装配，
// 前端导出时零拼装，保证封面口径与项目事实一致。
type bidCoverData struct {
	DocTitle      string `json:"docTitle"`
	ProjectName   string `json:"projectName"`
	ProjectNumber string `json:"projectNumber"`
	LotLabel      string `json:"lotLabel"`
	TendererName  string `json:"tendererName"`
	BidderName    string `json:"bidderName"`
	Date          string `json:"date"`
}

// buildCoverData 组装封面数据；缺失字段留空，由导出层兜底显示。
func (s *svcImpl) buildCoverData(c *gin.Context, proj *model.BidGenProject) *bidCoverData {
	if proj == nil {
		return nil
	}
	cover := &bidCoverData{
		DocTitle:    "投 标 文 件",
		ProjectName: strings.TrimSpace(proj.Name),
		Date:        formatCoverDate(time.Now()),
	}
	if facts := s.coverFacts(c, proj); len(facts) > 0 {
		cover.ProjectNumber = facts["project_number"]
		cover.TendererName = facts["tenderer_name"]
		cover.LotLabel = firstNonEmptyString(facts["procurement_scope"], facts["lot_amounts"])
		if cover.ProjectName == "" {
			cover.ProjectName = facts["project_name"]
		}
	}
	if proj.UserCompanyID > 0 {
		if name, err := s.mat.GetCompanyName(c, proj.UserCompanyID); err == nil {
			cover.BidderName = strings.TrimSpace(name)
		}
	}
	return cover
}

// coverFacts 从投标书来源快照读取封面所需的招标事实。
func (s *svcImpl) coverFacts(c *gin.Context, proj *model.BidGenProject) map[string]string {
	snapshot := s.projectSnapshot(c.Request.Context(), proj)
	if snapshot == nil {
		return nil
	}
	fieldByID := make(map[int64]*model.BidAnalysisV3Field, len(snapshot.Fields))
	for _, field := range snapshot.Fields {
		fieldByID[field.ID] = field
	}
	facts := make(map[string]string)
	for _, value := range snapshot.FieldValues {
		if value.ValueStatus != "active" && value.ValueStatus != "suggestion" {
			continue
		}
		field := fieldByID[value.FieldID]
		if field == nil || !bidFactWhitelist[field.FieldKey] {
			continue
		}
		if _, exists := facts[field.FieldKey]; exists {
			continue
		}
		facts[field.FieldKey] = truncateRunes(strings.TrimSpace(value.DisplayValue), 120)
	}
	return facts
}

// formatCoverDate 封面日期（如 2026 年 9 月 14 日）。
func formatCoverDate(value time.Time) string {
	return fmt.Sprintf("%d 年 %d 月 %d 日", value.Year(), int(value.Month()), value.Day())
}
