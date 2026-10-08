package bidreview

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
	repoLLM "bid-engine/pkg/repo/llm"
)

// maxUploadFileSize 上传文件大小上限：100MB
const maxUploadFileSize int64 = 100 * 1024 * 1024

// ================================================================
// DTO
// ================================================================

type projectDTO struct {
	ID                    int64             `json:"id"`
	Name                  string            `json:"name"`
	CreateType            string            `json:"create_type"`
	Status                string            `json:"status"`
	Stage                 string            `json:"stage"`
	Progress              int32             `json:"progress"`
	RunCount              int32             `json:"run_count"`
	StageStatus           map[string]string `json:"stage_status"`
	RetryableStage        string            `json:"retryable_stage"`
	LastError             string            `json:"last_error"`
	StartedAt             *time.Time        `json:"started_at"`
	FinishedAt            *time.Time        `json:"finished_at"`
	CancelledAt           *time.Time        `json:"cancelled_at"`
	SourceBidGenProjectID int64             `json:"source_bid_gen_project_id"`
	IsAnonymous           bool              `json:"is_anonymous"`
	TenderFileCount       int32             `json:"tender_file_count"`
	BidFileCount          int32             `json:"bid_file_count"`
	TotalItems            int32             `json:"total_items"`
	PassedItems           int32             `json:"passed_items"`
	WarningItems          int32             `json:"warning_items"`
	ErrorItems            int32             `json:"error_items"`
	TodoItems             int32             `json:"todo_items"`
	ScoringTotal          float64           `json:"scoring_total"`
	ScoringMax            float64           `json:"scoring_max"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

func toProjectDTO(p *model.BidReviewV2Project) projectDTO {
	stageStatus := parseStageStatusJSON(p.StageStatus)
	currentStage := bidreviewRepo.NormalizeStage(p.Stage)
	return projectDTO{
		ID: p.ID, Name: p.Name, CreateType: p.CreateType, Status: p.Status,
		Stage: currentStage, Progress: p.Progress, StageStatus: stageStatus,
		RunCount:              p.RunCount,
		RetryableStage:        bidreviewRepo.ResolveRetryableStage(stageStatus, currentStage),
		LastError:             p.LastError,
		StartedAt:             p.StartedAt,
		FinishedAt:            p.FinishedAt,
		CancelledAt:           p.CancelledAt,
		SourceBidGenProjectID: p.SourceBidGenProjectID,
		IsAnonymous:           p.IsAnonymous,
		TenderFileCount:       p.TenderFileCount,
		BidFileCount:          p.BidFileCount,
		TotalItems:            p.TotalItems,
		PassedItems:           p.PassedItems,
		WarningItems:          p.WarningItems,
		ErrorItems:            p.ErrorItems,
		TodoItems:             p.TodoItems,
		ScoringTotal:          p.ScoringTotal,
		ScoringMax:            p.ScoringMax,
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
	}
}

type checklistItemDTO struct {
	ID               int64         `json:"id"`
	ProjectID        int64         `json:"project_id"`
	ItemKey          string        `json:"item_key"`
	Dimension        string        `json:"dimension"`
	Category         string        `json:"category"`
	Title            string        `json:"title"`
	Requirement      string        `json:"requirement"`
	ExpectedEvidence string        `json:"expected_evidence"`
	Severity         string        `json:"severity"`
	Source           string        `json:"source"`
	RuleID           int64         `json:"rule_id"`
	TenderPage       int32         `json:"tender_page"`
	TenderQuote      string        `json:"tender_quote"`
	FullScore        float64       `json:"full_score"`
	ReviewStatus     string        `json:"review_status"`
	ReviewNote       string        `json:"review_note"`
	ReviewedBy       int64         `json:"reviewed_by"`
	ReviewedAt       *time.Time    `json:"reviewed_at"`
	SortOrder        int32         `json:"sort_order"`
	IsUserEdited     bool          `json:"is_user_edited"`
	Finding          *findingDTO   `json:"finding"`
	Evidences        []evidenceDTO `json:"evidences"`
}

type findingDTO struct {
	ID         int64     `json:"id"`
	Status     string    `json:"status"`
	Severity   string    `json:"severity"`
	Reason     string    `json:"reason"`
	Suggestion string    `json:"suggestion"`
	Confidence string    `json:"confidence"`
	Engine     string    `json:"engine"`
	Model      string    `json:"model"`
	LatencyMS  int64     `json:"latency_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

type evidenceDTO struct {
	ID         int64   `json:"id"`
	Side       string  `json:"side"`
	FileID     int64   `json:"file_id"`
	FileName   string  `json:"file_name"`
	PageNo     int32   `json:"page_no"`
	Quote      string  `json:"quote"`
	BBoxLeft   float64 `json:"bbox_left"`
	BBoxTop    float64 `json:"bbox_top"`
	BBoxWidth  float64 `json:"bbox_width"`
	BBoxHeight float64 `json:"bbox_height"`
	MatchScore float64 `json:"match_score"`
}

type remediationDTO struct {
	ID          int64      `json:"id"`
	ItemID      int64      `json:"checklist_item_id"`
	Dimension   string     `json:"dimension"`
	Title       string     `json:"title"`
	Suggestion  string     `json:"suggestion"`
	Severity    string     `json:"severity"`
	Status      string     `json:"status"`
	OwnerUserID int64      `json:"owner_user_id"`
	Note        string     `json:"note"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

type ruleDTO struct {
	ID               int64     `json:"id"`
	Dimension        string    `json:"dimension"`
	Category         string    `json:"category"`
	Title            string    `json:"title"`
	Requirement      string    `json:"requirement"`
	ExpectedEvidence string    `json:"expected_evidence"`
	Severity         string    `json:"severity"`
	AppliesWhen      string    `json:"applies_when"`
	Enabled          bool      `json:"enabled"`
	HitCount         int32     `json:"hit_count"`
	Version          int32     `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// stageDTO 阶段目录 + 运行状态（前端阶段面板用）
type stageDTO struct {
	Stage      string     `json:"stage"`
	Label      string     `json:"label"`
	Order      int32      `json:"order"`
	Weight     int32      `json:"weight"`
	Status     string     `json:"status"`
	Attempts   int32      `json:"attempts"`
	Total      int32      `json:"total"`
	Completed  int32      `json:"completed"`
	Failed     int32      `json:"failed"`
	Progress   int32      `json:"progress"`
	LastError  string     `json:"last_error"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Retryable  bool       `json:"retryable"`
	Hint       string     `json:"hint"`
}

// buildStageDTOs 合并阶段目录、stage_status 与阶段运行记录
func buildStageDTOs(proj *model.BidReviewV2Project, runs []*model.BidReviewV2StageRun) []stageDTO {
	runByStage := make(map[string]*model.BidReviewV2StageRun, len(runs))
	for _, r := range runs {
		runByStage[r.Stage] = r
	}
	stageStatus := parseStageStatusJSON(proj.StageStatus)
	activeTask := proj.Status == bidreviewRepo.ProjectStatusRunning
	out := make([]stageDTO, 0, len(bidreviewRepo.StageDefinitions))
	for _, def := range bidreviewRepo.StageDefinitions {
		status := stageStatus[def.Name]
		if status == "" {
			status = bidreviewRepo.StageStatusPending
		}
		dto := stageDTO{
			Stage: def.Name, Label: def.Label, Order: def.Order, Weight: def.Weight,
			Status: status,
			// 执行中的项目不允许重跑（避免与运行中的任务并发），其余情况均可从该阶段重跑
			Retryable: !activeTask,
		}
		if status == bidreviewRepo.StageStatusRunning && activeTask {
			dto.Hint = "执行中"
		}
		if r, ok := runByStage[def.Name]; ok {
			dto.Attempts = r.Attempts
			dto.Total = r.Total
			dto.Completed = r.Completed
			dto.Failed = r.Failed
			dto.Progress = r.Progress
			dto.LastError = r.LastError
			dto.StartedAt = r.StartedAt
			dto.FinishedAt = r.FinishedAt
			if status == bidreviewRepo.StageStatusRunning {
				var detail struct {
					Phase string `json:"phase"`
				}
				if json.Unmarshal([]byte(r.DetailJSON), &detail) == nil && strings.TrimSpace(detail.Phase) != "" {
					dto.Hint = detail.Phase
				}
			}
		}
		out = append(out, dto)
	}
	return out
}

// ================================================================
// 项目管理
// ================================================================

func (s *svcImpl) checkLLM(c *gin.Context, userID int64) error {
	if err := s.llm.LlmConfigAvailable(repoLLM.WithUserID(c.Request.Context(), userID), repoLLM.ModuleBidReview); err != nil {
		s.logger.Warnw("标书审核前置 LLM 配置检查失败", "userID", userID, "err", err)
		code, msg := repoLLM.LLMErrorMeta(err)
		c.JSON(400, gin.H{"code": 400, "llm_code": code, "msg": msg})
		return err
	}
	return nil
}

// CreateProject 上传招/投文件创建审核项目（multipart）
func (s *svcImpl) CreateProject(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if err := s.checkLLM(c, userID); err != nil {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请以 multipart/form-data 上传文件"})
		return
	}
	tenderFiles := form.File["tender_files"]
	bidFiles := form.File["bid_files"]
	if len(tenderFiles) == 0 || len(bidFiles) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "请同时上传招标文件与投标文件"})
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = strings.TrimSuffix(bidFiles[0].Filename, filepath.Ext(bidFiles[0].Filename))
	}
	proj := &model.BidReviewV2Project{
		Name: name, CreateType: "manual", Status: bidreviewRepo.ProjectStatusRunning,
		Stage: bidreviewRepo.StageTenderParse, StageStatus: "{}",
		TenderFileCount: int32(len(tenderFiles)), BidFileCount: int32(len(bidFiles)),
		IsAnonymous: c.PostForm("is_anonymous") == "true" || c.PostForm("is_anonymous") == "1",
		UserID:      userID,
	}
	s.fillUserScope(c, proj)

	if err := s.repo.AddProject(c.Request.Context(), proj); err != nil {
		s.logger.Errorw("创建审核项目失败", "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}
	if err := s.storeUploadedFiles(c, proj.ID, tenderFiles, "tender"); err != nil {
		s.failProjectCreate(c, proj, err)
		return
	}
	if err := s.storeUploadedFiles(c, proj.ID, bidFiles, "bid"); err != nil {
		s.failProjectCreate(c, proj, err)
		return
	}
	s.initProjectAndEnqueue(c, proj, "")
}

// CreateFromGen 从标书生成项目发起审核
func (s *svcImpl) CreateFromGen(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if err := s.checkLLM(c, userID); err != nil {
		return
	}
	genProjectID := parseInt64(c.PostForm("bid_gen_project_id"))
	if genProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "bid_gen_project_id 不能为空"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请以 multipart/form-data 上传投标文件"})
		return
	}
	bidFiles := form.File["bid_files"]
	extraTenderFiles := form.File["tender_files"]
	if len(bidFiles) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "请上传投标文件"})
		return
	}
	genProj, err := s.genRepo.GetProjectForUser(c.Request.Context(), userID, genProjectID)
	if err != nil || genProj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "标书生成项目不存在"})
		return
	}
	if genProj.ReviewProjectID != 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "该标书已生成过审核项目，请勿重复创建"})
		return
	}

	proj := &model.BidReviewV2Project{
		Name: genProj.Name, CreateType: "gen", Status: bidreviewRepo.ProjectStatusRunning,
		Stage: bidreviewRepo.StageTenderParse, StageStatus: "{}",
		SourceBidGenProjectID: genProjectID, AnalysisProjectID: genProj.TenderProjectID,
		BidFileCount: int32(len(bidFiles)), IsAnonymous: c.PostForm("is_anonymous") == "true",
		UserID: userID,
	}
	s.fillUserScope(c, proj)
	if err := s.repo.AddProject(c.Request.Context(), proj); err != nil {
		s.logger.Errorw("创建审核项目失败(gen)", "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}
	if err := s.genRepo.UpdateProjectFields(c.Request.Context(), genProjectID, map[string]interface{}{
		"review_project_id": proj.ID,
	}); err != nil {
		s.logger.Warnw("标记标书已生成审核项目失败", "gen_project_id", genProjectID, "err", err)
	}

	tenderCount := 0
	if genProj.TenderProjectID > 0 {
		analysisProj, aerr := s.analysisRepo.Project(c.Request.Context(), genProj.TenderProjectID, 0)
		if aerr == nil && analysisProj != nil && strings.TrimSpace(analysisProj.SourceObject) != "" {
			fileRow := &model.BidReviewV2File{
				ProjectID: proj.ID, FileType: "tender", FileRole: "main",
				FileName: analysisProj.SourceFileName, FileBucket: analysisProj.SourceBucket,
				FileObject: analysisProj.SourceObject, FileURL: analysisProj.SourceObject,
				SortOrder: 1, ParseStatus: "pending",
			}
			if err := s.repo.BatchCreateFiles(c, []*model.BidReviewV2File{fileRow}); err != nil {
				s.failProjectCreate(c, proj, err)
				return
			}
			tenderCount = 1
		}
	}
	// 兜底：标书生成项目自带招标来源文件（如从招标文件创建但解析项目不可用时）直接复用
	if tenderCount == 0 && strings.TrimSpace(genProj.SourceFileObject) != "" {
		fileRow := &model.BidReviewV2File{
			ProjectID: proj.ID, FileType: "tender", FileRole: "main",
			FileName:   firstNonEmpty(strings.TrimSpace(genProj.SourceFileName), "招标文件"),
			FileBucket: genProj.SourceFileBucket, FileObject: genProj.SourceFileObject,
			FileURL:   firstNonEmpty(genProj.SourceFileURL, genProj.SourceFileObject),
			SortOrder: 1, ParseStatus: "pending",
		}
		if err := s.repo.BatchCreateFiles(c, []*model.BidReviewV2File{fileRow}); err != nil {
			s.failProjectCreate(c, proj, err)
			return
		}
		tenderCount = 1
		s.logger.Infow("审核项目复用标书生成项目自带招标文件", "project_id", proj.ID, "gen_project_id", genProjectID)
	}
	if len(extraTenderFiles) > 0 {
		if err := s.storeUploadedFiles(c, proj.ID, extraTenderFiles, "tender"); err != nil {
			s.failProjectCreate(c, proj, err)
			return
		}
		tenderCount += len(extraTenderFiles)
	}
	if err := s.storeUploadedFiles(c, proj.ID, bidFiles, "bid"); err != nil {
		s.failProjectCreate(c, proj, err)
		return
	}
	if tenderCount == 0 {
		if _, cleanupErr := s.repo.DeleteProjectCascadeForUser(c.Request.Context(), proj.UserID, proj.ID); cleanupErr != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "创建失败且清理未完成，请稍后重试"})
			return
		}
		c.JSON(400, gin.H{"code": 400, "msg": "该标书生成项目未关联招标文件，请补充上传招标文件后重试"})
		return
	}
	_ = s.repo.UpdateProjectFields(c.Request.Context(), proj.ID, map[string]interface{}{
		"tender_file_count": tenderCount,
	})
	s.initProjectAndEnqueue(c, proj, "")
}

// UpdateAnonymousFlag 切换暗标评审开关
func (s *svcImpl) UpdateAnonymousFlag(c *gin.Context) {
	projectID := parseInt64(c.Param("id"))
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.GetProjectForUser(c.Request.Context(), userID, projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	isAnonymous := c.PostForm("is_anonymous") == "true" || c.PostForm("is_anonymous") == "1"
	if err := s.repo.UpdateProjectFields(c.Request.Context(), projectID, map[string]interface{}{
		"is_anonymous": isAnonymous,
	}); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "已更新"})
}

// PageListProject 项目列表
func (s *svcImpl) PageListProject(c *gin.Context) {
	var req entity.BidReviewListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		keyword = strings.TrimSpace(req.Name)
	}
	userID := entity.GetUserIDFromCtx(c)
	records, total, err := s.repo.GetProjectsForUser(c.Request.Context(), userID, req.PageNum, req.PageSize, req.Status, keyword)
	if err != nil {
		s.logger.Errorw("查询审核项目列表失败", "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	items := make([]projectDTO, 0, len(records))
	for _, p := range records {
		items = append(items, toProjectDTO(p))
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"list": items, "total": total}})
}

// GetProjectDetail 项目详情（项目 + 文件 + 维度 + 清单 + 证据 + 整改 + 阶段）
func (s *svcImpl) GetProjectDetail(c *gin.Context) {
	projectID := parseInt64(c.Query("project_id"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id 不能为空"})
		return
	}
	ctx := c.Request.Context()
	proj, err := s.repo.GetProjectForUser(ctx, entity.GetUserIDFromCtx(c), projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}

	files, _ := s.repo.GetFilesByProjectAndType(ctx, projectID, "")
	if files == nil {
		files = []*model.BidReviewV2File{}
	}
	items, _ := s.repo.GetChecklistItems(ctx, projectID)
	findings, _ := s.repo.GetLatestFindings(ctx, projectID)
	evidences, _ := s.repo.GetEvidencesByProject(ctx, projectID)
	remediations, _ := s.repo.GetRemediations(ctx, projectID)
	stageRuns, _ := s.repo.GetStageRuns(ctx, projectID)
	opLogs, _ := s.repo.GetOpLogs(ctx, projectID, 50)
	stats, scoreTotal, scoreMax, _ := s.buildScorecard(ctx, projectID)

	findingByItem := make(map[int64]*model.BidReviewV2Finding, len(findings))
	for _, f := range findings {
		findingByItem[f.ChecklistItemID] = f
	}
	evidencesByItem := make(map[int64][]evidenceDTO, len(items))
	for _, e := range evidences {
		evidencesByItem[e.ChecklistItemID] = append(evidencesByItem[e.ChecklistItemID], evidenceDTO{
			ID: e.ID, Side: e.Side, FileID: e.FileID, FileName: e.FileName, PageNo: e.PageNo,
			Quote: e.Quote, BBoxLeft: e.BBoxLeft, BBoxTop: e.BBoxTop,
			BBoxWidth: e.BBoxWidth, BBoxHeight: e.BBoxHeight, MatchScore: e.MatchScore,
		})
	}

	itemDTOs := make([]checklistItemDTO, 0, len(items))
	for _, it := range items {
		dto := checklistItemDTO{
			ID: it.ID, ProjectID: it.ProjectID, ItemKey: it.ItemKey, Dimension: it.Dimension,
			Category: it.Category, Title: it.Title, Requirement: it.Requirement,
			ExpectedEvidence: it.ExpectedEvidence, Severity: it.Severity, Source: it.Source,
			RuleID: it.RuleID, TenderPage: it.TenderPage, TenderQuote: it.TenderQuote,
			FullScore:    itemFullScore(it),
			ReviewStatus: it.ReviewStatus, ReviewNote: it.ReviewNote, ReviewedBy: it.ReviewedBy,
			ReviewedAt: it.ReviewedAt, SortOrder: it.SortOrder, IsUserEdited: it.IsUserEdited,
			Evidences: evidencesByItem[it.ID],
		}
		if dto.Evidences == nil {
			dto.Evidences = []evidenceDTO{}
		}
		if f, ok := findingByItem[it.ID]; ok {
			dto.Finding = &findingDTO{
				ID: f.ID, Status: f.Status, Severity: f.Severity, Reason: f.Reason,
				Suggestion: f.Suggestion, Confidence: f.Confidence, Engine: f.Engine,
				Model: f.Model, LatencyMS: f.LatencyMS, CreatedAt: f.CreatedAt,
			}
		}
		itemDTOs = append(itemDTOs, dto)
	}

	remediationDTOs := make([]remediationDTO, 0, len(remediations))
	for _, r := range remediations {
		remediationDTOs = append(remediationDTOs, remediationDTO{
			ID: r.ID, ItemID: r.ChecklistItemID, Dimension: r.Dimension, Title: r.Title,
			Suggestion: r.Suggestion, Severity: r.Severity, Status: r.Status,
			OwnerUserID: r.OwnerUserID, Note: r.Note, ResolvedAt: r.ResolvedAt,
		})
	}

	c.JSON(200, gin.H{"code": 200, "data": gin.H{
		"project":       toProjectDTO(proj),
		"files":         files,
		"dimensions":    stats,
		"scoring_total": scoreTotal,
		"scoring_max":   scoreMax,
		"items":         itemDTOs,
		"remediations":  remediationDTOs,
		"stages":        buildStageDTOs(proj, stageRuns),
		"stage_runs":    stageRuns,
		"op_logs":       opLogs,
	}})
}

// DeleteProject 删除审核项目
func (s *svcImpl) DeleteProject(c *gin.Context) {
	projectID := parseInt64(c.Query("id"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "id 不能为空"})
		return
	}
	if _, err := s.repo.DeleteProjectCascadeForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), projectID); err != nil {
		s.logger.Errorw("删除审核项目失败", "project_id", projectID, "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "删除成功"})
}

// RetryStage 阶段断点重试（failed 阶段，或 running 残留且无活跃锁时）
func (s *svcImpl) RetryStage(c *gin.Context) {
	projectID := parseInt64(c.Param("id"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "id 不能为空"})
		return
	}
	var req entity.BidReviewStageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	if err := s.checkLLM(c, userID); err != nil {
		return
	}
	ctx := c.Request.Context()
	proj, err := s.repo.GetProjectForUser(ctx, userID, projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	stageStatus := parseStageStatusJSON(proj.StageStatus)
	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		stage = bidreviewRepo.ResolveRetryableStage(stageStatus, proj.Stage)
	}
	if stage == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "当前没有可重试的阶段"})
		return
	}
	if !bidreviewRepo.IsValidStage(stage) {
		c.JSON(400, gin.H{"code": 400, "msg": "阶段名不合法: " + stage})
		return
	}
	if stageStatus[stage] == bidreviewRepo.StageStatusRunning && !s.isStageInterrupted(ctx, projectID) {
		c.JSON(409, gin.H{"code": 409, "msg": "该阶段正在执行中"})
		return
	}
	// 有活跃任务锁时不允许并发重试
	if !s.isStageInterrupted(ctx, projectID) {
		c.JSON(409, gin.H{"code": 409, "msg": "该项目已有审核任务在执行，请稍后再试"})
		return
	}

	// 断点重跑：把该阶段及其后续全部重置为 pending，并清理其产物
	stageStatus = bidreviewRepo.ResetStageStatusFrom(stageStatus, stage)
	for _, name := range bidreviewRepo.StageOrderFrom(stage) {
		if err := s.clearStageData(ctx, proj, name); err != nil {
			s.logger.Warnw("重跑前清理阶段数据失败（继续）", "project_id", projectID, "stage", name, "err", err)
		}
	}
	stageJSON, _ := json.Marshal(stageStatus)
	progress := bidreviewRepo.BaselineProgress(stageStatus, stage)
	now := time.Now()
	if err := s.repo.UpdateProjectFields(ctx, projectID, map[string]interface{}{
		"stage_status": string(stageJSON),
		"status":       bidreviewRepo.ProjectStatusRunning,
		"stage":        stage,
		"progress":     progress,
		"last_error":   "",
		"started_at":   now,
		"finished_at":  nil,
		"cancelled_at": nil,
		"run_count":    gormExprIncr("run_count", 1),
	}); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "重试失败: " + err.Error()})
		return
	}
	// 新的一次执行：清除取消标记
	s.clearCancelFlag(ctx, projectID)
	if _, enqErr := s.enqueueReviewTask(ctx, projectID, userID, stage); enqErr != nil {
		rollbackStatus := bidreviewRepo.ResetStageStatusFrom(stageStatus, stage)
		rollbackStatus[stage] = bidreviewRepo.StageStatusFailed
		rollback, _ := json.Marshal(rollbackStatus)
		_ = s.repo.UpdateProjectFields(ctx, projectID, map[string]interface{}{
			"stage_status": string(rollback),
			"status":       bidreviewRepo.ProjectStatusFailed,
			"finished_at":  time.Now(),
			"last_error":   "阶段重试入队失败: " + enqErr.Error(),
		})
		c.JSON(500, gin.H{"code": 500, "msg": "重试入队失败: " + enqErr.Error()})
		return
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: projectID, Action: "retry_stage", OperatorID: userID,
		Detail: "从“" + bidreviewRepo.StageLabel(stage) + "”阶段重跑",
	})
	c.JSON(200, gin.H{"code": 200, "msg": "重跑已开始", "data": gin.H{
		"stage":        stage,
		"stage_status": stageStatus,
		"progress":     progress,
	}})
}

// CancelProject 取消正在执行的审核任务（阶段边界生效）
func (s *svcImpl) CancelProject(c *gin.Context) {
	projectID := parseInt64(c.Param("id"))
	if projectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "id 不能为空"})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.GetProjectForUser(ctx, userID, projectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status != bidreviewRepo.ProjectStatusRunning {
		c.JSON(400, gin.H{"code": 400, "msg": "仅执行中的审核项目可取消"})
		return
	}
	if err := s.redisSvc.Client().Set(ctx, cancelKey(projectID), "1", 6*time.Hour).Err(); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "取消失败: " + err.Error()})
		return
	}
	// 无活跃任务（已中断）时立即置为取消态，避免用户等待
	if s.isStageInterrupted(ctx, projectID) {
		now := time.Now()
		_ = s.repo.UpdateProjectFields(ctx, projectID, map[string]interface{}{
			"status":       bidreviewRepo.ProjectStatusCancelled,
			"finished_at":  now,
			"cancelled_at": now,
		})
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: projectID, Action: "cancel", OperatorID: userID,
		Detail: "取消审核任务",
	})
	c.JSON(200, gin.H{"code": 200, "msg": "已请求取消，将在当前阶段结束后停止"})
}

// isStageInterrupted 判断项目当前是否没有活跃任务（无锁）
func (s *svcImpl) isStageInterrupted(ctx context.Context, projectID int64) bool {
	lockKey := fmt.Sprintf("lock:bid_review:%d", projectID)
	n, err := s.redisSvc.Client().Exists(ctx, lockKey).Result()
	return err == nil && n == 0
}

// GetSourcePdf 原文 PDF 流（溯源预览）
func (s *svcImpl) GetSourcePdf(c *gin.Context) {
	projectID := parseInt64(c.Query("project_id"))
	fileID := parseInt64(c.Query("file_id"))
	if projectID <= 0 || fileID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id / file_id 不能为空"})
		return
	}
	ctx := c.Request.Context()
	fileRow, err := s.repo.GetFileByID(ctx, fileID)
	if err != nil || fileRow == nil || fileRow.ProjectID != projectID {
		c.JSON(404, gin.H{"code": 404, "msg": "文件不存在"})
		return
	}
	if _, err := s.repo.GetProjectForUser(ctx, entity.GetUserIDFromCtx(c), fileRow.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "文件不存在"})
		return
	}
	if err := os.MkdirAll("./tmp", 0o777); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "准备预览文件失败"})
		return
	}
	if strings.TrimSpace(fileRow.PdfObject) != "" {
		localCached := filepath.Join("./tmp", fmt.Sprintf("bid-review-pdf-%d-%d.pdf", fileID, time.Now().UnixNano()))
		if err := s.oss.Get(ctx, fileRow.PdfObject, localCached); err == nil {
			defer func() { _ = os.Remove(localCached) }()
			s.streamPdf(c, localCached, fileRow.FileName)
			return
		}
	}
	objectKey := strings.TrimSpace(fileRow.FileObject)
	if objectKey == "" {
		c.JSON(404, gin.H{"code": 404, "msg": "文件对象缺失"})
		return
	}
	srcExt := strings.ToLower(filepath.Ext(strings.TrimSpace(fileRow.FileName)))
	if srcExt == "" {
		srcExt = ".pdf"
	}
	localSrc := filepath.Join("./tmp", fmt.Sprintf("bid-review-src-%d-%d%s", fileID, time.Now().UnixNano(), srcExt))
	if err := s.oss.Get(ctx, objectKey, localSrc); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "下载原文件失败"})
		return
	}
	defer func() { _ = os.Remove(localSrc) }()
	localPdf := localSrc
	if srcExt != ".pdf" {
		localPdf = filepath.Join("./tmp", fmt.Sprintf("bid-review-src-%d-%d.pdf", fileID, time.Now().UnixNano()))
		if cerr := s.pdf.Convert2Pdf(c, localSrc, localPdf); cerr != nil {
			if cerr2 := s.pdf.Convert2PdfBySoffice(c, localSrc, localPdf); cerr2 != nil {
				c.JSON(500, gin.H{"code": 500, "msg": "原文件转PDF失败"})
				return
			}
		}
		defer func() { _ = os.Remove(localPdf) }()
	}
	s.streamPdf(c, localPdf, fileRow.FileName)
}

func (s *svcImpl) streamPdf(c *gin.Context, localPdf, fileName string) {
	c.Header("Content-Type", "application/pdf")
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "source.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	name = strings.ReplaceAll(name, "\"", "")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", name))
	c.File(localPdf)
}

// ================================================================
// 创建辅助
// ================================================================

func (s *svcImpl) fillUserScope(c *gin.Context, proj *model.BidReviewV2Project) {
	if teamID, exists := c.Get("team_id"); exists {
		if v, ok := teamID.(int32); ok {
			proj.UserTeamID = v
		}
	}
	if companyID, exists := c.Get("company_id"); exists {
		if v, ok := companyID.(int32); ok {
			proj.UserCompanyID = v
		}
	}
}

func (s *svcImpl) failProjectCreate(c *gin.Context, proj *model.BidReviewV2Project, err error) {
	s.logger.Errorw("创建审核项目失败", "project_id", proj.ID, "err", err)
	_ = s.repo.UpdateProjectFields(c.Request.Context(), proj.ID, map[string]interface{}{
		"status":     bidreviewRepo.ProjectStatusFailed,
		"last_error": "创建失败: " + err.Error(),
	})
	c.JSON(500, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
}

func (s *svcImpl) storeUploadedFiles(c *gin.Context, projectID int64, files []*multipart.FileHeader, fileType string) error {
	if len(files) == 0 {
		return nil
	}
	_ = os.MkdirAll("./tmp", 0o777)
	rows := make([]*model.BidReviewV2File, 0, len(files))
	for i, fh := range files {
		if fh.Size > maxUploadFileSize {
			return fmt.Errorf("文件 %s 大小超过 100MB 限制", fh.Filename)
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		tmpPath := filepath.Join("./tmp", fmt.Sprintf("bid-review-%d-%d%s", projectID, time.Now().UnixNano(), ext))
		if err := c.SaveUploadedFile(fh, tmpPath); err != nil {
			return fmt.Errorf("保存文件失败: %w", err)
		}
		objectKey := fmt.Sprintf("bid-review/%d/%d-%d%s", projectID, time.Now().Unix(), time.Now().UnixNano(), ext)
		if err := s.oss.Put(c.Request.Context(), objectKey, tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("上传文件失败: %w", err)
		}
		_ = os.Remove(tmpPath)
		role := "main"
		if i > 0 && fileType == "tender" {
			role = "addendum"
		}
		rows = append(rows, &model.BidReviewV2File{
			ProjectID: projectID, FileType: fileType, FileRole: role, FileName: fh.Filename,
			FileBucket: s.oss.GetDefaultBucketName(), FileObject: objectKey, FileURL: objectKey,
			SortOrder: int32(i + 1), ParseStatus: "pending",
		})
	}
	return s.repo.BatchCreateFiles(c.Request.Context(), rows)
}

func (s *svcImpl) initProjectAndEnqueue(c *gin.Context, proj *model.BidReviewV2Project, resumeFromStage string) {
	stageStatus := bidreviewRepo.NewStageStatusMap()
	stageJSON, _ := json.Marshal(stageStatus)
	_ = s.repo.UpdateProjectFields(c.Request.Context(), proj.ID, map[string]interface{}{
		"stage_status": string(stageJSON),
		"last_error":   "",
		"status":       bidreviewRepo.ProjectStatusRunning,
		"stage":        bidreviewRepo.StageTenderParse,
	})
	if _, err := s.enqueueReviewTask(c.Request.Context(), proj.ID, proj.UserID, resumeFromStage); err != nil {
		s.logger.Errorw("审核任务入队失败", "project_id", proj.ID, "err", err)
		_ = s.repo.UpdateProjectFields(c.Request.Context(), proj.ID, map[string]interface{}{
			"status":     bidreviewRepo.ProjectStatusFailed,
			"last_error": "任务入队失败: " + err.Error(),
		})
		c.JSON(500, gin.H{"code": 500, "msg": "任务入队失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": proj.ID}})
}

func (s *svcImpl) enqueueReviewTask(ctx context.Context, projectID, userID int64, resumeFromStage string) (string, error) {
	return s.taskqueueRepo.Enqueue(ctx, "bid_review", ReviewTaskPayload{
		ProjectID: projectID, UserID: userID, ResumeFromStage: resumeFromStage,
	}, taskqueueEnqueueOpts(userID, projectID))
}
