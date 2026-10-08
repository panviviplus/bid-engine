package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	repollm "bid-engine/pkg/repo/llm"
)

func parseID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": name + " 参数无效"})
		return 0, false
	}
	return id, true
}

func (s *Service) CreateProject(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	llmCtx := repollm.WithUserID(c.Request.Context(), userID)
	if err := s.llm.LlmConfigAvailable(llmCtx, repollm.ModuleBidAnalysis); err != nil {
		code, msg := repollm.LLMErrorMeta(err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "llm_code": code, "msg": msg})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSourceBytes+(2<<20))
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请上传 PDF、DOC 或 DOCX 文件"})
		return
	}
	if file.Size > maxSourceBytes {
		c.JSON(400, gin.H{"code": 400, "msg": "文件超过 100MB 限制"})
		return
	}
	tmp, err := os.CreateTemp("", "bid-analysis-v3-upload-*"+strings.ToLower(filepath.Ext(file.Filename)))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "创建临时文件失败"})
		return
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		c.JSON(500, gin.H{"code": 500, "msg": "关闭临时上传文件失败"})
		return
	}
	defer os.Remove(tmpPath)
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "保存上传文件失败"})
		return
	}
	mime, err := validateUploadedDocument(tmpPath, file.Filename)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	sha, err := fileSHA256(tmpPath)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "计算文件摘要失败"})
		return
	}
	object := fmt.Sprintf("bid-analysis-v3/incoming/%d/%d-%s%s", userID, time.Now().UnixNano(), sha[:12], strings.ToLower(filepath.Ext(file.Filename)))
	if err := s.oss.Put(c, object, tmpPath); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "上传源文件失败"})
		return
	}
	cfg := repollm.ResolveConfig(llmCtx, llmFeatureFactExtract)
	contextWindow, maxOutput := 32768, 8192
	model := ""
	endpoint := ""
	if cfg != nil {
		contextWindow = cfg.ContextWindowTokens
		maxOutput = cfg.DefaultMaxTokens
		model = cfg.Model
		endpoint = cfg.EndpointPath
	}
	if contextWindow <= 0 {
		contextWindow = 32768
	}
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	// 以“模型无关预算”钳制后的有效值作为校验与落库依据：
	// 输出上限、上下文、输入分块均不再直接信任用户配置，小模型/异常配置不会被拒之门外。
	budget := resolveLLMBudget(contextWindow, maxOutput, llmOutputCapExtraction, defaultLLMInputChunkCeiling)
	contextWindow, maxOutput = budget.ContextWindow, budget.MaxOutput
	if contextWindow < llmMinContextWindow {
		s.deleteObjectBestEffort(c, object)
		c.JSON(400, gin.H{"code": 400, "msg": "模型上下文配置无效：有效上下文不足 8192 tokens，请更换模型或调整配置"})
		return
	}
	modelConfig, err := json.Marshal(map[string]any{"model": model, "endpoint_path": endpoint, "context_window_tokens": contextWindow, "max_output_tokens": maxOutput})
	if err != nil {
		s.deleteObjectBestEffort(c, object)
		c.JSON(500, gin.H{"code": 500, "msg": "序列化模型配置失败"})
		return
	}
	user := entity.GetUserFromCtx(c)
	companyID := int32(0)
	if user != nil {
		companyID = user.CompanyID
	}
	// 项目名称默认取文件名；招标情报站联动时允许前端传入更准确的公告标题
	projectName := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	if customName := strings.TrimSpace(c.PostForm("name")); customName != "" {
		projectName = truncateRunes(customName, 200)
	}
	project, run, err := s.repo.CreateProjectAndRun(c, repov3CreateInput(projectName, file.Filename, s.oss.GetDefaultBucketName(), object, sha, userID, companyID, mime, string(modelConfig)))
	if err != nil {
		s.deleteObjectBestEffort(c, object)
		c.JSON(500, gin.H{"code": 500, "msg": "创建项目失败: " + err.Error()})
		return
	}
	taskID, err := s.enqueue(c, project.ID, run.ID, userID, false, "document_preprocessing")
	if err != nil {
		if stateErr := s.repo.FailRun(c, project.ID, run.ID, "document_preprocessing", err); stateErr != nil {
			s.logger.Errorw("解析任务入队失败且运行状态保存失败", "project_id", project.ID, "run_id", run.ID, "enqueue_err", err, "state_err", stateErr)
			c.JSON(500, gin.H{"code": 500, "msg": "解析任务入队失败，且运行状态保存失败，请删除项目后重试"})
			return
		}
		c.JSON(500, gin.H{"code": 500, "msg": "解析任务入队失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": project.ID, "run_id": run.ID, "task_id": taskID}})
}

func repov3CreateInput(name, fileName, bucket, object, sha string, userID int64, companyID int32, mime, modelConfig string) repov3.CreateProjectInput {
	return repov3.CreateProjectInput{Name: name, SourceFileName: fileName, SourceBucket: bucket, SourceObject: object, SourceSHA256: sha, UserID: userID, CompanyID: companyID, ModelConfigJSON: modelConfig}
}

// truncateRunes 按字符截断项目名称，避免按字节切断中文。
func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (s *Service) ListProjects(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 12
	}
	items, total, err := s.repo.ListProjects(c, userID, page, size, c.Query("status"), strings.TrimSpace(c.Query("keyword")))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询项目失败"})
		return
	}
	ids := make([]int64, 0, len(items))
	for _, p := range items {
		ids = append(ids, p.ID)
	}
	metrics, err := s.projectMetrics(c, ids)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载项目指标失败"})
		return
	}
	runsByProject, err := s.repo.CurrentRunsByProjects(c, ids)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载项目运行状态失败"})
		return
	}
	runIDs := make([]int64, 0, len(runsByProject))
	for _, run := range runsByProject {
		runIDs = append(runIDs, run.ID)
	}
	controlsByRun, err := s.repo.LatestRunControls(c, runIDs)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载运行控制状态失败"})
		return
	}
	statusCounts, err := s.repo.ProjectStatusCounts(c, userID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载项目状态统计失败"})
		return
	}
	rows := make([]gin.H, 0, len(items))
	for _, p := range items {
		m := metrics[p.ID]
		run := runsByProject[p.ID]
		var control *model.BidAnalysisV3RunControl
		if run != nil {
			control = visibleControl(p.Status, controlsByRun[run.ID])
		}
		controlPending := control != nil && (control.Status == repov3.ControlRequested || control.Status == repov3.ControlApplying)
		canSkip := (p.Status == repov3.ProjectRunning || p.Status == repov3.ProjectFailed) && repov3.CanSkipStage(p.Stage) && !controlPending
		var factSubtask string
		var factSubtaskProgress gin.H
		if p.Status == repov3.ProjectRunning && p.Stage == "chapter_fact_extracting" && run != nil {
			statuses, statusErr := s.repo.FactSubtaskStatuses(c, run.ID)
			if statusErr != nil {
				c.JSON(500, gin.H{"code": 500, "msg": "加载事实提取子步骤进度失败"})
				return
			}
			factSubtask = currentFactSubtask(statuses)
			if factSubtask != "" {
				for _, status := range statuses {
					if status.SubTask == factSubtask {
						percent := int64(0)
						if status.Total > 0 {
							percent = status.Completed * 100 / status.Total
						}
						factSubtaskProgress = gin.H{"completed": status.Completed, "total": status.Total, "percent": percent}
						break
					}
				}
			}
		}
		rows = append(rows, gin.H{
			"id": p.ID, "run_id": func() int64 {
				if run != nil {
					return run.ID
				}
				return 0
			}(),
			"name": p.Name, "source_file_name": p.SourceFileName, "status": p.Status, "stage": p.Stage,
			"progress": p.Progress, "page_count": p.PageCount, "parsed_pages": runParsedPages(run),
			"field_count": m.FieldCount, "evidence_count": m.EvidenceCount, "table_count": m.TableCount,
			"warning_count": p.WarningCount, "last_error": userFacingStageError(p.Stage, p.LastError),
			"control": controlResponse(control), "can_pause": p.Status == repov3.ProjectRunning && !controlPending,
			"can_resume": p.Status == repov3.ProjectPaused && !controlPending, "can_skip_current": canSkip,
			"skip_blocked_reason": skipBlockedReason(p.Stage), "created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
			"fact_subtask": factSubtask, "fact_subtask_label": factSubtaskLabel(factSubtask), "fact_subtask_progress": factSubtaskProgress,
		})
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": rows, "total": total, "page": page, "page_size": size, "status_counts": statusCounts}})
}

// runParsedPages 返回“最新一次解析运行”已覆盖的页数。
//
// 列表页的“已覆盖 X / Y 页”是进度类指标，必须与阶段、进度条同源——三者都取项目最新一次运行。
// project.current_run_id 是“当前生效运行”，只在运行成功后切换；重新解析期间它仍指向上一次运行，
// 用它取页数会出现“进度 85%、已覆盖 0/159 页”这种自相矛盾的展示。
func runParsedPages(run *model.BidAnalysisV3ParseRun) int64 {
	if run == nil {
		return 0
	}
	return int64(run.ParsedPages)
}

type projectMetric struct {
	ProjectID     int64
	FieldCount    int64
	EvidenceCount int64
	TableCount    int64
}

// projectMetrics 汇总项目级“生效结果”指标：字段、证据、表格计数都跟随 current_run_id，
// 与详情页读取结果的口径一致；页数覆盖属于进度指标，由 runParsedPages 单独提供。
func (s *Service) projectMetrics(ctx context.Context, ids []int64) (map[int64]projectMetric, error) {
	out := map[int64]projectMetric{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []projectMetric
	if err := s.repo.DB().WithContext(ctx).Raw(`SELECT p.id project_id,(SELECT COUNT(*) FROM bid_analysis_v3_field f WHERE f.project_id=p.id AND f.extract_status IN ('found','ambiguous')) field_count,(SELECT COUNT(*) FROM bid_analysis_v3_field_value_evidence e JOIN bid_analysis_v3_field_value v ON v.id=e.field_value_id WHERE e.project_id=p.id AND (v.run_id=p.current_run_id OR v.origin='user') AND v.value_status IN ('active','suggestion')) evidence_count,(SELECT COUNT(*) FROM bid_analysis_v3_source_table t WHERE t.project_id=p.id AND t.run_id=p.current_run_id) table_count FROM bid_analysis_v3_project p WHERE p.id IN ?`, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ProjectID] = r
	}
	return out, nil
}

func (s *Service) GetProject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := s.repo.Project(c, id, entity.GetUserIDFromCtx(c))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	data, err := s.loadProjectDetail(c, p, entity.GetUserIDFromCtx(c))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载详情失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": data})
}

func (s *Service) loadProjectDetail(ctx context.Context, p *model.BidAnalysisV3Project, userID int64) (gin.H, error) {
	var categories []*model.BidAnalysisV3FieldCategory
	if err := s.repo.DB().WithContext(ctx).Order("sort_order").Find(&categories).Error; err != nil {
		return nil, err
	}
	var fields []*model.BidAnalysisV3Field
	if err := s.repo.DB().WithContext(ctx).Where("project_id=?", p.ID).Order("sort_order").Find(&fields).Error; err != nil {
		return nil, err
	}
	fieldIDs := make([]int64, 0, len(fields))
	for _, f := range fields {
		fieldIDs = append(fieldIDs, f.ID)
	}
	var values []*model.BidAnalysisV3FieldValue
	if len(fieldIDs) > 0 {
		if err := s.repo.DB().WithContext(ctx).Where("field_id IN ? AND (run_id=? OR origin='user')", fieldIDs, p.CurrentRunID).Order("field_id,value_status,id").Find(&values).Error; err != nil {
			return nil, err
		}
	}
	valueIDs := make([]int64, 0, len(values))
	for _, v := range values {
		valueIDs = append(valueIDs, v.ID)
	}
	var evidences []*model.BidAnalysisV3FieldValueEvidence
	if len(valueIDs) > 0 {
		if err := s.repo.DB().WithContext(ctx).Where("field_value_id IN ?", valueIDs).Order("field_value_id,sort_order").Find(&evidences).Error; err != nil {
			return nil, err
		}
	}
	evidenceByValue := map[int64][]*model.BidAnalysisV3FieldValueEvidence{}
	for _, e := range evidences {
		evidenceByValue[e.FieldValueID] = append(evidenceByValue[e.FieldValueID], e)
	}
	var derivedTables []*model.BidAnalysisV3DerivedTable
	if len(valueIDs) > 0 {
		if err := s.repo.DB().WithContext(ctx).Where("field_value_id IN ?", valueIDs).Order("field_value_id,id").Find(&derivedTables).Error; err != nil {
			return nil, err
		}
	}
	derivedIDs := make([]int64, 0, len(derivedTables))
	for _, table := range derivedTables {
		derivedIDs = append(derivedIDs, table.ID)
	}
	var derivedEvidences []*model.BidAnalysisV3DerivedTableEvidence
	if len(derivedIDs) > 0 {
		if err := s.repo.DB().WithContext(ctx).Where("derived_table_id IN ?", derivedIDs).Order("derived_table_id,sort_order").Find(&derivedEvidences).Error; err != nil {
			return nil, err
		}
	}
	derivedEvidenceMap := map[int64][]*model.BidAnalysisV3DerivedTableEvidence{}
	for _, evidence := range derivedEvidences {
		derivedEvidenceMap[evidence.DerivedTableID] = append(derivedEvidenceMap[evidence.DerivedTableID], evidence)
	}
	derivedByValue := map[int64][]gin.H{}
	for _, table := range derivedTables {
		derivedByValue[table.FieldValueID] = append(derivedByValue[table.FieldValueID], gin.H{"id": table.ID, "title": table.Title, "row_count": table.RowCount, "column_count": table.ColumnCount, "data": json.RawMessage(table.DataJSON), "source_count": len(derivedEvidenceMap[table.ID]), "evidences": derivedEvidenceMap[table.ID]})
	}
	valuesByField := map[int64][]gin.H{}
	for _, v := range values {
		valuesByField[v.FieldID] = append(valuesByField[v.FieldID], gin.H{"id": v.ID, "display_value": v.DisplayValue, "normalized_value": json.RawMessage(defaultJSON(v.NormalizedValueJSON)), "origin": v.Origin, "status": v.ValueStatus, "confidence": v.Confidence, "is_user_edited": v.IsUserEdited, "needs_evidence": v.NeedsEvidence, "evidence_count": len(evidenceByValue[v.ID]), "evidences": nonNilSlice(evidenceByValue[v.ID]), "derived_tables": nonNilSlice(derivedByValue[v.ID])})
	}
	fieldsByCategory := map[string][]gin.H{}
	for _, f := range fields {
		fieldsByCategory[f.CategoryKey] = append(fieldsByCategory[f.CategoryKey], buildFieldRow(f, nonNilSlice(valuesByField[f.ID])))
	}
	categoryRows := make([]gin.H, 0, len(categories))
	for _, category := range categories {
		categoryRows = append(categoryRows, gin.H{"key": category.CategoryKey, "name": category.DisplayName, "description": category.Description, "fields": nonNilSlice(fieldsByCategory[category.CategoryKey])})
	}
	chapters, err := s.repo.Chapters(ctx, p.CurrentRunID)
	if err != nil {
		return nil, err
	}
	var clauses []*model.BidAnalysisV3Clause
	if err := s.repo.DB().WithContext(ctx).Where("run_id=?", p.CurrentRunID).Order("chapter_id,sort_order").Find(&clauses).Error; err != nil {
		return nil, err
	}
	clauseIDs := make([]int64, 0, len(clauses))
	for _, cl := range clauses {
		clauseIDs = append(clauseIDs, cl.ID)
	}
	var clauseEvidences []*model.BidAnalysisV3ClauseEvidence
	if len(clauseIDs) > 0 {
		if err := s.repo.DB().WithContext(ctx).Where("clause_id IN ?", clauseIDs).Order("clause_id,sort_order").Find(&clauseEvidences).Error; err != nil {
			return nil, err
		}
	}
	ceMap := map[int64][]*model.BidAnalysisV3ClauseEvidence{}
	for _, e := range clauseEvidences {
		ceMap[e.ClauseID] = append(ceMap[e.ClauseID], e)
	}
	maxPage := int(p.PageCount)
	if maxPage <= 0 {
		for _, ch := range chapters {
			if int(ch.PageEnd) > maxPage {
				maxPage = int(ch.PageEnd)
			}
		}
	}
	// 关键条款按“顶级大章节”分组：只返回提炼到关键条款的顶级章节 group，
	// 避免把“1 适用范围”“一、总则”等小标题全部平铺成 group。
	topGroups := buildTopChapterGroups(chapters, clauses, maxPage)
	chapterRows := make([]gin.H, 0, len(topGroups))
	for _, group := range topGroups {
		clauseRows := make([]gin.H, 0, len(group.Clauses))
		for _, cl := range group.Clauses {
			clauseRows = append(clauseRows, buildClauseRow(cl, nonNilSlice(ceMap[cl.ID])))
		}
		chapterRows = append(chapterRows, gin.H{
			"id": group.Chapter.ID, "title": group.Chapter.ChapterTitle, "type": group.Chapter.ChapterType,
			"page_start": group.PageStart, "page_end": group.PageEnd, "boundary_source": group.Chapter.BoundarySource,
			"clauses": clauseRows,
		})
	}
	var sourceTables []*model.BidAnalysisV3SourceTable
	if p.CurrentRunID > 0 {
		if err := s.repo.DB().WithContext(ctx).
			Select("id, table_ref, caption, page_start, page_end, row_count, column_count, regions_json, bbox_left, bbox_top, bbox_width, bbox_height").
			Where("run_id=?", p.CurrentRunID).Order("page_start,id").Find(&sourceTables).Error; err != nil {
			return nil, err
		}
	}
	sourceTableRows := make([]gin.H, 0, len(sourceTables))
	for _, table := range sourceTables {
		sourceTableRows = append(sourceTableRows, gin.H{"id": table.ID, "table_ref": table.TableRef, "caption": table.Caption, "page_start": table.PageStart, "page_end": table.PageEnd, "row_count": table.RowCount, "column_count": table.ColumnCount, "regions": json.RawMessage(defaultJSON(table.RegionsJSON)), "bbox_left": table.BboxLeft, "bbox_top": table.BboxTop, "bbox_width": table.BboxWidth, "bbox_height": table.BboxHeight})
	}
	var summary model.BidAnalysisV3Summary
	if p.CurrentRunID > 0 {
		err := s.repo.DB().WithContext(ctx).Where("run_id=?", p.CurrentRunID).First(&summary).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	summaryData := any(nil)
	if summary.ID > 0 {
		var payload postprocessSummary
		if err := json.Unmarshal([]byte(defaultJSON(summary.SummaryJSON)), &payload); err != nil {
			return nil, err
		}
		resolvedMap := map[int32]bool{}
		var riskRows []*model.BidAnalysisV3SummaryRiskResolution
		if err := s.repo.DB().WithContext(ctx).Where("run_id=?", p.CurrentRunID).Find(&riskRows).Error; err != nil {
			return nil, err
		}
		for _, row := range riskRows {
			if row.Resolved {
				resolvedMap[row.RiskIndex] = true
			}
		}
		risks := make([]gin.H, 0, len(payload.Risks))
		for i, item := range payload.Risks {
			risks = append(risks, gin.H{"index": i, "text": item.Text, "kind": item.Kind, "label": item.Label, "resolved": resolvedMap[int32(i)]})
		}
		summaryData = gin.H{"id": summary.ID, "summary": gin.H{"overview": payload.Overview, "key_points": payload.KeyPoints, "risks": risks}}
	}
	var warningRows []warningGroupRow
	if p.CurrentRunID > 0 {
		warnings, err := s.repo.Warnings(ctx, p.CurrentRunID)
		if err != nil {
			return nil, err
		}
		warningRows = aggregateWarnings(warnings)
		stageRuns, err := s.repo.StageRuns(ctx, p.CurrentRunID)
		if err != nil {
			return nil, err
		}
		pruneRetryTargets(warningRows, p.Status, stageRuns)
	}
	var followRows []*model.BidAnalysisV3Follow
	if err := s.repo.DB().WithContext(ctx).Where("project_id=? AND user_id=?", p.ID, userID).Order("id").Find(&followRows).Error; err != nil {
		return nil, err
	}
	follows := make([]gin.H, 0, len(followRows))
	for _, f := range followRows {
		follows = append(follows, gin.H{
			"id": f.ID, "target_type": f.TargetType, "target_id": f.TargetID,
			"title": f.Title, "content": f.Content, "remark": f.Remark,
		})
	}
	return gin.H{"project": p, "categories": categoryRows, "chapters": chapterRows, "source_tables": sourceTableRows, "summary": summaryData, "warning_groups": warningRows, "follows": follows}, nil
}
func defaultJSON(v string) string {
	if strings.TrimSpace(v) == "" || !json.Valid([]byte(v)) {
		return "null"
	}
	return v
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return make([]T, 0)
	}
	return items
}

func compactStrings(items []string) []string {
	out := items[:0]
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func compactPositive(items []int64) []int64 {
	out := items[:0]
	for _, n := range items {
		if n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// buildFieldRow 组装字段详情行，供列表与详情接口复用。
func buildFieldRow(f *model.BidAnalysisV3Field, values []gin.H) gin.H {
	return gin.H{"id": f.ID, "field_key": f.FieldKey, "display_name": f.DisplayName, "origin": f.Origin, "value_type": f.ValueType, "extract_status": f.ExtractStatus, "ai_interpretation": f.AiInterpretation, "values": nonNilSlice(values)}
}

// buildClauseRow 组装条款详情行，供列表与详情接口复用。
func buildClauseRow(cl *model.BidAnalysisV3Clause, evidences []*model.BidAnalysisV3ClauseEvidence) gin.H {
	return gin.H{"id": cl.ID, "title": cl.Title, "content": cl.Content, "ai_interpretation": cl.AiInterpretation, "importance": cl.Importance, "status": cl.Status, "evidences": nonNilSlice(evidences)}
}

func (s *Service) GetProgress(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := s.repo.Project(c, id, entity.GetUserIDFromCtx(c))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	run, err := s.repo.CurrentRun(c, id)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "解析运行不存在"})
		return
	}
	stages, err := s.repo.StageRuns(c, run.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载阶段进度失败"})
		return
	}
	tasks, err := s.repo.StageTasks(c, run.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载子任务进度失败"})
		return
	}
	warnings, err := s.repo.Warnings(c, run.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载告警失败"})
		return
	}
	chunks, err := s.repo.Chunks(c, run.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载页块进度失败"})
		return
	}
	latestControl, controlErr := s.repo.LatestRunControls(c, []int64{run.ID})
	if controlErr != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载运行控制状态失败"})
		return
	}
	control := visibleControl(p.Status, latestControl[run.ID])
	controlPending := control != nil && (control.Status == repov3.ControlRequested || control.Status == repov3.ControlApplying)
	nextStage, _ := repov3.NextStage(run.Stage)
	factStatuses, err := s.repo.FactSubtaskStatuses(c, run.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载事实提取子步骤进度失败"})
		return
	}
	factSubtasks := make([]gin.H, 0, len(factStatuses))
	for _, status := range factStatuses {
		percent := int64(0)
		if status.Total > 0 {
			percent = status.Completed * 100 / status.Total
		}
		factSubtasks = append(factSubtasks, gin.H{
			"subtask": status.SubTask, "label": factSubtaskLabel(status.SubTask),
			"completed": status.Completed, "total": status.Total, "percent": percent,
		})
	}
	recovery := gin.H{
		"available":     p.Status == repov3.ProjectFailed && run.Status == repov3.ProjectFailed,
		"current_stage": run.Stage, "current_stage_label": stageDisplayName(run.Stage), "next_stage": nextStage,
		"can_retry_current": p.Status == repov3.ProjectFailed, "can_retry_global": p.Status != repov3.ProjectRunning && p.Status != repov3.ProjectPaused,
		"can_skip_current": (p.Status == repov3.ProjectFailed || p.Status == repov3.ProjectRunning) && repov3.CanSkipStage(run.Stage) && !controlPending,
		"last_error":       userFacingStageError(run.Stage, run.LastError), "skip_blocked_reason": skipBlockedReason(run.Stage),
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{
		"project": p, "run": run, "stages": stages, "tasks": tasks, "chunks": chunks, "warnings": warnings,
		"recovery": recovery, "control": controlResponse(control), "fact_subtasks": factSubtasks,
		"can_pause":           p.Status == repov3.ProjectRunning && !controlPending,
		"can_resume":          p.Status == repov3.ProjectPaused && !controlPending,
		"can_skip_current":    (p.Status == repov3.ProjectRunning || p.Status == repov3.ProjectFailed) && repov3.CanSkipStage(run.Stage) && !controlPending,
		"skip_blocked_reason": skipBlockedReason(run.Stage),
	}})
}

func (s *Service) Reparse(c *gin.Context) { s.createNewRun(c, "reparse") }

type retryRunReq struct {
	Mode string `json:"mode"`
}

func (s *Service) Retry(c *gin.Context) {
	var req retryRunReq
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(400, gin.H{"code": 400, "msg": "重试请求格式无效"})
		return
	}
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Mode == "global" {
		s.createNewRun(c, "retry_global")
		return
	}
	if req.Mode != "" && req.Mode != "current_stage" {
		c.JSON(400, gin.H{"code": 400, "msg": "mode 仅支持 current_stage 或 global"})
		return
	}
	s.retryCurrentStage(c)
}

type retryStageReq struct {
	RunID int64  `json:"run_id"`
	Stage string `json:"stage"`
}

// RetryStage 支持从“失败”或“完成但有告警”的项目按目标阶段级重试（复用已解析页块/章节）。
func (s *Service) RetryStage(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req retryStageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "重试请求格式无效"})
		return
	}
	req.Stage = strings.TrimSpace(req.Stage)
	if req.RunID <= 0 || req.Stage == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "请提供 run_id 与目标阶段"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.Status != repov3.ProjectFailed && p.Status != repov3.ProjectSucceededWithWarnings && p.Status != repov3.ProjectSucceeded {
		c.JSON(409, gin.H{"code": 409, "msg": "仅失败、完成或有告警的项目可以阶段级重试"})
		return
	}
	run, err := s.repo.CurrentRun(c, id)
	if err != nil || run.ID != req.RunID {
		c.JSON(409, gin.H{"code": 409, "msg": "解析运行已不是当前运行，请刷新后重试"})
		return
	}
	if err := s.repo.PrepareStageRetry(c, id, run.ID, uid, req.Stage); err != nil {
		c.JSON(409, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	taskID, err := s.enqueue(c, id, run.ID, uid, true, req.Stage)
	if err != nil {
		if stateErr := s.repo.FailRun(c, id, run.ID, req.Stage, err); stateErr != nil {
			s.logger.Errorw("阶段级重试入队失败且状态回滚失败", "project_id", id, "run_id", run.ID, "stage", req.Stage, "enqueue_err", err, "state_err", stateErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "阶段级重试入队失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": run.ID, "task_id": taskID, "start_stage": req.Stage}})
}

func (s *Service) retryCurrentStage(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.Status != repov3.ProjectFailed {
		c.JSON(409, gin.H{"code": 409, "msg": "仅失败项目可以从当前阶段重试"})
		return
	}
	run, err := s.repo.CurrentRun(c, id)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "解析运行不存在"})
		return
	}
	stage := run.Stage
	if err := s.repo.PrepareStageRetry(c, id, run.ID, uid, stage); err != nil {
		c.JSON(409, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	taskID, err := s.enqueue(c, id, run.ID, uid, true, stage)
	if err != nil {
		if stateErr := s.repo.FailRun(c, id, run.ID, stage, err); stateErr != nil {
			s.logger.Errorw("当前阶段重试入队失败且状态回滚失败", "project_id", id, "run_id", run.ID, "enqueue_err", err, "state_err", stateErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "当前阶段重试入队失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": run.ID, "task_id": taskID, "start_stage": stage}})
}

type resolveWarningsReq struct {
	IDs       []int64  `json:"ids"`
	GroupKeys []string `json:"group_keys"`
	Severity  string   `json:"severity"`
}

// ResolveWarnings 支持按告警 id、分组键或级别批量标记已解决；
// 已解决的非提示级告警会同步扣减项目/运行告警计数，提示级（info）不计入计数。
func (s *Service) ResolveWarnings(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req resolveWarningsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请求格式无效"})
		return
	}
	req.GroupKeys = compactStrings(req.GroupKeys)
	req.Severity = strings.TrimSpace(req.Severity)
	req.IDs = compactPositive(req.IDs)
	if len(req.IDs) == 0 && len(req.GroupKeys) == 0 && req.Severity == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "请提供告警 id、分组键或级别"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	run, err := s.repo.CurrentRun(c, id)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "解析运行不存在"})
		return
	}
	resolved, warningCount, err := s.repo.ResolveWarnings(c, p.ID, run.ID, req.IDs, req.GroupKeys, req.Severity)
	if err != nil {
		s.logger.Errorw("标记告警已解决失败", "project_id", id, "run_id", run.ID, "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "标记告警已解决失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"resolved": resolved, "warning_count": warningCount}})
}

func (s *Service) SkipCurrentStage(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req controlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请提供当前运行和阶段信息"})
		return
	}
	req.ExpectedStage = strings.TrimSpace(req.ExpectedStage)
	if req.ExpectedRunID <= 0 || req.ExpectedStage == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "expected_run_id 和 expected_stage 不能为空"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	run, err := s.repo.CurrentRun(c, id)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "解析运行不存在"})
		return
	}
	if p.Status == repov3.ProjectRunning {
		control, err := s.repo.RequestRunControl(c, repov3.RequestRunControlInput{
			ProjectID: id, RunID: req.ExpectedRunID, OperatorID: uid, Action: repov3.ControlActionSkip,
			Mode: repov3.ControlModeDiscardCurrent, ExpectedStage: req.ExpectedStage,
		})
		if err != nil {
			writeControlRequestError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"code": http.StatusAccepted, "data": gin.H{"control": controlResponse(control)}})
		return
	}
	if p.Status != repov3.ProjectFailed {
		c.JSON(409, gin.H{"code": 409, "msg": "当前项目状态不允许跳过阶段"})
		return
	}
	failedStage := run.Stage
	if req.ExpectedRunID != run.ID || req.ExpectedStage != failedStage {
		c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": repov3.ErrControlConflict.Error()})
		return
	}
	nextStage, err := s.repo.SkipFailedStage(c, id, run.ID, uid, failedStage)
	if err != nil {
		c.JSON(409, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	if nextStage == repov3.PipelineCompleteStage {
		if err := s.repo.CompleteRun(c, id, run.ID); err != nil {
			if stateErr := s.repo.FailRun(c, id, run.ID, failedStage, err); stateErr != nil {
				s.logger.Errorw("跳过末尾阶段后完成运行失败且状态回滚失败", "project_id", id, "run_id", run.ID, "complete_err", err, "state_err", stateErr)
			}
			c.JSON(500, gin.H{"code": 500, "msg": "跳过末尾阶段后完成运行失败"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": run.ID, "skipped_stage": failedStage, "completed": true}})
		return
	}
	taskID, err := s.enqueue(c, id, run.ID, uid, true, nextStage)
	if err != nil {
		if stateErr := s.repo.FailRun(c, id, run.ID, nextStage, err); stateErr != nil {
			s.logger.Errorw("跳过阶段后入队失败且状态回滚失败", "project_id", id, "run_id", run.ID, "enqueue_err", err, "state_err", stateErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "跳过阶段后的任务入队失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": run.ID, "task_id": taskID, "skipped_stage": failedStage, "start_stage": nextStage}})
}

type controlRequest struct {
	Mode          string `json:"mode"`
	ExpectedRunID int64  `json:"expected_run_id"`
	ExpectedStage string `json:"expected_stage"`
}

func (s *Service) Pause(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req controlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "暂停请求格式无效"})
		return
	}
	req.Mode = strings.TrimSpace(req.Mode)
	req.ExpectedStage = strings.TrimSpace(req.ExpectedStage)
	if req.Mode != repov3.ControlModeAfterStage && req.Mode != repov3.ControlModeDiscardCurrent {
		c.JSON(400, gin.H{"code": 400, "msg": "请选择暂停方式"})
		return
	}
	if req.ExpectedRunID <= 0 || req.ExpectedStage == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "expected_run_id 和 expected_stage 不能为空"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.Status != repov3.ProjectRunning {
		c.JSON(409, gin.H{"code": 409, "msg": "仅解析中的项目可以暂停"})
		return
	}
	control, err := s.repo.RequestRunControl(c, repov3.RequestRunControlInput{
		ProjectID: id, RunID: req.ExpectedRunID, OperatorID: uid, Action: repov3.ControlActionPause,
		Mode: req.Mode, ExpectedStage: req.ExpectedStage,
	})
	if err != nil {
		writeControlRequestError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": http.StatusAccepted, "data": gin.H{"control": controlResponse(control)}})
}

func (s *Service) Resume(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req controlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "继续解析请求格式无效"})
		return
	}
	req.ExpectedStage = strings.TrimSpace(req.ExpectedStage)
	if req.ExpectedRunID <= 0 || req.ExpectedStage == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "expected_run_id 和 expected_stage 不能为空"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.Status != repov3.ProjectPaused {
		c.JSON(409, gin.H{"code": 409, "msg": "仅已暂停项目可以继续解析"})
		return
	}
	if err := s.repo.ResumePausedRun(c, id, req.ExpectedRunID, uid, req.ExpectedStage); err != nil {
		writeControlRequestError(c, err)
		return
	}
	taskID, err := s.enqueue(c, id, req.ExpectedRunID, uid, true, req.ExpectedStage)
	if err != nil {
		if restoreErr := s.repo.RestorePausedRun(context.WithoutCancel(c), id, req.ExpectedRunID, uid, req.ExpectedStage, err); restoreErr != nil {
			s.logger.Errorw("继续解析入队失败且暂停状态恢复失败", "project_id", id, "run_id", req.ExpectedRunID, "enqueue_err", err, "restore_err", restoreErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "继续解析任务入队失败，项目仍保持暂停"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": req.ExpectedRunID, "task_id": taskID, "start_stage": req.ExpectedStage}})
}

func visibleControl(projectStatus string, control *model.BidAnalysisV3RunControl) *model.BidAnalysisV3RunControl {
	if control == nil {
		return nil
	}
	if control.Status == repov3.ControlRequested || control.Status == repov3.ControlApplying {
		return control
	}
	if projectStatus == repov3.ProjectPaused && control.Action == repov3.ControlActionPause && control.Status == repov3.ControlApplied {
		return control
	}
	// 近期失败/取消的控制也透出，供前端提示“暂停/跳过未生效”，避免无声消失。
	if (control.Status == repov3.ControlFailed || control.Status == repov3.ControlCancelled) &&
		time.Since(control.UpdatedAt) < 24*time.Hour {
		return control
	}
	return nil
}

func controlResponse(control *model.BidAnalysisV3RunControl) any {
	if control == nil {
		return nil
	}
	return gin.H{
		"id": control.ID, "project_id": control.ProjectID, "run_id": control.RunID,
		"action": control.Action, "mode": control.Mode, "target_stage": control.TargetStage,
		"resume_stage": control.ResumeStage, "status": control.Status, "last_error": control.LastError,
		"requested_at": control.RequestedAt, "applied_at": control.AppliedAt,
	}
}

func skipBlockedReason(stage string) string {
	if repov3.CanSkipStage(stage) {
		return ""
	}
	return "基础阶段不可跳过"
}

func writeControlRequestError(c *gin.Context, err error) {
	if errors.Is(err, repov3.ErrControlConflict) || errors.Is(err, repov3.ErrControlRejected) {
		c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": "操作未完成，请稍后重试"})
}

func stageDisplayName(stage string) string {
	labels := map[string]string{
		"document_preprocessing": "文档预处理", "document_parsing": "全文解析", "document_summary": "文档摘要",
		"chapter_identifying": "章节识别", "chapter_fact_extracting": "事实提取", "content_consolidating": "内容归并",
	}
	return labels[stage]
}

func currentFactSubtask(statuses []repov3.FactSubtaskStatus) string {
	for _, status := range statuses {
		if status.Total > 0 && status.Completed < status.Total {
			return status.SubTask
		}
	}
	for _, status := range statuses {
		if status.Total == 0 {
			return status.SubTask
		}
	}
	return ""
}

func factSubtaskLabel(subtask string) string {
	labels := map[string]string{
		repov3.FactSubtaskDynamicClauses: "事实提取-动态字段和条款",
		repov3.FactSubtaskFixedFields:    "事实提取-固定字段",
		repov3.FactSubtaskEvidence:       "事实提取-证据核验",
		repov3.FactSubtaskAIInterpret:    "事实提取-AI解读",
	}
	return labels[subtask]
}

func userFacingStageError(stage, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "该阶段未能完成，请选择重试方式。"
	}
	messages := map[string]string{
		"document_preprocessing":  "文件预处理未完成，请确认文件可以正常打开后重试。",
		"document_parsing":        "招标文件中有页面未能完整解析，请从当前阶段重试或重新解析全文。",
		"document_summary":        "文档摘要未能生成，可重试或跳过并保留告警。",
		"chapter_identifying":     "未能建立连续、可信的章节结构，请从当前阶段重试或重新解析全文。",
		"chapter_fact_extracting": "部分章节的字段与条款提取未能完成，可重试、跳过并保留告警，或重新解析全文。",
		"content_consolidating":   "字段与条款的归并未能完成，可重试、跳过并保留告警，或重新解析全文。",
	}
	if message := messages[stage]; message != "" {
		return message
	}
	return "解析未能完成，请选择合适的方式重试。"
}

func (s *Service) createNewRun(c *gin.Context, trigger string) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, id, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.Status == repov3.ProjectRunning || p.Status == repov3.ProjectPaused {
		c.JSON(409, gin.H{"code": 409, "msg": "项目已有尚未结束的解析运行，请先继续完成或删除项目"})
		return
	}
	llmCtx := repollm.WithUserID(c.Request.Context(), uid)
	cfg := repollm.ResolveConfig(llmCtx, llmFeatureFactExtract)
	if cfg == nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请先配置招标解析模型"})
		return
	}
	contextWindow, maxOutput := cfg.ContextWindowTokens, cfg.DefaultMaxTokens
	if contextWindow <= 0 {
		contextWindow = 32768
	}
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	budget := resolveLLMBudget(contextWindow, maxOutput, llmOutputCapExtraction, defaultLLMInputChunkCeiling)
	contextWindow, maxOutput = budget.ContextWindow, budget.MaxOutput
	if contextWindow < llmMinContextWindow {
		c.JSON(400, gin.H{"code": 400, "msg": "模型上下文配置无效：有效上下文不足 8192 tokens，请更换模型或调整配置"})
		return
	}
	mc, err := json.Marshal(map[string]any{"model": cfg.Model, "endpoint_path": cfg.EndpointPath, "context_window_tokens": contextWindow, "max_output_tokens": maxOutput})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "序列化模型配置失败"})
		return
	}
	run, err := s.repo.CreateRun(c, id, uid, trigger, string(mc))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "创建解析运行失败"})
		return
	}
	taskID, err := s.enqueue(c, id, run.ID, uid, true, "document_preprocessing")
	if err != nil {
		if stateErr := s.repo.FailRun(c, id, run.ID, "document_preprocessing", err); stateErr != nil {
			s.logger.Errorw("重试任务入队失败且运行状态保存失败", "project_id", id, "run_id", run.ID, "enqueue_err", err, "state_err", stateErr)
			c.JSON(500, gin.H{"code": 500, "msg": "重试任务入队失败，且运行状态保存失败"})
			return
		}
		c.JSON(500, gin.H{"code": 500, "msg": "重试任务入队失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"run_id": run.ID, "task_id": taskID}})
}

type updateValueReq struct {
	DisplayValue    string `json:"display_value" binding:"required"`
	NormalizedValue any    `json:"normalized_value"`
}

func (s *Service) UpdateFieldValue(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req updateValueReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DisplayValue) == "" || len([]rune(req.DisplayValue)) > 20000 {
		c.JSON(400, gin.H{"code": 400, "msg": "字段值不能为空"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	var original model.BidAnalysisV3FieldValue
	err := s.repo.DB().WithContext(c).Table("bid_analysis_v3_field_value v").Select("v.*").Joins("JOIN bid_analysis_v3_project p ON p.id=v.project_id").Where("v.id=? AND p.user_id=?", id, uid).Scan(&original).Error
	if err != nil || original.ID == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "字段值不存在"})
		return
	}
	project, err := s.repo.Project(c, original.ProjectID, uid)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	norm, _ := json.Marshal(req.NormalizedValue)
	var created model.BidAnalysisV3FieldValue
	err = s.repo.DB().WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.BidAnalysisV3FieldValue{}).Where("field_id=? AND origin='user' AND value_status='active'", original.FieldID).Update("value_status", "historical").Error; err != nil {
			return err
		}
		if original.Origin == "ai" {
			if err := tx.Model(&original).Update("value_status", "suggestion").Error; err != nil {
				return err
			}
		}
		created = model.BidAnalysisV3FieldValue{ProjectID: original.ProjectID, FieldID: original.FieldID, RunID: project.CurrentRunID, Origin: "user", ValueStatus: "active", DisplayValue: strings.TrimSpace(req.DisplayValue), NormalizedValueJSON: string(norm), Confidence: "high", IsUserEdited: true, NeedsEvidence: true, CreatedBy: uid}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		return tx.Model(&model.BidAnalysisV3Field{}).Where("id=?", original.FieldID).Updates(map[string]any{"current_value_id": created.ID, "extract_status": "found"}).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "保存字段失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": created})
}

type resolveSummaryRiskReq struct {
	Resolved bool `json:"resolved"`
}

func (s *Service) ResolveSummaryRisk(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	riskIndex, err := strconv.Atoi(c.Param("index"))
	if err != nil || riskIndex < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "风险项序号无效"})
		return
	}
	var req resolveSummaryRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请求参数无效"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, projectID, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if p.CurrentRunID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "项目尚无解析结果"})
		return
	}
	var summary model.BidAnalysisV3Summary
	if err := s.repo.DB().WithContext(c).Where("run_id=?", p.CurrentRunID).First(&summary).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "摘要不存在"})
		return
	}
	var payload postprocessSummary
	if err := json.Unmarshal([]byte(defaultJSON(summary.SummaryJSON)), &payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "摘要数据解析失败"})
		return
	}
	if riskIndex >= len(payload.Risks) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "风险项序号超出范围"})
		return
	}
	var resolvedAt *time.Time
	var resolvedBy *int64
	if req.Resolved {
		now := time.Now()
		resolvedAt = &now
		resolvedBy = &uid
	}
	row := &model.BidAnalysisV3SummaryRiskResolution{
		ProjectID:  projectID,
		RunID:      p.CurrentRunID,
		RiskIndex:  int32(riskIndex),
		Resolved:   req.Resolved,
		ResolvedAt: resolvedAt,
		ResolvedBy: resolvedBy,
	}
	if err := s.repo.DB().WithContext(c).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "run_id"}, {Name: "risk_index"}},
		DoUpdates: clause.AssignmentColumns([]string{"resolved", "resolved_at", "resolved_by", "updated_at"}),
	}).Create(row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "保存风险状态失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"resolved": req.Resolved}})
}

type addFollowReq struct {
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Remark     string `json:"remark"`
}

func (s *Service) AddFollow(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req addFollowReq
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.TargetType != "field" && req.TargetType != "clause") || req.TargetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "关注对象参数无效"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	p, err := s.repo.Project(c, projectID, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	var title, content string
	if req.TargetType == "field" {
		var field model.BidAnalysisV3Field
		if err := s.repo.DB().WithContext(c).Where("id=? AND project_id=?", req.TargetID, projectID).First(&field).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "字段不存在"})
			return
		}
		title = field.DisplayName
		if field.CurrentValueID > 0 {
			var value model.BidAnalysisV3FieldValue
			if err := s.repo.DB().WithContext(c).Where("id=? AND field_id=?", field.CurrentValueID, field.ID).First(&value).Error; err == nil {
				content = value.DisplayValue
			}
		}
		if content == "" {
			var value model.BidAnalysisV3FieldValue
			if err := s.repo.DB().WithContext(c).Where("field_id=? AND value_status='active'", field.ID).Order("id").First(&value).Error; err == nil {
				content = value.DisplayValue
			}
		}
	} else {
		var clause model.BidAnalysisV3Clause
		if err := s.repo.DB().WithContext(c).Where("id=? AND project_id=? AND run_id=?", req.TargetID, projectID, p.CurrentRunID).First(&clause).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "条款不存在"})
			return
		}
		title = clause.Title
		content = clause.Content
	}
	follow := &model.BidAnalysisV3Follow{
		ProjectID:  projectID,
		UserID:     uid,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Title:      title,
		Content:    content,
		Remark:     strings.TrimSpace(req.Remark),
	}
	if err := s.repo.DB().WithContext(c).Create(follow).Error; err != nil {
		if strings.Contains(err.Error(), "1062") || strings.Contains(err.Error(), "Duplicate entry") {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该关注项已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "添加关注失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"id": follow.ID, "target_type": follow.TargetType, "target_id": follow.TargetID,
		"title": follow.Title, "content": follow.Content, "remark": follow.Remark,
	}})
}

func (s *Service) RemoveFollow(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	followID, ok := parseID(c, "followId")
	if !ok {
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	res := s.repo.DB().WithContext(c).
		Where("id=? AND project_id=? AND user_id=?", followID, projectID, uid).
		Delete(&model.BidAnalysisV3Follow{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "取消关注失败"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "关注项不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"deleted": true}})
}

type addEvidenceReq struct {
	Evidences []struct {
		SourceKind string  `json:"source_kind"`
		BlockID    int64   `json:"block_id"`
		TableID    int64   `json:"table_id"`
		PageNo     int32   `json:"page_no"`
		Quote      string  `json:"quote"`
		Left       float64 `json:"left"`
		Top        float64 `json:"top"`
		Width      float64 `json:"width"`
		Height     float64 `json:"height"`
	} `json:"evidences" binding:"required"`
}

func (s *Service) AddEvidences(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req addEvidenceReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Evidences) == 0 || len(req.Evidences) > 50 {
		c.JSON(400, gin.H{"code": 400, "msg": "请至少选择一处证据"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	var value model.BidAnalysisV3FieldValue
	if err := s.repo.DB().WithContext(c).Table("bid_analysis_v3_field_value v").Select("v.*").Joins("JOIN bid_analysis_v3_project p ON p.id=v.project_id").Where("v.id=? AND p.user_id=?", id, uid).Scan(&value).Error; err != nil || value.ID == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "字段值不存在"})
		return
	}
	project, err := s.repo.Project(c, value.ProjectID, uid)
	if err != nil || project.CurrentRunID <= 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "项目当前解析结果不存在"})
		return
	}
	err = s.repo.DB().WithContext(c).Transaction(func(tx *gorm.DB) error {
		var maxSort int32
		if err := tx.Model(&model.BidAnalysisV3FieldValueEvidence{}).Where("field_value_id=?", value.ID).Select("COALESCE(MAX(sort_order),-1)").Scan(&maxSort).Error; err != nil {
			return err
		}
		requestEvidence := make(map[string]struct{}, len(req.Evidences))
		nextSort := maxSort + 1
		for _, item := range req.Evidences {
			e := &model.BidAnalysisV3FieldValueEvidence{ProjectID: value.ProjectID, RunID: project.CurrentRunID, FieldValueID: value.ID, SourceKind: "manual_text", BlockID: item.BlockID, SourceTableID: item.TableID, PageNo: item.PageNo, Quote: strings.TrimSpace(item.Quote), BboxLeft: item.Left, BboxTop: item.Top, BboxWidth: item.Width, BboxHeight: item.Height, CreatedBy: uid}
			var regions []sourceRegion
			if item.BlockID > 0 {
				var b model.BidAnalysisV3DocumentBlock
				if err := tx.Where("id=? AND project_id=? AND run_id=?", item.BlockID, value.ProjectID, project.CurrentRunID).First(&b).Error; err != nil {
					return fmt.Errorf("文本块无效")
				}
				e.SourceKind = "manual_text"
				e.PageNo = b.PageNo
				e.BboxLeft = b.BboxLeft
				e.BboxTop = b.BboxTop
				e.BboxWidth = b.BboxWidth
				e.BboxHeight = b.BboxHeight
				if e.Quote == "" {
					e.Quote = b.Text
				}
			} else if item.TableID > 0 {
				var t model.BidAnalysisV3SourceTable
				if err := tx.Where("id=? AND project_id=? AND run_id=?", item.TableID, value.ProjectID, project.CurrentRunID).First(&t).Error; err != nil {
					return fmt.Errorf("表格无效")
				}
				e.SourceKind = "manual_table"
				e.PageNo = t.PageStart
				e.BboxLeft = t.BboxLeft
				e.BboxTop = t.BboxTop
				e.BboxWidth = t.BboxWidth
				e.BboxHeight = t.BboxHeight
				regions = sourceTableRegions(&t)
				if e.Quote == "" {
					e.Quote = t.Caption
				}
			} else {
				if item.PageNo <= 0 || item.PageNo > project.PageCount || e.Quote == "" || !validEvidenceBBox(item.Left, item.Top, item.Width, item.Height) {
					return fmt.Errorf("手工文本证据的位置或内容无效")
				}
			}
			if len([]rune(e.Quote)) > 10000 {
				return fmt.Errorf("证据原文超过 10000 字")
			}
			e.ContentHash = hashText(e.Quote)
			identity := fmt.Sprintf("raw:%d:%.8f:%.8f:%.8f:%.8f:%s", e.PageNo, e.BboxLeft, e.BboxTop, e.BboxWidth, e.BboxHeight, e.ContentHash)
			duplicateQuery := tx.Model(&model.BidAnalysisV3FieldValueEvidence{}).Where("field_value_id=?", value.ID)
			if e.SourceTableID > 0 {
				identity = fmt.Sprintf("table:%d", e.SourceTableID)
				duplicateQuery = duplicateQuery.Where("source_table_id=?", e.SourceTableID)
			} else if e.BlockID > 0 {
				identity = fmt.Sprintf("block:%d", e.BlockID)
				duplicateQuery = duplicateQuery.Where("block_id=?", e.BlockID)
			} else {
				duplicateQuery = duplicateQuery.Where("page_no=? AND bbox_left=? AND bbox_top=? AND bbox_width=? AND bbox_height=? AND content_hash=?", e.PageNo, e.BboxLeft, e.BboxTop, e.BboxWidth, e.BboxHeight, e.ContentHash)
			}
			if _, exists := requestEvidence[identity]; exists {
				return fmt.Errorf("同一证据不能重复绑定")
			}
			requestEvidence[identity] = struct{}{}
			var duplicate int64
			if err := duplicateQuery.Count(&duplicate).Error; err != nil {
				return err
			}
			if duplicate > 0 {
				return fmt.Errorf("该证据已经绑定")
			}
			if len(regions) == 0 {
				e.SortOrder = nextSort
				nextSort++
				if err := tx.Create(e).Error; err != nil {
					return err
				}
				continue
			}
			for _, region := range regions {
				regionEvidence := *e
				regionEvidence.PageNo = region.PageNo
				regionEvidence.BboxLeft = region.Left
				regionEvidence.BboxTop = region.Top
				regionEvidence.BboxWidth = region.Width
				regionEvidence.BboxHeight = region.Height
				regionEvidence.SortOrder = nextSort
				nextSort++
				if err := tx.Create(&regionEvidence).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.BidAnalysisV3FieldValue{}).Where("id=?", value.ID).Update("needs_evidence", false).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "绑定证据失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "证据已绑定"})
}

func validEvidenceBBox(left, top, width, height float64) bool {
	return left >= 0 && top >= 0 && width > 0 && height > 0 && left <= 1 && top <= 1 && width <= 1 && height <= 1 && left+width <= 1.000001 && top+height <= 1.000001
}

func (s *Service) GetEvidences(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	var count int64
	if err := s.repo.DB().WithContext(c).Table("bid_analysis_v3_field_value v").Joins("JOIN bid_analysis_v3_project p ON p.id=v.project_id").Where("v.id=? AND p.user_id=?", id, uid).Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "校验证据权限失败"})
		return
	}
	if count == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "字段值不存在"})
		return
	}
	var items []*model.BidAnalysisV3FieldValueEvidence
	if err := s.repo.DB().WithContext(c).Where("field_value_id=?", id).Order("sort_order,id").Find(&items).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载证据失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": items})
}

func (s *Service) GetSourcePDF(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := s.repo.Project(c, id, entity.GetUserIDFromCtx(c))
	if err != nil || p.NormalizedPdfObject == "" {
		c.JSON(404, gin.H{"code": 404, "msg": "规范化 PDF 尚未生成"})
		return
	}
	stream, err := s.oss.Open(c, p.NormalizedPdfObject)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "读取 PDF 失败"})
		return
	}
	defer stream.Close()
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=project-%d.pdf", p.ID))
	if _, err := io.Copy(c.Writer, stream); err != nil {
		s.logger.Warnw("向客户端输出规范化 PDF 失败", "project_id", p.ID, "err", err)
	}
}

func (s *Service) GetSourceTable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	var table model.BidAnalysisV3SourceTable
	err := s.repo.DB().WithContext(c).Table("bid_analysis_v3_source_table t").Select("t.*").Joins("JOIN bid_analysis_v3_project p ON p.id=t.project_id").Where("t.id=? AND p.user_id=?", id, uid).Scan(&table).Error
	if err != nil || table.ID == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "表格不存在"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": table.ID, "table_ref": table.TableRef, "caption": table.Caption, "page_start": table.PageStart, "page_end": table.PageEnd, "data": json.RawMessage(table.DataJSON), "regions": json.RawMessage(defaultJSON(table.RegionsJSON)), "bbox": gin.H{"left": table.BboxLeft, "top": table.BboxTop, "width": table.BboxWidth, "height": table.BboxHeight}}})
}

type deleteReq struct {
	Reason string `json:"reason"`
}

func (s *Service) DeleteProject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req deleteReq
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(400, gin.H{"code": 400, "msg": "删除请求格式无效"})
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if len([]rune(req.Reason)) > 500 {
		c.JSON(400, gin.H{"code": 400, "msg": "删除原因不能超过 500 个字符"})
		return
	}
	uid := entity.GetUserIDFromCtx(c)
	cancelKey := fmt.Sprintf("cancel:tender_parse_v3:%d", id)
	if err := s.redis.Client().Set(c, cancelKey, "1", 24*time.Hour).Err(); err != nil {
		c.JSON(503, gin.H{"code": 503, "msg": "解析任务暂时无法安全撤销，请稍后重试"})
		return
	}
	jobID, err := s.repo.DeleteProject(c, id, uid, uid, req.Reason)
	if err != nil {
		if cleanupErr := s.redis.Client().Del(c, cancelKey).Err(); cleanupErr != nil {
			s.logger.Warnw("删除V3项目失败后回滚取消标记失败", "project_id", id, "err", cleanupErr)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": "项目不存在"})
			return
		}
		s.logger.Errorw("删除V3项目事务失败", "project_id", id, "user_id", uid, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": "删除项目失败，请稍后重试"})
		return
	}
	go func() {
		if err := s.processDeletionJob(context.Background(), jobID, id); err != nil {
			s.logger.Warnw("V3项目后台对象清理未完成", "project_id", id, "job_id", jobID, "err", err)
		}
	}()
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"deletion_job_id": jobID}, "msg": "项目已从工作区移除，关联文件正在清理"})
}
