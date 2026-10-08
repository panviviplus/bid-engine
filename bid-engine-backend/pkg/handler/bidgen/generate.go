package bidgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	bidgenRepo "bid-engine/pkg/repo/bidgen"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
	"bid-engine/pkg/service/biddoc"
)

const (
	llmFeatureChapterWrite = "bid_gen_chapter_write"
	genCancelKeyPrefix     = "cancel_gen:"

	// 章节生成模式：write-撰写 / rewrite-重写 / expand-扩写 / condense-缩写
	GenModeWrite    = "write"
	GenModeRewrite  = "rewrite"
	GenModeExpand   = "expand"
	GenModeCondense = "condense"
)

// ===== SSE 事件数据 =====

type sseChapterStart struct {
	OutlineID int64  `json:"outlineId"`
	Title     string `json:"title"`
}

type sseDelta struct {
	OutlineID int64  `json:"outlineId"`
	Text      string `json:"text"`
}

type sseChapterDone struct {
	OutlineID int64           `json:"outlineId"`
	JSON      json.RawMessage `json:"json"`
}

type sseChapterError struct {
	OutlineID int64  `json:"outlineId"`
	Msg       string `json:"msg"`
}

type sseProgress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
	Pct     int `json:"pct"`
}

type sseError struct {
	Msg  string `json:"msg"`
	Code int32  `json:"code,omitempty"`
}

// GenerateFull 创建整篇后台生成任务。
func (s *svcImpl) GenerateFull(c *gin.Context) {
	var req entity.BidGenGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	req.OutlineIDs = nil // 整篇
	s.runGenerate(c, &req)
}

// GenerateChapter 创建指定章节后台生成任务。
func (s *svcImpl) GenerateChapter(c *gin.Context) {
	var req entity.BidGenGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	if len(req.OutlineIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请至少选择一个章节"})
		return
	}
	s.runGenerate(c, &req)
}

// runGenerate 校验请求并创建可恢复后台任务；LLM 调用由 BidGenGenerateTaskHandler 执行。
func (s *svcImpl) runGenerate(c *gin.Context, req *entity.BidGenGenerateReq) {
	userID := entity.GetUserIDFromCtx(c)
	length := req.Length
	if length == "" {
		length = LengthStandard
	}
	if length != LengthConcise && length != LengthStandard && length != LengthDetailed {
		length = LengthStandard
	}
	mode := req.Mode
	if mode == "" {
		mode = GenModeWrite
	}
	switch mode {
	case GenModeWrite, GenModeRewrite, GenModeExpand, GenModeCondense:
	default:
		mode = GenModeWrite
	}
	// 重写/扩写/缩写仅支持单个章节
	if mode != GenModeWrite && len(req.OutlineIDs) != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "重写/扩写/缩写仅支持单个章节"})
		return
	}
	if len([]rune(req.Instruction)) > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "补充要求不能超过 5000 字"})
		return
	}

	// 1. LLM 配置检查
	if err := s.llm.LlmConfigAvailable(repoLLM.WithUserID(c.Request.Context(), userID), repoLLM.ModuleBidGeneration); err != nil {
		// 只透传分类后的友好文案与错误码，原始错误细节进服务端日志
		s.logger.Warnw("标书生成前置 LLM 配置检查失败", "userID", userID, "err", err)
		code, msg := repoLLM.LLMErrorMeta(err)
		c.JSON(http.StatusBadRequest, gin.H{"code": code, "msg": msg})
		return
	}

	// 2. 加载项目
	proj, err := s.repo.GetProjectForUser(c, userID, req.ProjectID)
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	if proj.Status == ProjectStatusGenerating {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "项目正在生成中，请勿重复操作"})
		return
	}
	if proj.Status == ProjectStatusParsing {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "项目仍在解析中，请稍后再试"})
		return
	}
	if proj.Status == ProjectStatusOutlineReview {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "请先确认大纲，再生成正文"})
		return
	}

	// 3. 目标章节列表与正文结构在入队前固化，保证后台任务输入稳定。
	outlineNodes, err := s.repo.GetOutlineByProjectID(c, proj.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加载大纲失败，请重试"})
		return
	}
	orderedOutline, err := biddoc.OrderOutlineTree(outlineNodes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "大纲结构异常: " + err.Error()})
		return
	}
	if _, err := s.ensureDocumentAnchors(c, proj.ID, orderedOutline); err != nil {
		s.logger.Warnw("生成前正文结构校验失败", "project_id", proj.ID, "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "正文结构异常，请重新加载并确认大纲"})
		return
	}
	// 章节级写作（write）：选中父章节时连同其子章节一起写
	targets := s.selectTargets(orderedOutline, req.OutlineIDs, mode == GenModeWrite)
	if len(targets) == 0 {
		if len(req.OutlineIDs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "目录根节点无需生成正文"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "没有待生成的章节（已完成章节不会重复生成）"})
		}
		return
	}
	// 重写/扩写/缩写依赖现有正文：目标章节无正文时拒绝
	if mode != GenModeWrite {
		for _, t := range targets {
			cc, err := s.repo.GetChapterContent(c, proj.ID, t.ID)
			if err != nil || cc == nil || strings.TrimSpace(cc.ContentJSON) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该章节暂无正文内容，请先使用“生成本章”"})
				return
			}
		}
	}

	// 4. 数据库事务创建业务任务并锁定项目。
	task := &model.BidGenTask{
		ProjectID:   proj.ID,
		TaskType:    TaskTypeFull,
		OutlineIds:  marshalIDs(targetIDs(targets)),
		Status:      "pending",
		TotalCount:  int32(len(targets)),
		UserID:      userID,
		LengthTier:  length,
		GenMode:     mode,
		Instruction: strings.TrimSpace(req.Instruction),
	}
	if len(req.OutlineIDs) > 0 {
		task.TaskType = TaskTypeChapter
	}
	if err := s.repo.CreateGenerationTask(c, userID, task); err != nil {
		if errors.Is(err, bidgenRepo.ErrGenerationActive) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该项目已有生成任务正在执行"})
			return
		}
		if errors.Is(err, bidgenRepo.ErrTaskNotRunnable) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "当前项目状态不可生成正文"})
			return
		}
		s.logger.Errorw("创建标书生成任务失败", "project_id", proj.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建生成任务失败，请重试"})
		return
	}

	// 5. 使用确定性队列ID原子入队；对账器可安全重试该步骤。
	queueID := fmt.Sprintf("bid_gen_generate_%d", task.ID)
	if err := s.repo.UpdateTaskFields(c, task.ID, map[string]interface{}{"queue_task_id": queueID}); err != nil {
		s.failGenerationTask(context.Background(), task, "绑定生成队列任务失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "启动生成任务失败，请重试"})
		return
	}
	task.QueueTaskID = &queueID
	_, err = s.taskqueueRepo.Enqueue(c, "bid_gen_generate", GenerateTaskPayload{BidTaskID: task.ID}, taskqueue.EnqueueOpts{
		TaskID: queueID, ProjectID: task.ProjectID, UserID: task.UserID,
	})
	if err != nil {
		s.logger.Errorw("标书生成任务入队失败", "project_id", task.ProjectID, "task_id", task.ID, "err", err)
		s.failGenerationTask(context.Background(), task, "生成任务入队失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成任务入队失败，请重试"})
		return
	}
	_, _ = s.publishGenerationEvent(c, task.ID, "task_state", taskResponse(task))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": taskResponse(task)})
}

// selectTargets 选择待生成章节：整篇= pending/failed；指定=按传入 ID（无论状态，支持强制重生成）。
// includeDescendants=true 时，指定父章节连带其子章节：章节级写作的单元是一棵子树，
// 避免只写父章节、留下未写的子章节。连带范围按父章节自身状态区分：
//   - 父章节已完成（菜单为“重新生成”）→ 整棵子树强制重写；
//   - 父章节未完成（菜单为“生成本章”）→ 只补齐未完成的子章节，保留已完成内容。
func (s *svcImpl) selectTargets(nodes []*model.BidGenOutline, ids []int64, includeDescendants bool) []*model.BidGenOutline {
	selected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	if len(ids) == 0 {
		var targets []*model.BidGenOutline
		for _, n := range nodes {
			if biddoc.IsDocumentRoot(n) {
				continue
			}
			if n.GenStatus == OutlineGenPending || n.GenStatus == OutlineGenFailed {
				targets = append(targets, n)
			}
		}
		return targets
	}
	// 重新生成（选中节点已完成）→ 子树整体重写；否则只补齐未完成节点
	forceSubtree := false
	if includeDescendants {
		for _, n := range nodes {
			if _, ok := selected[n.ID]; ok && n.GenStatus == OutlineGenSucceeded {
				forceSubtree = true
				break
			}
		}
	}
	var targets []*model.BidGenOutline
	// inBranch：已进入选中子树（含被跳过的已完成节点，保证能继续向下遍历）
	inBranch := make(map[int64]struct{}, len(ids))
	for _, n := range nodes {
		if biddoc.IsDocumentRoot(n) {
			continue
		}
		_, picked := selected[n.ID]
		_, inSelectedBranch := inBranch[n.ParentID]
		if !picked && includeDescendants && inSelectedBranch {
			// 树序保证父节点先于子节点：父节点命中即整棵子树进入范围
			picked = forceSubtree || n.GenStatus != OutlineGenSucceeded
		}
		if picked {
			targets = append(targets, n)
		}
		if picked || inSelectedBranch {
			inBranch[n.ID] = struct{}{}
		}
	}
	return targets
}

// collectSubtreeIDs 返回 rootID 自身及其全部后代节点 ID（输入为树序节点，父先于子）。
func collectSubtreeIDs(nodes []*model.BidGenOutline, rootID int64) []int64 {
	inSubtree := map[int64]struct{}{rootID: {}}
	ids := make([]int64, 0, 4)
	for _, n := range nodes {
		if _, ok := inSubtree[n.ID]; ok {
			ids = append(ids, n.ID)
			continue
		}
		if _, ok := inSubtree[n.ParentID]; ok {
			inSubtree[n.ID] = struct{}{}
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// generateChapter 生成单个章节（写作规格 → 证据包 → 流式撰写 → 图表装配 → 落库）。
// mode 支持 write/rewrite/expand/condense；rewrite/expand/condense 复用已有写作规格，
// 并把现有正文与现有图片一并保留。
func (s *svcImpl) generateChapter(ctx context.Context, proj *model.BidGenProject, node *model.BidGenOutline, ordered []*model.BidGenOutline, wordTargets map[int64]int, globalCtx, length, mode, instruction string, taskID int64, completed, total int) (json.RawMessage, error) {
	children := directChildTitles(ordered, node.ID)

	existingContent := ""
	existingImages := []pmNode{}
	if mode != GenModeWrite {
		if cc, err := s.repo.GetChapterContent(ctx, proj.ID, node.ID); err == nil && cc != nil && cc.ContentJSON != "" {
			existingContent = extractChapterBodyText(cc.ContentJSON)
			existingImages = extractImageNodes(cc.ContentJSON)
		}
	}

	// 1. 写作规格 + 证据包
	var spec *chapterSpec
	var evidence *chapterEvidence
	if mode == GenModeWrite {
		targetWords := wordTargets[node.ID]
		if targetWords <= 0 {
			targetWords = minChapterWords
		}
		// 先组装候选证据（含素材候选），再由规格决定引用哪些素材与评分项
		evidence = s.buildChapterEvidence(ctx, proj, node, ordered, nil)
		spec = s.generateChapterSpec(ctx, proj, node, children, targetWords, globalCtx, evidence)
		if spec != nil {
			evidence = s.buildChapterEvidence(ctx, proj, node, ordered, spec)
		}
	} else {
		spec = s.loadChapterSpec(ctx, proj.ID, node.ID)
		if spec != nil && spec.TargetWords <= 0 {
			spec.TargetWords = wordTargets[node.ID]
		}
		evidence = s.buildChapterEvidence(ctx, proj, node, ordered, spec)
	}

	system, prompt := s.buildChapterPrompt(chapterPromptInput{
		proj: proj, node: node, globalCtx: globalCtx, evidence: evidence, spec: spec,
		length: length, mode: mode, existing: existingContent, instruction: instruction, children: children,
	})
	temperature := float64Ptr(0.7)
	if mode == GenModeRewrite || mode == GenModeCondense {
		temperature = float64Ptr(0.4)
	}
	maxTokens := chapterMaxTokens(ctx, length, specTargetWordsForTokenBudget(spec))
	req := &repoLLM.ChatRequest{
		System:         system,
		Prompt:         prompt,
		Temperature:    temperature,
		MaxTokens:      &maxTokens,
		ResponseFormat: map[string]string{"type": "text"},
	}
	deltaC, errC := s.llm.ChatOnceStreamByFeature(ctx, llmFeatureChapterWrite, req)

	var buf strings.Builder
	var pending strings.Builder
	lastSend := time.Now()
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		delta := pending.String()
		pending.Reset()
		if err := s.publishGenerationDelta(ctx, taskID, node.ID, node.Title, delta, buf.String()); err != nil {
			s.logger.Warnw("发布章节生成增量失败", "task_id", taskID, "outline_id", node.ID, "err", err)
		}
		lastSend = time.Now()
	}

	streamErr := error(nil)
	for deltaC != nil || errC != nil {
		select {
		case d, ok := <-deltaC:
			if !ok {
				deltaC = nil
				continue
			}
			buf.WriteString(d.ContentDelta)
			pending.WriteString(d.ContentDelta)
			if time.Since(lastSend) >= 100*time.Millisecond {
				flush()
			}
		case err, ok := <-errC:
			if !ok {
				errC = nil
				continue
			}
			streamErr = err
			deltaC = nil
			errC = nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if streamErr != nil {
		return nil, fmt.Errorf("LLM 生成失败: %w", streamErr)
	}
	flush()

	content := buf.String()
	if strings.TrimSpace(content) == "" {
		return nil, permanentGeneration(fmt.Errorf("章节内容为空"))
	}

	// 2. 转换 + 图表装配
	chapterNo := chapterOrdinal(ordered, node)
	var chapterNodes []pmNode
	var figures []chapterFigure
	if mode == GenModeWrite {
		pendingRefs := []string{}
		if spec != nil {
			pendingRefs = spec.MaterialRefs
		}
		chapterNodes, figures = renderChapter(content, evidence, chapterNo, pendingRefs)
	} else {
		chapterNodes = textToPMJSON(content)
		// 重写/扩写/缩写不得丢失原有证明材料图片
		chapterNodes = append(chapterNodes, existingImages...)
	}
	if len(chapterNodes) == 0 {
		chapterNodes = []pmNode{paragraphNode()}
	}

	// 完整章节片段（标题 + 内容）
	segment := []pmNode{headingNode(int(node.Level), node.ID, node.Title)}
	segment = append(segment, chapterNodes...)
	segJSON, err := json.Marshal(segment)
	if err != nil {
		return nil, fmt.Errorf("序列化章节正文: %w", err)
	}

	// 落库：chapter_content + doc_content + outline/task 状态单事务提交。
	cc := &model.BidGenChapterContent{
		ProjectID:   proj.ID,
		OutlineID:   node.ID,
		ContentJSON: string(segJSON),
		WordCount:   int32(len([]rune(content))),
		Source:      "ai",
		GenTaskID:   taskID,
	}
	doc, err := s.repo.GetDocContent(ctx, proj.ID)
	if err != nil {
		return nil, fmt.Errorf("加载主文档失败: %w", err)
	}
	if doc == nil || strings.TrimSpace(doc.DocJSON) == "" {
		return nil, fmt.Errorf("主文档内容为空")
	}
	newDoc, err := mergeChapter(doc.DocJSON, segment)
	if err != nil {
		return nil, permanentGeneration(fmt.Errorf("正文结构异常，合并章节失败: %w", err))
	}
	if err := s.repo.SaveGeneratedChapter(ctx, cc, &model.BidGenDocContent{
		ProjectID: proj.ID,
		DocJSON:   newDoc,
		DocHTML:   doc.DocHTML,
	}, taskID, int32(completed), int32(progressPct(completed, total))); err != nil {
		return nil, err
	}

	// 3. 写作规格、素材引用与评分覆盖回写
	s.persistChapterEvidence(ctx, proj, node, spec, evidence, figures, content, taskID)
	return segJSON, nil
}

// persistChapterEvidence 落库写作规格、素材引用与评分项覆盖结果。
// 全部为“尽力而为”：失败只记日志，不影响已生成的正文。
func (s *svcImpl) persistChapterEvidence(ctx context.Context, proj *model.BidGenProject, node *model.BidGenOutline, spec *chapterSpec, evidence *chapterEvidence, figures []chapterFigure, content string, taskID int64) {
	if spec != nil {
		spec.Figures = figures
		var coverage []byte
		if len(spec.Scoring) > 0 {
			if encoded, err := json.Marshal(evaluateScoringCoverage(content, spec.Scoring)); err == nil {
				coverage = encoded
			}
		}
		if err := s.saveChapterSpec(ctx, proj, node.ID, taskID, spec, coverage); err != nil {
			s.logger.Warnw("保存章节写作规格失败", "project_id", proj.ID, "outline_id", node.ID, "err", err)
		}
	}

	// outline.material_ids 真正落库，供前端与后续重写复用
	if spec != nil && len(spec.MaterialIDs) > 0 {
		if encoded, err := json.Marshal(spec.MaterialIDs); err == nil {
			if err := s.repo.UpdateOutlineNode(ctx, node.ID, map[string]interface{}{"material_ids": string(encoded)}); err != nil {
				s.logger.Warnw("回写章节素材引用失败", "project_id", proj.ID, "outline_id", node.ID, "err", err)
			}
		}
	}

	// 素材引用记录：只记录“规格选定”或“实际插入图片”的素材，先清后写避免堆积
	if evidence != nil && len(evidence.Materials) > 0 {
		selected := make(map[int64]bool)
		if spec != nil {
			for _, ref := range spec.MaterialRefs {
				if card, ok := evidence.material(ref); ok {
					selected[card.ID] = true
				}
			}
		}
		for _, figure := range figures {
			selected[figure.MaterialID] = true
		}
		usedCards := make([]materialCard, 0, len(selected))
		for _, card := range evidence.Materials {
			if selected[card.ID] {
				usedCards = append(usedCards, card)
			}
		}
		refs := materialRefRecords(proj, node.ID, usedCards, figures)
		if len(refs) == 0 {
			return
		}
		db := s.repo.DB()
		if db != nil {
			if err := db.WithContext(ctx).Table(model.TableNameBidGenMaterialRef).
				Where("project_id = ? AND outline_id = ?", proj.ID, node.ID).
				Delete(&model.BidGenMaterialRef{}).Error; err != nil {
				s.logger.Warnw("清理旧素材引用失败", "project_id", proj.ID, "outline_id", node.ID, "err", err)
			}
		}
		if err := s.repo.BatchCreateMaterialRefs(ctx, refs); err != nil {
			s.logger.Warnw("写入素材引用失败", "project_id", proj.ID, "outline_id", node.ID, "err", err)
		}
	}
}

// directChildTitles 返回直接子章节标题（用于“只写概述”的判定与提示）。
func directChildTitles(ordered []*model.BidGenOutline, outlineID int64) []string {
	var titles []string
	for _, node := range ordered {
		if node.ParentID == outlineID {
			titles = append(titles, node.Title)
		}
	}
	return titles
}

// specTargetWordsForTokenBudget 规格为空（规格生成降级）时返回 0，由档位默认值兜底。
func specTargetWordsForTokenBudget(spec *chapterSpec) int {
	if spec == nil {
		return 0
	}
	return spec.TargetWords
}

// chapterOrdinal 返回章节所属一级章节的序号（1 起），用于“图 X-Y”“表 X-Y”编号。
func chapterOrdinal(ordered []*model.BidGenOutline, node *model.BidGenOutline) int {
	byID := make(map[int64]*model.BidGenOutline, len(ordered))
	for _, item := range ordered {
		byID[item.ID] = item
	}
	current := node
	for hops := 0; hops < 8 && current != nil && current.ParentID != 0; hops++ {
		parent := byID[current.ParentID]
		if parent == nil || biddoc.IsDocumentRoot(parent) {
			break
		}
		current = parent
	}
	if current == nil {
		return 1
	}
	ordinal := 0
	for _, item := range ordered {
		if biddoc.IsDocumentRoot(item) {
			continue
		}
		if item.Level != current.Level {
			continue
		}
		ordinal++
		if item.ID == current.ID {
			return ordinal
		}
	}
	return 1
}

// resetGeneratingOutlines 生成中断/取消时，将生成中的章节恢复为待生成
func (s *svcImpl) resetGeneratingOutlines(ctx context.Context, projectID int64) {
	nodes, err := s.repo.GetOutlineByProjectID(ctx, projectID)
	if err != nil {
		return
	}
	contents, _ := s.repo.GetChapterContentsByProjectID(ctx, projectID)
	contentByOutline := make(map[int64]bool, len(contents))
	for _, content := range contents {
		contentByOutline[content.OutlineID] = strings.TrimSpace(content.ContentJSON) != ""
	}
	for _, n := range nodes {
		if n.GenStatus == OutlineGenGenerating {
			status := OutlineGenPending
			if contentByOutline[n.ID] {
				status = OutlineGenSucceeded
			}
			_ = s.repo.UpdateOutlineNode(ctx, n.ID, map[string]interface{}{"gen_status": status})
		}
	}
}

// CancelGenerate 取消生成
func (s *svcImpl) CancelGenerate(c *gin.Context) {
	var req entity.BidGenCancelReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ProjectID <= 0 || req.TaskID <= 0 {
		c.JSON(400, gin.H{"code": 400, "msg": "projectId 和 taskId 不能为空"})
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	task, err := s.repo.GetTaskForUser(c, userID, req.TaskID)
	if err != nil || task == nil || task.ProjectID != req.ProjectID {
		c.JSON(404, gin.H{"code": 404, "msg": "生成任务不存在"})
		return
	}
	if isGenerationTerminal(task.Status) {
		c.JSON(200, gin.H{"code": 200, "msg": "当前无进行中的生成任务"})
		return
	}
	if task.Status == "cancelling" {
		c.JSON(200, gin.H{"code": 200, "msg": "任务正在停止"})
		return
	}
	now := time.Now()
	updated, err := s.repo.UpdateTaskFieldsIfStatus(c, task.ID, []string{"pending", "running"}, map[string]interface{}{
		"status": "cancelling", "cancel_requested_at": now,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "请求取消失败，请重试"})
		return
	}
	if !updated {
		c.JSON(200, gin.H{"code": 200, "msg": "任务已经结束"})
		return
	}
	if err := s.redisSvc.Client().Set(c.Request.Context(), generationCancelKey(task.ProjectID, task.ID), "1", 24*time.Hour).Err(); err != nil {
		_, _ = s.repo.UpdateTaskFieldsIfStatus(c, task.ID, []string{"cancelling"}, map[string]interface{}{
			"status": task.Status, "cancel_requested_at": nil,
		})
		c.JSON(500, gin.H{"code": 500, "msg": "请求取消失败，请重试"})
		return
	}
	_ = s.redisSvc.Client().Publish(c, generationCancelChannel(task.ID), "cancel").Err()
	_, _ = s.publishGenerationEvent(c, task.ID, "task_state", gin.H{"taskId": task.ID, "status": "cancelling"})
	queueWasPending := task.Status == "pending"
	if task.QueueTaskID != nil && *task.QueueTaskID != "" {
		if queueMeta, queueErr := s.taskqueueRepo.GetTask(c, *task.QueueTaskID); queueErr == nil && queueMeta != nil {
			queueWasPending = queueMeta.Status == taskqueue.TaskStatusPending
		}
	}
	if queueWasPending {
		if task.QueueTaskID != nil && *task.QueueTaskID != "" {
			_ = s.taskqueueRepo.CancelTask(c, *task.QueueTaskID)
		}
		s.finishCancelledGeneration(task.ProjectID, task.ID, int(task.CompletedCount))
		_, _ = s.publishGenerationEvent(c, task.ID, "cancelled", gin.H{"taskId": task.ID})
		c.JSON(200, gin.H{"code": 200, "msg": "已取消排队任务"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "已请求取消"})
}

// ===== 工具 =====

func isCancelRequested(ctx context.Context, s *svcImpl, projectID, taskID int64) bool {
	v, err := s.redisSvc.Client().Get(ctx, generationCancelKey(projectID, taskID)).Result()
	if err != nil {
		return false
	}
	return v == "1"
}

func generationCancelKey(projectID, taskID int64) string {
	return fmt.Sprintf("%s%d:%d", genCancelKeyPrefix, projectID, taskID)
}

func generationCancelChannel(taskID int64) string {
	return fmt.Sprintf("bidgen:cancel:%d", taskID)
}

func (s *svcImpl) deleteGenerationKey(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.redisSvc.Client().Del(ctx, key).Err(); err != nil {
		s.logger.Warnw("清理标书生成 Redis 键失败", "key", key, "err", err)
	}
}

func (s *svcImpl) finishCancelledGeneration(projectID, taskID int64, completed int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.resetGeneratingOutlines(ctx, projectID)
	nodes, _ := s.repo.GetOutlineByProjectID(ctx, projectID)
	overallCompleted, overallTotal, overallPct := generationCompletion(nodes)
	taskProgress := 0
	if task, err := s.repo.GetTaskByID(ctx, taskID); err == nil && task != nil {
		taskProgress = progressPct(completed, int(task.TotalCount))
	}
	projectStatus := ProjectStatusDraft
	if overallTotal > 0 && overallCompleted == overallTotal {
		projectStatus = ProjectStatusSucceeded
	}
	if err := s.repo.FinishGeneration(ctx, projectID, map[string]interface{}{
		"status": projectStatus, "stage": "", "progress": overallPct, "last_error": "",
	}, taskID, map[string]interface{}{
		"status": "cancelled", "error_msg": "用户取消", "completed_count": int32(completed),
		"progress": taskProgress, "heartbeat_at": time.Now(), "finished_at": time.Now(),
	}); err != nil {
		s.logger.Errorw("保存生成取消终态失败", "project_id", projectID, "task_id", taskID, "err", err)
	}
	s.clearGenerationPartial(ctx, taskID)
	s.deleteGenerationKey(generationCancelKey(projectID, taskID))
}

func (s *svcImpl) failGenerationTask(ctx context.Context, task *model.BidGenTask, msg string) {
	if task == nil {
		return
	}
	s.resetGeneratingOutlines(ctx, task.ProjectID)
	nodes, _ := s.repo.GetOutlineByProjectID(ctx, task.ProjectID)
	_, _, overallPct := generationCompletion(nodes)
	if err := s.repo.FinishGeneration(ctx, task.ProjectID, map[string]interface{}{
		"status": ProjectStatusFailed, "stage": "", "progress": overallPct, "last_error": msg,
	}, task.ID, map[string]interface{}{
		"status": "failed", "error_msg": msg, "finished_at": time.Now(),
	}); err != nil {
		s.logger.Errorw("保存生成失败终态失败", "project_id", task.ProjectID, "task_id", task.ID, "err", err)
	}
	s.clearGenerationPartial(ctx, task.ID)
}

func generationCompletion(nodes []*model.BidGenOutline) (completed, total, pct int) {
	for _, node := range nodes {
		if biddoc.IsDocumentRoot(node) {
			continue
		}
		total++
		if node.GenStatus == OutlineGenSucceeded {
			completed++
		}
	}
	return completed, total, progressPct(completed, total)
}

// chapterMaxTokens 单章输出上限：按篇幅档位取值，并保证至少能写出本章目标字数
// （中文约 2 token/字），再受模型上下文窗口约束。
func chapterMaxTokens(ctx context.Context, length string, targetWords int) int {
	limit := map[string]int{
		LengthConcise:  4096,
		LengthStandard: 8192,
		LengthDetailed: 16384,
	}[length]
	if limit == 0 {
		limit = 8192
	}
	if targetWords > 0 {
		if needed := targetWords * 2; needed > limit {
			limit = needed
		}
	}
	contextLimit := 0
	if cfg := repoLLM.ResolveConfig(ctx, llmFeatureChapterWrite); cfg != nil {
		if cfg.DefaultMaxTokens > 0 && cfg.DefaultMaxTokens < limit {
			// 仅在用户显式压低模型输出上限时收缩，且不低于目标字数所需预算
			floor := targetWords * 2
			if cfg.DefaultMaxTokens > floor {
				limit = cfg.DefaultMaxTokens
			}
		}
		contextLimit = cfg.ContextWindowTokens - 4096
	}
	if contextLimit > 0 && contextLimit < limit {
		limit = contextLimit
	}
	if limit < 512 {
		return 512
	}
	return limit
}

func progressPct(current, total int) int {
	if total <= 0 {
		return 0
	}
	p := current * 100 / total
	if p > 100 {
		return 100
	}
	return p
}

func marshalIDs(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

func targetIDs(nodes []*model.BidGenOutline) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	return ids
}

func taskResponse(task *model.BidGenTask) entity.BidGenTaskResp {
	if task == nil {
		return entity.BidGenTaskResp{}
	}
	return entity.BidGenTaskResp{
		ID: task.ID, ProjectID: task.ProjectID, TaskType: task.TaskType, Status: task.Status,
		Progress: task.Progress, CurrentOutlineID: task.CurrentOutlineID,
		CompletedCount: task.CompletedCount, TotalCount: task.TotalCount, ErrorMsg: task.ErrorMsg,
		LengthTier: task.LengthTier, GenMode: task.GenMode, AttemptCount: task.AttemptCount,
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
}

func sendSSEEvent(c *gin.Context, event string, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		b = []byte(`{}`)
	}
	_, _ = c.Writer.WriteString("event: " + event + "\n")
	_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
	c.Writer.Flush()
}

func sendSSEError(c *gin.Context, msg string) {
	setSSEHeaders(c)
	sendSSEEvent(c, "error", sseError{Msg: msg})
}

// sendSSEErrorWithCode 发送带 LLM 错误码的 SSE error 事件（code 供前端差异化提示）
func sendSSEErrorWithCode(c *gin.Context, code int32, msg string) {
	setSSEHeaders(c)
	sendSSEEvent(c, "error", sseError{Msg: msg, Code: code})
}

// newGinCtx 构造最小 gin.Context（供后台任务调用 material 等依赖 gin.Context 的仓库）
func newGinCtx(ctx context.Context, userID int64) *gin.Context {
	w := httptest.NewRecorder()
	ac, _ := gin.CreateTestContext(w)
	ac.Request, _ = http.NewRequestWithContext(ctx, "POST", "/internal/bidgen", nil)
	if userID > 0 {
		ac.Set("user_id", userID)
	}
	return ac
}
