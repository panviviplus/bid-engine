package bidreview

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
)

var validReviewStatus = map[string]bool{
	CheckPending: true, CheckConfirmed: true, CheckRejected: true,
}

var validDimensions = map[string]bool{
	DimensionCompliance: true, DimensionCompleteness: true,
	DimensionCompetitiveness: true, DimensionFormat: true,
}

var validRemediationStatus = map[string]bool{
	RemediationTodo: true, RemediationDoing: true, RemediationDone: true, RemediationIgnored: true,
}

// ================================================================
// 清单项
// ================================================================

// AddChecklistItem 用户自定义检查项（人工项，不触发 AI 判定）
func (s *svcImpl) AddChecklistItem(c *gin.Context) {
	var req entity.BidReviewItemAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.GetProjectForUser(ctx, userID, req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	dimension := req.Dimension
	if !validDimensions[dimension] {
		dimension = DimensionCompliance
	}
	items, _ := s.repo.GetChecklistItems(ctx, req.ProjectID)
	item := &model.BidReviewV2ChecklistItem{
		ProjectID: req.ProjectID,
		ItemKey:   "user:" + shortHash(fmt.Sprintf("%d-%s", req.ProjectID, req.Title)),
		Dimension: dimension, Category: firstNonEmpty(req.Category, "自定义"),
		Title: req.Title, Requirement: req.Requirement, ExpectedEvidence: req.ExpectedEvidence,
		Severity: firstNonEmpty(normalizeSeverity(req.Severity, "medium"), "medium"),
		Source:   SourceUser, ReviewStatus: CheckPending, SortOrder: int32(len(items) + 1),
		OriginJSON: "{}", IsUserEdited: true,
	}
	if err := s.repo.BatchCreateChecklistItems(ctx, []*model.BidReviewV2ChecklistItem{item}); err != nil {
		s.logger.Errorw("添加自定义检查项失败", "err", err)
		c.JSON(500, gin.H{"code": 500, "msg": "添加失败: " + err.Error()})
		return
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: req.ProjectID, ChecklistItemID: item.ID, Action: "add_item", OperatorID: userID,
		Detail: "新增自定义检查项“" + req.Title + "”",
	})
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": item.ID}})
}

// UpdateChecklistItem 人工确认/驳回 + 备注
func (s *svcImpl) UpdateChecklistItem(c *gin.Context) {
	var req entity.BidReviewItemUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	if !validReviewStatus[req.ReviewStatus] {
		c.JSON(400, gin.H{"code": 400, "msg": "review_status 不合法"})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	item, err := s.repo.GetChecklistItemByID(ctx, req.ItemID)
	if err != nil || item == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	if _, err := s.repo.GetProjectForUser(ctx, userID, item.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	now := time.Now()
	if err := s.repo.UpdateChecklistItem(ctx, req.ItemID, map[string]interface{}{
		"review_status": req.ReviewStatus,
		"review_note":   req.Note,
		"reviewed_by":   userID,
		"reviewed_at":   now,
	}); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}
	// 人工确认“已确认”时，同步关闭对应整改项
	if req.ReviewStatus == CheckConfirmed {
		if remediation := s.findRemediationByItem(ctx, item.ProjectID, req.ItemID); remediation != nil {
			_ = s.repo.UpdateRemediation(ctx, remediation.ID, map[string]interface{}{
				"status": RemediationDone, "resolved_at": now,
				"note": firstNonEmpty(req.Note, remediation.Note),
			})
		}
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: item.ProjectID, ChecklistItemID: req.ItemID, Action: req.ReviewStatus, OperatorID: userID,
		Detail: fmt.Sprintf("检查项“%s”标记为 %s", item.Title, req.ReviewStatus),
	})
	_ = s.refreshProjectStats(ctx, item.ProjectID)
	c.JSON(200, gin.H{"code": 200, "msg": "更新成功"})
}

// RecheckChecklistItem 复检（单项或按维度整批）
func (s *svcImpl) RecheckChecklistItem(c *gin.Context) {
	var req entity.BidReviewItemRecheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	if err := s.checkLLM(c, userID); err != nil {
		return
	}
	proj, err := s.repo.GetProjectForUser(ctx, userID, req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	items, err := s.repo.GetChecklistItems(ctx, req.ProjectID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询检查项失败"})
		return
	}
	targets := make([]*model.BidReviewV2ChecklistItem, 0, len(items))
	for _, it := range items {
		if req.ItemID > 0 && it.ID == req.ItemID {
			targets = append(targets, it)
			break
		}
		if req.ItemID == 0 && req.Dimension != "" && it.Dimension == req.Dimension {
			targets = append(targets, it)
		}
	}
	if len(targets) == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "未找到需要复检的检查项"})
		return
	}
	files, _ := s.repo.GetFilesByProjectAndType(ctx, req.ProjectID, "bid")
	fileMap := make(map[int64]*model.BidReviewV2File, len(files))
	for _, f := range files {
		fileMap[f.ID] = f
	}
	// 整批复检可能是几十项，逐条串行会让请求挂几分钟；
	// 这里保持与判定阶段一致的有限并发（LLM 并发由模块信号量兜住）。
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed int
	)
	for _, item := range targets {
		if item.Dimension == DimensionFormat {
			if _, err := s.runFormatScanStage(ctx, proj); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
			}
			continue
		}
		wg.Add(1)
		go func(it *model.BidReviewV2ChecklistItem) {
			defer wg.Done()
			if err := s.verdictOneItem(ctx, proj, it, fileMap); err != nil {
				s.logger.Warnw("复检失败", "item_id", it.ID, "err", err)
				mu.Lock()
				failed++
				mu.Unlock()
			}
		}(item)
	}
	wg.Wait()
	_ = s.refreshProjectStats(ctx, req.ProjectID)
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: req.ProjectID, ChecklistItemID: req.ItemID, Action: "recheck", OperatorID: userID,
		Detail: fmt.Sprintf("复检 %d 个检查项，失败 %d 个", len(targets), failed),
	})
	if failed > 0 && failed == len(targets) {
		c.JSON(500, gin.H{"code": 500, "msg": "复检失败，请稍后重试"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"checked": len(targets), "failed": failed}})
}

// DeleteChecklistItem 删除清单项（仅人工/用户自定义项）
func (s *svcImpl) DeleteChecklistItem(c *gin.Context) {
	var req entity.BidReviewItemDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	item, err := s.repo.GetChecklistItemByID(ctx, req.ItemID)
	if err != nil || item == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	if _, err := s.repo.GetProjectForUser(ctx, userID, item.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	if item.Source != SourceUser {
		c.JSON(400, gin.H{"code": 400, "msg": "仅自定义检查项可删除"})
		return
	}
	if err := s.repo.DeleteChecklistItem(ctx, req.ItemID); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: item.ProjectID, ChecklistItemID: req.ItemID, Action: "delete_item", OperatorID: userID,
		Detail: "删除自定义检查项“" + item.Title + "”",
	})
	_ = s.refreshProjectStats(ctx, item.ProjectID)
	c.JSON(200, gin.H{"code": 200, "msg": "删除成功"})
}

// ================================================================
// 整改闭环
// ================================================================

// UpdateRemediation 更新整改状态/责任人/备注
func (s *svcImpl) UpdateRemediation(c *gin.Context) {
	var req entity.BidReviewRemediationUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	if req.Status != "" && !validRemediationStatus[req.Status] {
		c.JSON(400, gin.H{"code": 400, "msg": "status 不合法"})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	remediation, err := s.repo.GetRemediationByID(ctx, req.ID)
	if err != nil || remediation == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "整改项不存在"})
		return
	}
	if _, err := s.repo.GetProjectForUser(ctx, userID, remediation.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "整改项不存在"})
		return
	}
	fields := map[string]interface{}{}
	if req.Status != "" {
		fields["status"] = req.Status
		if req.Status == RemediationDone {
			fields["resolved_at"] = time.Now()
		} else {
			fields["resolved_at"] = nil
		}
	}
	if req.Note != "" {
		fields["note"] = req.Note
	}
	if req.OwnerUserID > 0 {
		fields["owner_user_id"] = req.OwnerUserID
	}
	if err := s.repo.UpdateRemediation(ctx, req.ID, fields); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "更新失败: " + err.Error()})
		return
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: remediation.ProjectID, ChecklistItemID: remediation.ChecklistItemID,
		Action: "remediation", OperatorID: userID,
		Detail: fmt.Sprintf("整改项“%s”更新为 %s", remediation.Title, firstNonEmpty(req.Status, "备注更新")),
	})
	_ = s.refreshProjectStats(ctx, remediation.ProjectID)
	c.JSON(200, gin.H{"code": 200, "msg": "已更新"})
}

func (s *svcImpl) findRemediationByItem(ctx context.Context, projectID, itemID int64) *model.BidReviewV2Remediation {
	rows, err := s.repo.GetRemediations(ctx, projectID)
	if err != nil {
		return nil
	}
	for _, r := range rows {
		if r.ChecklistItemID == itemID {
			return r
		}
	}
	return nil
}

// ================================================================
// 企业规则库
// ================================================================

func (s *svcImpl) ListRules(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	dimension := strings.TrimSpace(c.Query("dimension"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	enabledOnly := c.Query("enabled_only") == "true" || c.Query("enabled_only") == "1"
	rows, err := s.repo.ListRules(c.Request.Context(), userID, dimension, enabledOnly, keyword)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询规则失败: " + err.Error()})
		return
	}
	out := make([]ruleDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, ruleDTO{
			ID: r.ID, Dimension: r.Dimension, Category: r.Category, Title: r.Title,
			Requirement: r.Requirement, ExpectedEvidence: r.ExpectedEvidence, Severity: r.Severity,
			AppliesWhen: r.AppliesWhen, Enabled: r.Enabled, HitCount: r.HitCount,
			Version: r.Version, UpdatedAt: r.UpdatedAt,
		})
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"list": out, "total": len(out)}})
}

func (s *svcImpl) SaveRule(c *gin.Context) {
	var req entity.BidReviewRuleSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	dimension := req.Dimension
	if !validDimensions[dimension] {
		dimension = DimensionCompliance
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	appliesWhen := strings.TrimSpace(req.AppliesWhen)
	if appliesWhen == "" {
		appliesWhen = "{}"
	}
	if req.ID > 0 {
		if err := s.repo.UpdateRule(ctx, userID, req.ID, map[string]interface{}{
			"dimension": dimension, "category": req.Category, "title": req.Title,
			"requirement": req.Requirement, "expected_evidence": req.ExpectedEvidence,
			"severity": normalizeSeverity(req.Severity, "medium"), "applies_when": appliesWhen,
			"enabled": enabled, "version": gormExprIncr("version", 1),
		}); err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "保存失败: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": req.ID}})
		return
	}
	rule := &model.BidReviewV2Rule{
		UserID: userID, Dimension: dimension, Category: req.Category, Title: req.Title,
		Requirement: req.Requirement, ExpectedEvidence: req.ExpectedEvidence,
		Severity: normalizeSeverity(req.Severity, "medium"), AppliesWhen: appliesWhen,
		Source: "user", Enabled: enabled, Version: 1,
	}
	if teamID, ok := c.Get("team_id"); ok {
		if v, ok2 := teamID.(int32); ok2 {
			rule.UserTeamID = v
		}
	}
	if companyID, ok := c.Get("company_id"); ok {
		if v, ok2 := companyID.(int32); ok2 {
			rule.UserCompanyID = v
		}
	}
	if err := s.repo.AddRule(ctx, rule); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "保存失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": rule.ID}})
}

func (s *svcImpl) DeleteRule(c *gin.Context) {
	var req entity.BidReviewRuleDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	if err := s.repo.DeleteRule(c.Request.Context(), entity.GetUserIDFromCtx(c), req.ID); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "删除失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "删除成功"})
}

// SaveRuleFromItem 把清单项沉淀为企业审核规则
func (s *svcImpl) SaveRuleFromItem(c *gin.Context) {
	var req entity.BidReviewRuleFromItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := entity.GetUserIDFromCtx(c)
	item, err := s.repo.GetChecklistItemByID(ctx, req.ItemID)
	if err != nil || item == nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	if _, err := s.repo.GetProjectForUser(ctx, userID, item.ProjectID); err != nil {
		c.JSON(404, gin.H{"code": 404, "msg": "检查项不存在"})
		return
	}
	rule := &model.BidReviewV2Rule{
		UserID: userID, Dimension: item.Dimension, Category: item.Category, Title: item.Title,
		Requirement: item.Requirement, ExpectedEvidence: item.ExpectedEvidence,
		Severity: item.Severity, AppliesWhen: "{}", Source: "user", Enabled: true, Version: 1,
	}
	if err := s.repo.AddRule(ctx, rule); err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "沉淀失败: " + err.Error()})
		return
	}
	_ = s.repo.AddOpLog(ctx, &model.BidReviewV2OpLog{
		ProjectID: item.ProjectID, ChecklistItemID: item.ID, Action: "rule_from_item", OperatorID: userID,
		Detail: "检查项沉淀为审核规则“" + item.Title + "”",
	})
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"id": rule.ID}})
}

// ================================================================
// 统计刷新
// ================================================================

// refreshProjectStats 重算项目结论快照
func (s *svcImpl) refreshProjectStats(ctx context.Context, projectID int64) error {
	stats, scoreTotal, scoreMax, err := s.buildScorecard(ctx, projectID)
	if err != nil {
		return err
	}
	total, passed, warning, errCount := int64(0), int64(0), int64(0), int64(0)
	for _, st := range stats {
		total += int64(st.Total)
		passed += int64(st.Passed)
		warning += int64(st.Warning)
		errCount += int64(st.Error + st.NotFound)
	}
	todo, _ := s.repo.CountTodoRemediations(ctx, projectID)
	return s.repo.UpdateProjectFields(ctx, projectID, map[string]interface{}{
		"total_items": total, "passed_items": passed, "warning_items": warning,
		"error_items": errCount, "todo_items": todo,
		"scoring_total": scoreTotal, "scoring_max": scoreMax,
	})
}
