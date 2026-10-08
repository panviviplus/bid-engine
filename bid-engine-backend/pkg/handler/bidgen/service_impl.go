package bidgen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
	"bid-engine/pkg/service/biddoc"
)

// errProjectNotFound 项目不存在（单删返回 404，批量计入 failed）
var errProjectNotFound = errors.New("项目不存在")

// 标书生成项目状态
const (
	ProjectStatusParsing       = "parsing"        // 异步解析中（从招标文件/模板创建）
	ProjectStatusOutlineReview = "outline_review" // 大纲待确认
	ProjectStatusDraft         = "draft"          // 可编辑/可生成
	ProjectStatusGenerating    = "generating"     // AI 生成中
	ProjectStatusSucceeded     = "succeeded"      // 已完成
	ProjectStatusFailed        = "failed"         // 失败
	parseStagePending          = "pending"
	parseStageRunning          = "running"
	parseStageSucceeded        = "succeeded"
	parseStageFailed           = "failed"
)

// 解析展示阶段（写 bid_gen_project.stage_status）
const (
	GenStageTenderInterpretation = "tender_interpretation" // 招标文件解读
	GenStageInfoExtraction       = "info_extraction"       // 提炼重要信息
	GenStageBlueprintGeneration  = "blueprint_generation"  // 大纲蓝图生成
	GenStageTemplateParse        = "template_parse"        // 模板文档解析
	GenStageTemplateOutline      = "template_outline"      // 模板大纲提取
)

// 章节生成状态
const (
	OutlineGenPending    = "pending"
	OutlineGenGenerating = "generating"
	OutlineGenSucceeded  = "succeeded"
	OutlineGenFailed     = "failed"
)

// 任务类型
const (
	TaskTypeFull    = "full"    // 整篇生成
	TaskTypeChapter = "chapter" // 指定章节生成
	TaskTypeParse   = "parse"   // 解析（招标文件/模板）
)

// 篇幅档位
const (
	LengthConcise  = "concise"
	LengthStandard = "standard"
	LengthDetailed = "detailed"
)

// maxUploadFileSize 上传文件大小上限：100MB
const maxUploadFileSize int64 = 100 * 1024 * 1024

// ================================================================
// 项目管理
// ================================================================

// CreateProject 创建空白标书
func (s *svcImpl) CreateProject(c *gin.Context) {
	var req entity.BidGenCreateBlankReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "项目名称不能为空"})
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	proj := &model.BidGenProject{
		Name:       strings.TrimSpace(req.Name),
		CreateType: "blank",
		Status:     ProjectStatusOutlineReview,
		UserID:     userID,
	}
	if teamID, exists := c.Get("team_id"); exists {
		proj.UserTeamID = teamID.(int32)
	}
	if companyID, exists := c.Get("company_id"); exists {
		proj.UserCompanyID = companyID.(int32)
	}
	if err := s.repo.AddProject(c, proj); err != nil {
		s.logger.Errorw("创建空白标书失败", "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "创建标书项目失败，请重试"})
		return
	}
	doc, _ := buildDocFromOutline(nil)
	if err := s.repo.UpsertDocContent(c, &model.BidGenDocContent{ProjectID: proj.ID, DocJSON: doc}); err != nil {
		s.logger.Errorw("初始化空白标书文档失败", "project_id", proj.ID, "err", err)
		if _, cleanupErr := s.repo.DeleteProjectCascadeForUser(c, userID, proj.ID); cleanupErr != nil {
			s.logger.Errorw("初始化空白标书失败后清理项目失败", "project_id", proj.ID, "err", cleanupErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "初始化标书文档失败，请重试"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": proj.ID}})
}

// CreateFromTender 从招标文件创建：上传 → 建内部招标解析项目 → 入队 bid_gen_parse
func (s *svcImpl) CreateFromTender(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	// 解析管线复用招标解析模块（feature 归属 ModuleBidAnalysis），创建前做可用性检查
	if err := s.llm.LlmConfigAvailable(repoLLM.WithUserID(c.Request.Context(), userID), repoLLM.ModuleBidAnalysis); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "招标解析模型未配置: " + err.Error()})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请上传招标文件"})
		return
	}
	if file.Size > maxUploadFileSize {
		c.JSON(400, gin.H{"code": 400, "msg": "文件大小不能超过 100MB"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !isSupportedDoc(ext) {
		c.JSON(400, gin.H{"code": 400, "msg": "不支持的文件格式: " + ext})
		return
	}
	_ = os.MkdirAll("./tmp", 0777)
	tmpPath := filepath.Join("./tmp", fmt.Sprintf("bidgen-tender-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "文件保存失败: " + err.Error()})
		return
	}
	defer os.Remove(tmpPath)

	objectKey := fmt.Sprintf("bid-gen/tender/%d/%d%s", time.Now().Unix(), time.Now().UnixNano(), ext)
	if err := s.oss.Put(c, objectKey, tmpPath); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "文件上传失败: " + err.Error()})
		return
	}

	// 1. 创建内部招标解析 V3 项目（is_internal=1，不出现在招标解析列表）。
	sha, shaErr := sha256File(tmpPath)
	if shaErr != nil {
		_ = s.oss.Delete(c, objectKey)
		c.JSON(500, gin.H{"code": 500, "msg": "计算文件摘要失败: " + shaErr.Error()})
		return
	}
	var teamID, companyID int32
	if value, exists := c.Get("team_id"); exists {
		teamID, _ = value.(int32)
	}
	if user := entity.GetUserFromCtx(c); user != nil {
		companyID = user.CompanyID
	}
	analysisProj, _, err := s.analysisSvc.CreateInternalProject(c, strings.TrimSuffix(file.Filename, ext), file.Filename, s.oss.GetDefaultBucketName(), objectKey, sha, userID, teamID, companyID)
	if err != nil {
		_ = s.oss.Delete(c, objectKey)
		c.JSON(500, gin.H{"code": 500, "msg": "创建解析项目失败: " + err.Error()})
		return
	}

	// 2. 创建标书生成项目
	name := strings.TrimSuffix(file.Filename, ext)
	proj := &model.BidGenProject{
		Name:             name,
		CreateType:       "tender_file",
		Status:           ProjectStatusParsing,
		TenderProjectID:  analysisProj.ID,
		SourceFileBucket: s.oss.GetDefaultBucketName(),
		SourceFileName:   file.Filename,
		SourceFileObject: objectKey,
		SourceFileURL:    objectKey,
		UserID:           userID,
	}
	if teamID, exists := c.Get("team_id"); exists {
		proj.UserTeamID = teamID.(int32)
	}
	if companyID, exists := c.Get("company_id"); exists {
		proj.UserCompanyID = companyID.(int32)
	}
	if err := s.repo.AddProject(c, proj); err != nil {
		s.logger.Errorw("创建招标文件标书项目失败", "analysis_project_id", analysisProj.ID, "err", err)
		if cleanupErr := s.analysisSvc.DeleteInternalProject(c, analysisProj.ID, userID); cleanupErr != nil {
			s.logger.Errorw("创建标书项目失败后清理内部解析项目失败", "analysis_project_id", analysisProj.ID, "err", cleanupErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "创建标书项目失败，请重试"})
		return
	}
	_ = s.initGenStageStatus(c, proj.ID, []string{GenStageTenderInterpretation, GenStageInfoExtraction, GenStageBlueprintGeneration})

	// 3. 入队解析任务
	if err := s.enqueueParseTask(c, proj.ID, analysisProj.ID, userID, "tender"); err != nil {
		_ = s.repo.UpdateProjectFields(c, proj.ID, map[string]interface{}{
			"status": ProjectStatusFailed, "last_error": "解析任务入队失败: " + err.Error(),
		})
		c.JSON(500, gin.H{"code": 500, "msg": "解析任务入队失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": proj.ID, "status": ProjectStatusParsing}})
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CreateFromTemplate 从模板创建：上传模板 → 异步 docling 解析 + 大纲提取
func (s *svcImpl) CreateFromTemplate(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)

	var sourceObject, sourceName, sourceBucket string
	file, err := c.FormFile("file")
	if err == nil {
		if file.Size > maxUploadFileSize {
			c.JSON(400, gin.H{"code": 400, "msg": "文件大小不能超过 100MB"})
			return
		}
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if !isSupportedTemplate(ext) {
			c.JSON(400, gin.H{"code": 400, "msg": "不支持的文件格式: " + ext})
			return
		}
		_ = os.MkdirAll("./tmp", 0777)
		tmpPath := filepath.Join("./tmp", fmt.Sprintf("bidgen-tpl-%d%s", time.Now().UnixNano(), ext))
		if err := c.SaveUploadedFile(file, tmpPath); err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "文件保存失败: " + err.Error()})
			return
		}
		defer os.Remove(tmpPath)
		objectKey := fmt.Sprintf("bid-gen/template/%d/%d%s", time.Now().Unix(), time.Now().UnixNano(), ext)
		if err := s.oss.Put(c, objectKey, tmpPath); err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "文件上传失败: " + err.Error()})
			return
		}
		sourceObject, sourceName, sourceBucket = objectKey, file.Filename, s.oss.GetDefaultBucketName()
	} else {
		// 从素材库模板创建（template_material_id）——V1 若未传文件则报错提示
		c.JSON(400, gin.H{"code": 400, "msg": "请上传模板文件"})
		return
	}

	proj := &model.BidGenProject{
		Name:             strings.TrimSuffix(sourceName, filepath.Ext(sourceName)),
		CreateType:       "template",
		Status:           ProjectStatusParsing,
		SourceFileBucket: sourceBucket,
		SourceFileName:   sourceName,
		SourceFileObject: sourceObject,
		SourceFileURL:    sourceObject,
		UserID:           userID,
	}
	if teamID, exists := c.Get("team_id"); exists {
		proj.UserTeamID = teamID.(int32)
	}
	if companyID, exists := c.Get("company_id"); exists {
		proj.UserCompanyID = companyID.(int32)
	}
	if err := s.repo.AddProject(c, proj); err != nil {
		s.logger.Errorw("创建模板标书项目失败", "source_object", sourceObject, "err", err)
		if cleanupErr := s.oss.Delete(c, sourceObject); cleanupErr != nil {
			s.logger.Warnw("创建模板标书项目失败后清理源文件失败", "source_object", sourceObject, "err", cleanupErr)
		}
		c.JSON(500, gin.H{"code": 500, "msg": "创建标书项目失败，请重试"})
		return
	}
	_ = s.initGenStageStatus(c, proj.ID, []string{GenStageTemplateParse, GenStageTemplateOutline})

	if err := s.enqueueParseTask(c, proj.ID, 0, userID, "template"); err != nil {
		_ = s.repo.UpdateProjectFields(c, proj.ID, map[string]interface{}{
			"status": ProjectStatusFailed, "last_error": "解析任务入队失败: " + err.Error(),
		})
		c.JSON(500, gin.H{"code": 500, "msg": "解析任务入队失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": proj.ID, "status": ProjectStatusParsing}})
}

// ConfirmOutline 大纲确认：outline_review → draft
func (s *svcImpl) ConfirmOutline(c *gin.Context) {
	var req entity.BidGenConfirmOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status != ProjectStatusOutlineReview {
		c.JSON(400, gin.H{"code": 400, "msg": "当前状态不是大纲待确认"})
		return
	}
	nodes, err := s.repo.GetOutlineByProjectID(c, proj.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载大纲失败，请重试"})
		return
	}
	if len(nodes) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "请至少创建一个大纲章节后再确认"})
		return
	}
	if _, err := s.ensureDocumentAnchors(c, proj.ID, nodes); err != nil {
		s.logger.Warnw("确认大纲前正文结构校验失败", "project_id", proj.ID, "err", err)
		msg := "正文结构校验失败，请重新加载后再试"
		if errors.Is(err, errDocumentAnchorsPartial) {
			msg = err.Error()
		}
		c.JSON(400, gin.H{"code": 400, "msg": msg})
		return
	}
	if err := s.repo.UpdateProjectFields(c, proj.ID, map[string]interface{}{"status": ProjectStatusDraft}); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "确认失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "大纲确认成功"})
}

// UnconfirmOutline 撤销大纲确认：draft → outline_review（仅当无任何章节已生成/生成中）
func (s *svcImpl) UnconfirmOutline(c *gin.Context) {
	var req entity.BidGenUnconfirmOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status != ProjectStatusDraft {
		c.JSON(400, gin.H{"code": 400, "msg": "当前状态不可撤销确认"})
		return
	}
	nodes, err := s.repo.GetOutlineByProjectID(c, proj.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "加载大纲失败: " + err.Error()})
		return
	}
	if hasGeneratedOutline(nodes) {
		c.JSON(400, gin.H{"code": 400, "msg": "已有章节生成内容，无法重新编排大纲"})
		return
	}
	if err := s.repo.UpdateProjectFields(c, proj.ID, map[string]interface{}{"status": ProjectStatusOutlineReview}); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "撤销确认失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "已撤销确认，可重新编排大纲"})
}

// hasGeneratedOutline 判断大纲中是否存在已生成/生成中的章节（用于撤销确认前置校验）
func hasGeneratedOutline(nodes []*model.BidGenOutline) bool {
	for _, n := range nodes {
		if n.GenStatus == OutlineGenSucceeded || n.GenStatus == OutlineGenGenerating {
			return true
		}
	}
	return false
}

// PageListProject 项目列表
func (s *svcImpl) PageListProject(c *gin.Context) {
	var req entity.BidGenListReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 10
	}
	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	records, total, err := s.repo.GetProjectsForUser(c, entity.GetUserIDFromCtx(c), int(req.PageNum), int(req.PageSize), req.Status, req.Name)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询失败: " + err.Error()})
		return
	}
	items := make([]entity.BidGenProjectItemResp, 0, len(records))
	// 批量统计章节数（避免 N+1）
	projIDs := make([]int64, 0, len(records))
	for _, p := range records {
		projIDs = append(projIDs, p.ID)
	}
	countMap, _ := s.repo.CountOutlineByProjectIDs(c, projIDs)
	for _, p := range records {
		item := entity.BidGenProjectItemResp{
			ID:              p.ID,
			Name:            p.Name,
			CreateType:      p.CreateType,
			Status:          p.Status,
			Progress:        p.Progress,
			Stage:           p.Stage,
			SourceFileName:  p.SourceFileName,
			StageStatus:     parseStageStatusJSON(p.StageStatus),
			ReviewProjectID: p.ReviewProjectID,
			CreatedTime:     p.CreatedAt.Unix(),
			UpdatedTime:     p.UpdatedAt.Unix(),
		}
		if v, ok := countMap[p.ID]; ok {
			item.OutlineCount = v[0]
			item.SucceededCount = v[1]
		}
		items = append(items, item)
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"list": items, "total": total}})
}

// GetProjectDetail 项目详情
func (s *svcImpl) GetProjectDetail(c *gin.Context) {
	pid := parseInt64(c.Query("project_id"))
	if pid <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id 不能为空"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), pid)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	resp := entity.BidGenProjectDetailResp{
		ID:               proj.ID,
		Name:             proj.Name,
		CreateType:       proj.CreateType,
		Status:           proj.Status,
		Progress:         proj.Progress,
		Stage:            proj.Stage,
		StageStatus:      parseStageStatusJSON(proj.StageStatus),
		ReviewProjectID:  proj.ReviewProjectID,
		SourceFileName:   proj.SourceFileName,
		SourceFileURL:    replaceHostDocker(proj.SourceFileURL),
		SourceFileObject: proj.SourceFileObject,
		LastError:        proj.LastError,
		CreatedTime:      proj.CreatedAt.Unix(),
		UpdatedTime:      proj.UpdatedAt.Unix(),
	}

	// 文档内容
	if dc, err := s.repo.GetDocContent(c, pid); err == nil && dc != nil {
		resp.DocJSON = dc.DocJSON
		resp.DocHTML = dc.DocHTML
	}

	// 大纲树
	outlineNodes, _ := s.repo.GetOutlineByProjectID(c, pid)
	resp.Outline = buildOutlineResp(outlineNodes)

	// 封面数据（导出时装配封面页使用）
	if cover := s.buildCoverData(c, proj); cover != nil {
		resp.Cover = &entity.BidGenCoverResp{
			DocTitle:      cover.DocTitle,
			ProjectName:   cover.ProjectName,
			ProjectNumber: cover.ProjectNumber,
			LotLabel:      cover.LotLabel,
			TendererName:  cover.TendererName,
			BidderName:    cover.BidderName,
			Date:          cover.Date,
		}
	}

	// 进行中任务
	if proj.Status == ProjectStatusGenerating {
		if task, err := s.repo.GetRunningTask(c, pid); err == nil && task != nil {
			value := taskResponse(task)
			resp.RunningTask = &value
		}
	}
	c.JSON(200, gin.H{"code": 200, "data": resp})
}

// deleteProjectCascade 级联删除单个标书项目（含内部招标解析项目清理，单删/批量删共用）
func (s *svcImpl) deleteProjectCascade(c *gin.Context, pid int64) error {
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.DeleteProjectCascadeForUser(c, userID, pid)
	if errors.Is(err, gorm.ErrRecordNotFound) || proj == nil {
		return errProjectNotFound
	}
	if err != nil {
		return err
	}
	// 清理内部招标解析项目（仅 create_type=tender_file 的内部解析项目；
	// create_type=analysis 关联的是用户真实的招标解析项目，禁止删除）
	if proj.TenderProjectID > 0 && proj.CreateType == "tender_file" {
		if err := s.analysisSvc.DeleteInternalProject(c, proj.TenderProjectID, proj.UserID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteProject 删除项目（级联清理）
func (s *svcImpl) DeleteProject(c *gin.Context) {
	pid := parseInt64(c.Query("project_id"))
	if pid <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id 不能为空"})
		return
	}
	if err := s.deleteProjectCascade(c, pid); err != nil {
		if errors.Is(err, errProjectNotFound) {
			c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
			return
		}
		c.JSON(500, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "删除成功"})
}

// BatchDeleteProject 批量删除项目（级联清理）
func (s *svcImpl) BatchDeleteProject(c *gin.Context) {
	var req entity.BatchDeleteBidGenProjectsReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "ids 不能为空"})
		return
	}
	deleted := make([]int64, 0, len(req.IDs))
	failed := make([]map[string]interface{}, 0)
	for _, pid := range req.IDs {
		if err := s.deleteProjectCascade(c, pid); err != nil {
			failed = append(failed, map[string]interface{}{"projectId": pid, "err": err.Error()})
			continue
		}
		deleted = append(deleted, pid)
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"deleted": deleted, "failed": failed}})
}

// SaveDocContent 自动保存文档内容
func (s *svcImpl) SaveDocContent(c *gin.Context) {
	var req entity.BidGenSaveDocReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	// 生成期间挂起自动保存
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，文档暂不可修改"})
		return
	}
	dc := &model.BidGenDocContent{
		ProjectID: req.ProjectID,
		DocJSON:   req.DocJSON,
		DocHTML:   req.DocHTML,
	}
	if err := s.repo.UpsertDocContent(c, dc); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "保存成功"})
}

// ================================================================
// 大纲管理
// ================================================================

func (s *svcImpl) GetOutline(c *gin.Context) {
	pid := parseInt64(c.Query("project_id"))
	if pid <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "project_id 不能为空"})
		return
	}
	if _, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), pid); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	nodes, _ := s.repo.GetOutlineByProjectID(c, pid)
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"outline": buildOutlineResp(nodes)}})
}

func (s *svcImpl) AddOutlineNode(c *gin.Context) {
	var req entity.BidGenAddOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 || strings.TrimSpace(req.Title) == "" {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，大纲暂不可修改"})
		return
	}
	if req.Level <= 0 {
		req.Level = 2
	}
	if req.Level > 4 {
		req.Level = 4
	}
	node := &model.BidGenOutline{
		ProjectID: req.ProjectID,
		ParentID:  req.ParentID,
		Level:     req.Level,
		Title:     strings.TrimSpace(req.Title),
		GenStatus: OutlineGenPending,
		Source:    "user",
	}
	// 同级排序：插入到 sortAfter 之后或末尾
	siblings, _ := s.repo.GetOutlineByProjectID(c, req.ProjectID)
	var sameLevel []*model.BidGenOutline
	for _, n := range siblings {
		if n.ParentID == req.ParentID {
			sameLevel = append(sameLevel, n)
		}
	}
	if req.SortAfter > 0 {
		maxOrder := int32(0)
		for _, n := range sameLevel {
			if n.ID == req.SortAfter {
				node.SortOrder = n.SortOrder + 100
				if n.SortOrder+100 > maxOrder {
					maxOrder = n.SortOrder + 100
				}
			}
		}
		// 后续节点顺移
		for _, n := range sameLevel {
			if n.SortOrder >= node.SortOrder {
				_ = s.repo.UpdateOutlineNode(c, n.ID, map[string]interface{}{"sort_order": n.SortOrder + 200})
			}
		}
	} else {
		maxOrder := int32(0)
		for _, n := range sameLevel {
			if n.SortOrder > maxOrder {
				maxOrder = n.SortOrder
			}
		}
		node.SortOrder = maxOrder + 100
	}
	if err := s.repo.AddOutlineNode(c, node); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "添加失败: " + err.Error()})
		return
	}
	// 同步文档标题
	s.insertHeadingToDoc(c, req.ProjectID, node)
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": node.ID}})
}

func (s *svcImpl) UpdateOutlineNode(c *gin.Context) {
	var req entity.BidGenUpdateOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "id 不能为空"})
		return
	}
	node, err := s.repo.GetOutlineByID(c, req.ID)
	if err != nil || node == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), node.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，大纲暂不可修改"})
		return
	}
	fields := map[string]interface{}{}
	if strings.TrimSpace(req.Title) != "" {
		fields["title"] = strings.TrimSpace(req.Title)
	}
	if len(fields) == 0 {
		c.JSON(200, gin.H{"code": 200, "msg": "无变更"})
		return
	}
	if err := s.repo.UpdateOutlineNode(c, req.ID, fields); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}
	node.Title = strings.TrimSpace(req.Title)
	s.updateHeadingInDoc(c, node.ProjectID, node)
	c.JSON(200, gin.H{"code": 200, "msg": "更新成功"})
}

// CompleteOutlineNode 人工标记章节写作完成/取消完成。
// 人工撰写的章节不走 AI 生成流程，gen_status 会一直停留在 pending，
// 导致项目整体状态无法进入 succeeded；此接口把该章节直接置为 succeeded，
// 并按“全部章节完成”重算项目整体状态与进度。取消完成则回退为 pending。
func (s *svcImpl) CompleteOutlineNode(c *gin.Context) {
	var req entity.BidGenCompleteOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 || req.ID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	completed := true
	if req.Completed != nil {
		completed = *req.Completed
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	switch proj.Status {
	case ProjectStatusGenerating:
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，章节状态暂不可修改"})
		return
	case ProjectStatusParsing, ProjectStatusOutlineReview:
		c.JSON(400, gin.H{"code": 400, "msg": "当前状态不可标记章节完成"})
		return
	}
	node, err := s.repo.GetOutlineByID(c, req.ID)
	if err != nil || node == nil || node.ProjectID != req.ProjectID {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	if biddoc.IsDocumentRoot(node) {
		c.JSON(400, gin.H{"code": 400, "msg": "目录根节点无需标记完成"})
		return
	}
	genStatus := OutlineGenPending
	if completed {
		genStatus = OutlineGenSucceeded
	}
	// 重算整体完成度：全部章节完成 → succeeded；取消完成 → 从 succeeded 回退 draft
	// 先按“目标状态生效后”的口径计算，避免读到写入前的中间态；
	// 父章节的状态变更连带其全部子章节（章节写作单元 = 整棵子树）
	nodes, err := s.repo.GetOutlineByProjectID(c, req.ProjectID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "大纲查询失败: " + err.Error()})
		return
	}
	subtreeIDs := collectSubtreeIDs(nodes, node.ID)
	targets := make(map[int64]struct{}, len(subtreeIDs))
	for _, id := range subtreeIDs {
		targets[id] = struct{}{}
	}
	for _, n := range nodes {
		if _, ok := targets[n.ID]; !ok {
			continue
		}
		// 正在生成的章节由后台任务写入终态，人工改动会被覆盖，直接拒绝
		if n.GenStatus == OutlineGenGenerating {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该章节（含子章节）正在生成，请稍后再试"})
			return
		}
		n.GenStatus = genStatus
	}
	completedCount, total, pct := generationCompletion(nodes)
	projectStatus := proj.Status
	projectFields := map[string]interface{}{"progress": pct}
	if total > 0 && completedCount == total {
		projectStatus = ProjectStatusSucceeded
		projectFields["status"] = projectStatus
		projectFields["last_error"] = ""
	} else if proj.Status == ProjectStatusSucceeded {
		projectStatus = ProjectStatusDraft
		projectFields["status"] = projectStatus
	}
	// 章节（含子章节）状态与项目完成度在同一事务内落库，防止只写成功一半
	if err := s.repo.SetOutlineSubtreeCompleted(c, req.ProjectID, subtreeIDs, genStatus, projectFields); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
			return
		}
		s.logger.Errorw("标记章节完成失败", "project_id", req.ProjectID, "outline_id", node.ID, "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "更新成功", "data": gin.H{
		"id":             node.ID,
		"genStatus":      genStatus,
		"outlineIds":     subtreeIDs,
		"status":         projectStatus,
		"completedCount": completedCount,
		"totalCount":     total,
		"progress":       pct,
	}})
}

func (s *svcImpl) DeleteOutlineNode(c *gin.Context) {
	var req entity.BidGenDeleteOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	node, err := s.repo.GetOutlineByID(c, req.ID)
	if err != nil || node == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	if node.ProjectID != req.ProjectID {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "大纲节点不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，大纲暂不可修改"})
		return
	}
	deletedIDs, err := s.repo.DeleteOutlineSubtree(c, req.ProjectID, req.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	for _, id := range deletedIDs {
		s.deleteChapterRangeInDoc(c, req.ProjectID, id)
	}
	c.JSON(200, gin.H{"code": 200, "msg": "删除成功"})
}

// ApplyOutline 大纲结构快照：拖拽排序/跨级移动后提交本项目全部大纲节点的权威结构（单事务）。
// 仅更新 parent_id/level/sort_order，不做删除/重命名语义（分别走 delete / update 接口）。
func (s *svcImpl) ApplyOutline(c *gin.Context) {
	var req entity.BidGenApplyOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 || len(req.Nodes) == 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，大纲暂不可修改"})
		return
	}
	if err := s.repo.ApplyOutlineStructure(c, req.ProjectID, req.Nodes); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "大纲结构更新失败: " + err.Error()})
		return
	}
	nodes, err := s.repo.GetOutlineByProjectID(c, req.ProjectID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "大纲查询失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"outline": buildOutlineResp(nodes)}})
}

// SyncOutline 编辑器文档标题结构 → 大纲表全量对账（导航/编辑器双向同步的"编辑器→导航"方向）。
// 请求按文档顺序的标题列表，后端推导树结构并新建/更新/删除大纲节点，返回新节点 id 映射与权威大纲。
func (s *svcImpl) SyncOutline(c *gin.Context) {
	var req entity.BidGenSyncOutlineReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "生成中，大纲暂不可修改"})
		return
	}
	items := deriveOutlineTree(req.Headings)
	mapping, err := s.repo.ReconcileOutline(c, req.ProjectID, items)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "大纲同步失败: " + err.Error()})
		return
	}
	nodes, err := s.repo.GetOutlineByProjectID(c, req.ProjectID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "大纲同步失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{
		"mapping": mapping,
		"outline": buildOutlineResp(nodes),
	}})
}

// ================================================================
// 导出记录
// ================================================================

// RecordExport 记录导出历史（DOCX 前端生成后回调）
func (s *svcImpl) RecordExport(c *gin.Context) {
	var req entity.BidGenExportRecordReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	if req.ExportType != "docx" && req.ExportType != "pdf" {
		c.JSON(400, gin.H{"code": 400, "msg": "exportType 仅支持 docx/pdf"})
		return
	}
	if _, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	r := &model.BidGenExportRecord{
		ProjectID:  req.ProjectID,
		ExportType: req.ExportType,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Status:     "succeeded",
		UserID:     entity.GetUserIDFromCtx(c),
	}
	if err := s.repo.AddExportRecord(c, r); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "记录失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "记录成功"})
}

// ================================================================
// 内部工具
// ================================================================

func isSupportedDoc(ext string) bool {
	switch ext {
	case ".pdf", ".doc", ".docx", ".txt", ".md":
		return true
	}
	return false
}

func isSupportedTemplate(ext string) bool {
	return ext == ".pdf" || ext == ".doc" || ext == ".docx"
}

// initGenStageStatus 初始化解析阶段状态并写入
func (s *svcImpl) initGenStageStatus(c *gin.Context, projectID int64, stages []string) error {
	m := make(map[string]string, len(stages))
	for _, st := range stages {
		m[st] = parseStagePending
	}
	b, _ := json.Marshal(m)
	return s.repo.UpdateProjectFields(c, projectID, map[string]interface{}{
		"stage_status": string(b),
		"stage":        stages[0],
	})
}

func parseStageStatusJSON(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]string{}
	}
	return m
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func buildOutlineResp(nodes []*model.BidGenOutline) []entity.BidGenOutlineResp {
	resp := make([]entity.BidGenOutlineResp, 0, len(nodes))
	for _, n := range nodes {
		resp = append(resp, entity.BidGenOutlineResp{
			ID:             n.ID,
			ProjectID:      n.ProjectID,
			ParentID:       n.ParentID,
			Level:          n.Level,
			SortOrder:      n.SortOrder,
			Title:          n.Title,
			ClauseIds:      n.ClauseIds,
			MaterialIds:    n.MaterialIds,
			GenStatus:      n.GenStatus,
			Source:         n.Source,
			IsRequiredFile: n.IsRequiredFile,
			IsAiSuggested:  n.IsAiSuggested,
		})
	}
	return resp
}

// replaceHostDocker 容器环境 URL 替换（AGENTS.md 踩坑记录）
func replaceHostDocker(u string) string {
	return strings.ReplaceAll(u, "host.docker.internal", "localhost")
}

// enqueueParseTask 入队 bid_gen_parse 任务
func (s *svcImpl) enqueueParseTask(ctx context.Context, genProjectID, analysisProjectID int64, userID int64, parseType string) error {
	payload := ParseTaskPayload{
		GenProjectID:      genProjectID,
		AnalysisProjectID: analysisProjectID,
		UserID:            userID,
		ParseType:         parseType,
	}
	_, err := s.taskqueueRepo.Enqueue(ctx, "bid_gen_parse", payload, taskqueue.EnqueueOpts{
		UserID:    userID,
		ProjectID: genProjectID,
	})
	return err
}
