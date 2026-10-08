package bidgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/repo/docling"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/taskqueue"
	bidparse "bid-engine/pkg/service/bidparse"
)

// ParseTaskPayload bid_gen_parse 任务负载
type ParseTaskPayload struct {
	GenProjectID      int64  `json:"gen_project_id"`
	AnalysisProjectID int64  `json:"analysis_project_id"`
	UserID            int64  `json:"user_id"`
	ParseType         string `json:"parse_type"` // tender / template
}

// BidGenParseHandler bid_gen_parse 队列任务 Handler
type BidGenParseHandler struct {
	Svc Service
}

// Handle 实现 taskqueue.TaskHandler
func (h *BidGenParseHandler) Handle(ctx context.Context, task *taskqueue.Task) error {
	var payload ParseTaskPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("解析 bid_gen_parse payload 失败: %w", err)
	}
	svc, ok := h.Svc.(*svcImpl)
	if !ok {
		return fmt.Errorf("BidGenParseHandler: Svc 类型断言失败")
	}
	flowCtx := repoLLM.WithUserID(ctx, payload.UserID)

	proj, err := svc.repo.GetProjectByID(flowCtx, payload.GenProjectID)
	if err != nil || proj == nil {
		return fmt.Errorf("标书项目不存在: %d", payload.GenProjectID)
	}

	switch payload.ParseType {
	case "tender":
		return svc.runTenderParse(flowCtx, proj, payload, task)
	case "template":
		return svc.runTemplateParse(flowCtx, proj, payload, task)
	default:
		return fmt.Errorf("未知解析类型: %s", payload.ParseType)
	}
}

var _ taskqueue.TaskHandler = (*BidGenParseHandler)(nil)

// HandleParseTaskFinal 仅在队列最终失败时落业务失败态，避免中间重试被前端误判为终态。
func (s *svcImpl) HandleParseTaskFinal(ctx context.Context, task *taskqueue.Task, succeeded bool) {
	if succeeded {
		return
	}
	var payload ParseTaskPayload
	if json.Unmarshal([]byte(task.Payload), &payload) != nil || payload.GenProjectID <= 0 {
		return
	}
	project, err := s.repo.GetProjectByID(ctx, payload.GenProjectID)
	if err != nil || project == nil || project.Status != ProjectStatusParsing {
		return
	}
	latest, _ := s.taskqueueRepo.GetTask(ctx, task.ID)
	message := "解析任务多次重试后失败，请稍后重试"
	if payload.ParseType == "template" {
		message = "模板解析失败，请确认文件未损坏且包含可识别的大纲结构"
	}
	if latest != nil && strings.TrimSpace(latest.LastError) != "" {
		s.logger.Warnw("标书解析任务最终失败", "task_id", task.ID, "project_id", project.ID, "err", latest.LastError)
		if payload.ParseType == "template" && strings.Contains(latest.LastError, "LLM未配置") {
			message = "模板未识别到结构化大纲，且标书生成模型未配置"
		}
	}
	stage := GenStageTemplateParse
	if payload.ParseType == "template" && (project.Stage == GenStageTemplateParse || project.Stage == GenStageTemplateOutline) {
		stage = project.Stage
	} else if payload.ParseType == "tender" {
		stage = findFirstFailedStage(project.StageStatus)
		if stage == "" {
			stage = GenStageTenderInterpretation
		}
	}
	_ = s.markGenStage(ctx, project.ID, stage, parseStageFailed, 0)
	_ = s.failProject(ctx, project.ID, message)
}

// runTenderParse 招标文件解析：招标文件解读 → 提炼重要信息 → 大纲蓝图生成
func (s *svcImpl) runTenderParse(ctx context.Context, proj *model.BidGenProject, payload ParseTaskPayload, task *taskqueue.Task) error {
	_ = s.markGenStage(ctx, proj.ID, GenStageTenderInterpretation, parseStageRunning, 20)
	if err := s.analysisSvc.RunForBidGen(ctx, payload.AnalysisProjectID, payload.UserID); err != nil {
		_ = s.markGenStage(ctx, proj.ID, GenStageTenderInterpretation, parseStageFailed, 0)
		return err
	}
	_ = s.markGenStage(ctx, proj.ID, GenStageTenderInterpretation, parseStageSucceeded, 35)
	_ = s.markGenStage(ctx, proj.ID, GenStageInfoExtraction, parseStageSucceeded, 65)
	_ = s.markGenStage(ctx, proj.ID, GenStageBlueprintGeneration, parseStageSucceeded, 95)

	// 全部成功：蓝图 → 大纲 + 文档
	if err := s.blueprintToOutline(ctx, proj, payload); err != nil {
		_ = s.markGenStage(ctx, proj.ID, GenStageBlueprintGeneration, parseStageFailed, 0)
		return err
	}
	if err := s.repo.UpdateProjectFields(ctx, proj.ID, map[string]interface{}{
		"status":   ProjectStatusOutlineReview,
		"stage":    "",
		"progress": 100,
	}); err != nil {
		return err
	}
	s.logger.Infow("招标文件解析完成，待确认大纲", "project_id", proj.ID)
	return nil
}

// runAnalysisStages 依次执行招标解析阶段
// markGenStage 更新标书项目展示阶段状态与进度
func (s *svcImpl) markGenStage(ctx context.Context, projectID int64, stage, status string, progress int32) error {
	m := parseStageStatusJSON(loadStageStatusStr(ctx, s, projectID))
	m[stage] = status
	b, _ := json.Marshal(m)
	fields := map[string]interface{}{"stage_status": string(b), "stage": stage}
	if progress > 0 {
		fields["progress"] = progress
	}
	return s.repo.UpdateProjectFields(ctx, projectID, fields)
}

func loadStageStatusStr(ctx context.Context, s *svcImpl, projectID int64) string {
	p, err := s.repo.GetProjectByID(ctx, projectID)
	if err != nil || p == nil {
		return ""
	}
	return p.StageStatus
}

func findFirstFailedStage(stageStatusStr string) string {
	m := parseStageStatusJSON(stageStatusStr)
	for _, st := range []string{GenStageTenderInterpretation, GenStageInfoExtraction, GenStageBlueprintGeneration, GenStageTemplateParse, GenStageTemplateOutline} {
		if m[st] == parseStageFailed {
			return st
		}
	}
	return ""
}

// blueprintToOutline 将内部招标解析项目的蓝图节点平移为 bid_gen_outline + 生成仅标题文档
func (s *svcImpl) blueprintToOutline(ctx context.Context, genProj *model.BidGenProject, payload ParseTaskPayload) error {
	blueprintNodes, err := s.analysisSvc.BlueprintNodes(ctx, payload.AnalysisProjectID)
	if err != nil {
		return fmt.Errorf("加载蓝图失败: %w", err)
	}
	if len(blueprintNodes) == 0 {
		return fmt.Errorf("蓝图为空")
	}

	// 蓝图后处理：按树序重排章节序号（幂等，兼容存量旧蓝图），返回父先于子的树序列表
	ordered := normalizeBlueprintTitles(blueprintNodes)

	return s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id=?", genProj.ID).Delete(&model.BidGenOutline{}).Error; err != nil {
			return err
		}
		outlineNodes := make([]*model.BidGenOutline, 0, len(ordered))
		idMap := make(map[int64]int64)
		for _, bn := range ordered {
			source := "ai"
			if bn.ParentID == 0 && bn.NodeSource == "system_root" {
				source = "system_root"
			}
			node := &model.BidGenOutline{
				ProjectID: genProj.ID, ParentID: idMap[bn.ParentID], Level: bn.Level, SortOrder: bn.SortOrder,
				Title: bn.Title, ClauseIds: bn.ClauseIdsJSON, MaterialIds: bn.MaterialIdsJSON,
				GenStatus: OutlineGenPending, Source: source, IsRequiredFile: bn.IsRequiredFile, IsAiSuggested: bn.IsAiSuggested,
			}
			if err := tx.Create(node).Error; err != nil {
				return fmt.Errorf("创建大纲节点失败: %w", err)
			}
			idMap[bn.ID] = node.ID
			outlineNodes = append(outlineNodes, node)
		}
		docJSON, err := buildDocFromOutline(outlineNodes)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.Assignments(map[string]any{"doc_json": docJSON, "version": gorm.Expr("version + 1")}),
		}).Create(&model.BidGenDocContent{ProjectID: genProj.ID, DocJSON: docJSON}).Error; err != nil {
			return err
		}
		if len(ordered) > 0 && ordered[0].ParentID == 0 && strings.TrimSpace(ordered[0].Title) != "" {
			return tx.Model(&model.BidGenProject{}).Where("id=?", genProj.ID).Update("name", strings.TrimSpace(ordered[0].Title)).Error
		}
		return nil
	})
}

func sortBlueprintNodes(nodes []*model.BidAnalysisV3Blueprint) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j-1].SortOrder > nodes[j].SortOrder; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
}

// normalizeOutlineNodes 按层级重排大纲标题序号（模板提取路径用）
func normalizeOutlineNodes(nodes []*model.BidGenOutline) {
	if len(nodes) == 0 {
		return
	}
	items := make([]bidparse.OutlineItem, len(nodes))
	for i, n := range nodes {
		items[i] = bidparse.OutlineItem{Title: n.Title, Level: n.Level}
	}
	items = bidparse.NormalizeOutlineTitles(items)
	for i, n := range nodes {
		n.Title = items[i].Title
	}
}

// normalizeBlueprintTitles 按树序重排蓝图标题序号并返回树序列表
// （蓝图平移路径用，幂等兜底兼容存量旧蓝图）。
// 蓝图的 level=1 节点是文档标题根（不编号），level=2 起为“第一章/1.1/1.1.1…”。
func normalizeBlueprintTitles(nodes []*model.BidAnalysisV3Blueprint) []*model.BidAnalysisV3Blueprint {
	if len(nodes) == 0 {
		return nodes
	}
	ordered := flattenBlueprintTree(nodes)
	items := make([]bidparse.OutlineItem, len(ordered))
	for i, n := range ordered {
		items[i] = bidparse.OutlineItem{Title: n.Title, Level: n.Level}
	}
	items = bidparse.NormalizeBlueprintTitles(items)
	for i, n := range ordered {
		n.Title = items[i].Title
	}
	return ordered
}

// flattenBlueprintTree 把蓝图的扁平节点按“父先于子、同级按 sort_order”的树序展开，
// 避免全局按 sort_order 排序破坏父子先后关系。
func flattenBlueprintTree(nodes []*model.BidAnalysisV3Blueprint) []*model.BidAnalysisV3Blueprint {
	byParent := map[int64][]*model.BidAnalysisV3Blueprint{}
	var roots []*model.BidAnalysisV3Blueprint
	for _, n := range nodes {
		if n.ParentID == 0 {
			roots = append(roots, n)
		} else {
			byParent[n.ParentID] = append(byParent[n.ParentID], n)
		}
	}
	sortBlueprintNodes(roots)
	for _, list := range byParent {
		sortBlueprintNodes(list)
	}
	ordered := make([]*model.BidAnalysisV3Blueprint, 0, len(nodes))
	var walk func(n *model.BidAnalysisV3Blueprint)
	walk = func(n *model.BidAnalysisV3Blueprint) {
		ordered = append(ordered, n)
		for _, child := range byParent[n.ID] {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return ordered
}

// runTemplateParse 模板解析：docling → HTML → 大纲提取
func (s *svcImpl) runTemplateParse(ctx context.Context, proj *model.BidGenProject, payload ParseTaskPayload, task *taskqueue.Task) error {
	if err := s.markGenStage(ctx, proj.ID, GenStageTemplateParse, parseStageRunning, 30); err != nil {
		s.logger.Warnw("更新阶段状态失败", "err", err)
	}
	_ = os.MkdirAll("./tmp", 0777)
	ext := strings.ToLower(filepath.Ext(proj.SourceFileName))
	localPath := filepath.Join("./tmp", fmt.Sprintf("bidgen-tpl-%d%s", time.Now().UnixNano(), ext))
	if err := s.oss.Get(ctx, proj.SourceFileObject, localPath); err != nil {
		return err
	}
	defer os.Remove(localPath)

	parseResult, parseErr := s.parseTemplateSource(ctx, proj, localPath, ext)
	if parseErr != nil {
		s.logger.Warnw("模板解析失败", "project_id", proj.ID, "status", templateParseStatus(parseResult), "err", parseErr)
		return parseErr
	}
	if err := s.markGenStage(ctx, proj.ID, GenStageTemplateParse, parseStageSucceeded, 60); err != nil {
		return err
	}

	if err := s.markGenStage(ctx, proj.ID, GenStageTemplateOutline, parseStageRunning, 80); err != nil {
		return err
	}
	headings, candidates := extractTemplateHeadings(parseResult)
	if len(headings) == 0 {
		var err error
		headings, err = s.llmTemplateOutline(ctx, parseResult, orderedTemplateCandidates(candidates))
		if err != nil {
			return &permanentParseError{err: fmt.Errorf("模板未提取到可靠大纲: %w", err)}
		}
	}
	sourceHTML := templateSourceHTML(parseResult)
	if err := s.persistTemplateOutline(ctx, proj.ID, headings, sourceHTML); err != nil {
		return &permanentParseError{err: err}
	}
	s.logger.Infow("模板大纲提取完成", "project_id", proj.ID, "nodes", len(headings))
	s.logger.Infow("模板解析完成，待确认大纲", "project_id", proj.ID)
	return nil
}

func (s *svcImpl) failProject(ctx context.Context, projectID int64, msg string) error {
	return s.repo.UpdateProjectFields(ctx, projectID, map[string]interface{}{
		"status": ProjectStatusFailed, "last_error": msg,
	})
}

// extractOutlineFromHTML 从 HTML 提取 h1-h6 标题序列（文档顺序）
func extractOutlineFromHTML(htmlStr string) []*model.BidGenOutline {
	if strings.TrimSpace(htmlStr) == "" {
		return nil
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil
	}
	type headingItem struct {
		level int
		text  string
	}
	var headings []headingItem
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
			text := strings.TrimSpace(extractText(n))
			if text != "" {
				headings = append(headings, headingItem{level: int(n.Data[1] - '0'), text: text})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(headings) == 0 {
		return nil
	}
	nodes := make([]*model.BidGenOutline, 0, len(headings))
	sortOrder := int32(0)
	for _, h := range headings {
		nodes = append(nodes, &model.BidGenOutline{
			Level:     int32(h.level),
			SortOrder: sortOrder,
			Title:     h.text,
			GenStatus: OutlineGenPending,
			Source:    "user",
		})
		sortOrder += 100
	}
	return nodes
}

func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// fixOutlineParents 插入后按标题层级修正 parent_id（栈法）
func (s *svcImpl) fixOutlineParents(ctx context.Context, projectID int64) {
	nodes, err := s.repo.GetOutlineByProjectID(ctx, projectID)
	if err != nil || len(nodes) == 0 {
		return
	}
	stack := []int{-1}
	for i, n := range nodes {
		for len(stack) > 1 {
			top := stack[len(stack)-1]
			if nodes[top].Level >= n.Level {
				stack = stack[:len(stack)-1]
			} else {
				break
			}
		}
		parentIdx := stack[len(stack)-1]
		parentID := int64(0)
		if parentIdx >= 0 {
			parentID = nodes[parentIdx].ID
		}
		if n.ParentID != parentID {
			_ = s.repo.UpdateOutlineNode(ctx, n.ID, map[string]interface{}{"parent_id": parentID})
			n.ParentID = parentID
		}
		stack = append(stack, i)
	}
}

// extractOutlineFromDocling 从 Docling 解析结果提取大纲：
// 1) 优先用结构化 label（title/section_header/...）
// 2) 无结构化标题时，按章节编号模式启发式提取
func extractOutlineFromDocling(result *docling.ParseResult) []*model.BidGenOutline {
	var nodes []*model.BidGenOutline
	sortOrder := int32(0)

	if result != nil {
		doc, err := result.ParseDocument()
		if err == nil && doc != nil {
			for _, t := range doc.Texts {
				if t.ContentLayer == "furniture" {
					continue
				}
				level := labelHeadingLevel(t.Label)
				if level == 0 {
					continue
				}
				title := strings.TrimSpace(t.Text)
				if title == "" {
					continue
				}
				nodes = append(nodes, &model.BidGenOutline{
					Level:     int32(level),
					SortOrder: sortOrder,
					Title:     title,
					GenStatus: OutlineGenPending,
					Source:    "user",
				})
				sortOrder += 100
			}
		}
	}

	if len(nodes) == 0 && result != nil {
		text := result.Text
		if strings.TrimSpace(text) == "" {
			text = result.Markdown
		}
		nodes = extractOutlineHeuristic(text)
	}
	return nodes
}

// labelHeadingLevel Docling 文本 label → 标题层级（0 表示非标题）
func labelHeadingLevel(label string) int {
	switch label {
	case "title":
		return 1
	case "section_header":
		return 2
	case "subsection_header":
		return 3
	case "sub_subsection_header":
		return 4
	}
	return 0
}

var (
	reChapterNum  = regexp.MustCompile(`^第[一二三四五六七八九十百千]+[章节篇部分]`)
	reChineseList = regexp.MustCompile(`^[一二三四五六七八九十]+[、．.]`)
	reNum3        = regexp.MustCompile(`^\d+\.\d+\.\d+`)
	reNum2        = regexp.MustCompile(`^\d+\.\d+\s`)
	reNum1        = regexp.MustCompile(`^\d+[、.．]\s*`)
	reParenList   = regexp.MustCompile(`^[（(][一二三四五六七八九十]+[）)]`)
)

// extractOutlineHeuristic 按章节编号模式启发式提取大纲（Docling 未保留标题语义时的兜底）
func extractOutlineHeuristic(text string) []*model.BidGenOutline {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var nodes []*model.BidGenOutline
	sortOrder := int32(0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		level := 0
		switch {
		case reChapterNum.MatchString(line):
			level = 1
		case reChineseList.MatchString(line):
			level = 1
		case reParenList.MatchString(line):
			level = 2
		case reNum3.MatchString(line):
			level = 4
		case reNum2.MatchString(line):
			level = 3
		case reNum1.MatchString(line):
			level = 2
		}
		if level == 0 {
			continue
		}
		nodes = append(nodes, &model.BidGenOutline{
			Level:     int32(level),
			SortOrder: sortOrder,
			Title:     line,
			GenStatus: OutlineGenPending,
			Source:    "user",
		})
		sortOrder += 100
	}
	return nodes
}
