package bidanalysisv3

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"bid-engine/pkg/db/model"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
)

type warningGroupRow struct {
	ID               int64               `json:"id"`
	GroupKey         string              `json:"group_key"`
	Code             string              `json:"code"`
	Stage            string              `json:"stage"`
	Severity         string              `json:"severity"`
	Count            int                 `json:"count"`
	Occurrences      int                 `json:"occurrences"`
	SampleMessage    string              `json:"sample_message"`
	AffectedChapters []warningChapterRef `json:"affected_chapters"`
	Reasons          map[string]int      `json:"reasons,omitempty"`
	RetryTargetStage string              `json:"retry_target_stage"`
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

// warningRetryStage 返回告警组可重试的目标阶段；不可重试返回空串。
func warningRetryStage(code, stage string) string {
	switch code {
	case "chapter_extract_failed", "dynamic_second_pass_failed", "candidate_validation_failed", "global_fixed_extract_failed", "fixed_field_evidence_invalid":
		return "chapter_fact_extracting"
	case "chapter_fallback", "chapter_outline_refine_failed":
		return "chapter_identifying"
	case "ai_interpretation_failed", "conflict_resolution_failed", "dynamic_fields_below_target", "dynamic_fields_ranked":
		return "chapter_fact_extracting"
	case "summary_failed", "summary_llm_fallback":
		return "document_summary"
	case "stage_skipped_by_user":
		// 跳过的阶段允许“补跑”：重新执行该阶段及其后续所有阶段。
		return stage
	}
	return ""
}

// aggregateWarnings 按去重分组键聚合未解决告警，供详情接口展示。
// 旧数据 group_key 为空时回退 code+stage+message 作为分组键。
func aggregateWarnings(warnings []*model.BidAnalysisV3Warning) []warningGroupRow {
	type group struct {
		row      warningGroupRow
		severity int
	}
	groups := map[string]*group{}
	order := make([]string, 0)
	for _, warning := range warnings {
		if warning == nil || warning.Resolved {
			continue
		}
		key := strings.TrimSpace(warning.GroupKey)
		if key == "" {
			key = "legacy:" + warning.Code + ":" + warning.Stage + ":" + hashText(warning.Message)
		}
		g := groups[key]
		if g == nil {
			severity := warning.Severity
			if severity == "" {
				severity = "warning"
			}
			g = &group{
				row: warningGroupRow{
					ID: warning.ID, GroupKey: key, Code: warning.Code, Stage: warning.Stage, Severity: severity,
					Count: 0, Occurrences: 0, SampleMessage: warning.Message, Reasons: map[string]int{},
					RetryTargetStage: warningRetryStage(warning.Code, warning.Stage),
				},
				severity: severityRank(severity),
			}
			groups[key] = g
			order = append(order, key)
		}
		g.row.Count++
		g.row.SampleMessage = warning.Message
		if rank := severityRank(warning.Severity); rank > g.severity {
			g.severity = rank
			g.row.Severity = warning.Severity
		}
		occurrences := 1
		if warning.DetailJSON != nil && strings.TrimSpace(*warning.DetailJSON) != "" {
			var detail struct {
				Occurrences int                 `json:"occurrences"`
				Chapters    []warningChapterRef `json:"chapters"`
				Reasons     map[string]int      `json:"reasons"`
			}
			if json.Unmarshal([]byte(*warning.DetailJSON), &detail) == nil {
				if detail.Occurrences > 0 {
					occurrences = detail.Occurrences
				}
				for _, chapter := range detail.Chapters {
					g.row.AffectedChapters = appendUniqueWarningChapter(g.row.AffectedChapters, chapter)
				}
				for reason, n := range detail.Reasons {
					g.row.Reasons[reason] += n
				}
			}
		}
		g.row.Occurrences += occurrences
	}
	rows := make([]warningGroupRow, 0, len(order))
	for _, key := range order {
		rows = append(rows, groups[key].row)
	}
	for i := range rows {
		if rows[i].Code == "candidate_validation_failed" && len(rows[i].Reasons) > 0 {
			rows[i].SampleMessage = formatCandidateValidationMessage(rows[i].Occurrences, rows[i].Reasons)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if severityRank(rows[i].Severity) != severityRank(rows[j].Severity) {
			return severityRank(rows[i].Severity) > severityRank(rows[j].Severity)
		}
		if rows[i].Occurrences != rows[j].Occurrences {
			return rows[i].Occurrences > rows[j].Occurrences
		}
		return rows[i].Code < rows[j].Code
	})
	return rows
}

// formatCandidateValidationMessage 依据被忽略候选数与原因分布生成告警摘要。
func formatCandidateValidationMessage(total int, reasons map[string]int) string {
	type reasonCount struct {
		name  string
		count int
	}
	items := make([]reasonCount, 0, len(reasons))
	for reason, n := range reasons {
		items = append(items, reasonCount{name: reason, count: n})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].name < items[j].name
	})
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s %d 个", item.name, item.count))
	}
	return fmt.Sprintf("%d 个候选因字段或证据校验失败而被忽略：%s", total, strings.Join(parts, "、"))
}

// pruneRetryTargets 依据项目状态与目标阶段真实状态裁剪阶段级重试能力：
// 只有“失败”或“完成但有告警”的项目，且目标阶段处于失败/部分完成时，才下发重试阶段。
// 否则前端不展示“重试该阶段”，避免出现点击后被后端拒绝的无效操作。
func pruneRetryTargets(rows []warningGroupRow, projectStatus string, stageRuns []*model.BidAnalysisV3StageRun) {
	allowProject := projectStatus == repov3.ProjectFailed ||
		projectStatus == repov3.ProjectSucceededWithWarnings ||
		projectStatus == repov3.ProjectSucceeded
	statusByStage := make(map[string]string, len(stageRuns))
	for _, sr := range stageRuns {
		if sr != nil {
			statusByStage[sr.Stage] = sr.Status
		}
	}
	for i := range rows {
		target := rows[i].RetryTargetStage
		if target == "" {
			continue
		}
		if !allowProject {
			rows[i].RetryTargetStage = ""
			continue
		}
		status := statusByStage[target]
		if status != repov3.StageFailed && status != repov3.StagePartial && status != repov3.StageSkipped {
			rows[i].RetryTargetStage = ""
		}
	}
}

func appendUniqueWarningChapter(items []warningChapterRef, chapter warningChapterRef) []warningChapterRef {
	for _, item := range items {
		if item.ID == chapter.ID {
			return items
		}
	}
	return append(items, chapter)
}
