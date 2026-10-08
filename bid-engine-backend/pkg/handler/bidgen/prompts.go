package bidgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bid-engine/pkg/db/model"
)

// buildGlobalContext 构建生成全局上下文：项目名称 + 招标解析摘要。
// 改造后不再把全部招标事实广播到每一章，事实改由章节证据包按需提供。
func (s *svcImpl) buildGlobalContext(ctx context.Context, proj *model.BidGenProject) string {
	if proj == nil {
		return ""
	}
	parts := []string{"【项目名称】" + proj.Name}
	if proj.TenderProjectID > 0 {
		if tenderContext, err := s.analysisSvc.GenerationContext(ctx, proj.TenderProjectID); err == nil {
			if trimmed := strings.TrimSpace(tenderContext); trimmed != "" {
				parts = append(parts, "【招标解析摘要】"+truncateRunes(trimmed, 1500))
			}
		}
	}
	return strings.Join(parts, "\n")
}

type chapterPromptInput struct {
	proj        *model.BidGenProject
	node        *model.BidGenOutline
	globalCtx   string
	evidence    *chapterEvidence
	spec        *chapterSpec
	length      string
	mode        string
	existing    string
	instruction string
	children    []string
}

// buildChapterPrompt 构建单章生成 prompt（系统提示 + 用户提示）。
// 四种模式共用同一套投标文件写作规范，差异只在任务描述与“是否基于现有正文”。
func (s *svcImpl) buildChapterPrompt(in chapterPromptInput) (string, string) {
	node := in.node
	taskLine := "任务：为指定的标书章节撰写正文内容。"
	opening := "请撰写以下标书章节的正文。"
	switch in.mode {
	case GenModeRewrite:
		taskLine = "任务：重写指定章节的现有正文。保持原意、事实、数据与承诺不变，语言更专业、精炼、通顺，并保持原有的响应性与结构层次。"
		opening = "请重写以下标书章节的现有正文（保持原意与响应性不变，语言更专业、精炼、通顺）。"
	case GenModeExpand:
		taskLine = "任务：在保留现有正文内容、结构、事实与承诺的基础上扩写该章节。只补充论证、举证、流程、机制与细节，不得注水、不得新增原文没有的事实。"
		opening = "请在保留现有内容与结构的基础上，扩写以下标书章节的正文。"
	case GenModeCondense:
		taskLine = "任务：精简压缩该章节现有正文。必须保留全部核心要点、对招标要求的响应表述与关键数据，删除冗余、重复与空泛表达。"
		opening = "请精简压缩以下标书章节的现有正文（保留全部核心要点、响应表述与关键数据）。"
	}

	system := strings.Join([]string{
		bidWritingSafetyPrefix,
		"你是资深投标文件撰写专家，精通商务标书与技术标书编制，熟悉招投标评审规则。" + taskLine,
		bidWritingRules(),
		`输出格式要求：
1. 只输出该章节的正文内容，不要输出章节标题本身（标题已存在），也不要输出任何解释、前言或结语说明。
2. 用 ## / ### 作为正文小标题，用 - 作为项目符号，用 Markdown 管道表格 | 呈现表格；不同段落之间用空行分隔。
3. 需要插入素材图片时，在相应论述之后另起一行输出占位标记 [[图:M1]]；需要评分响应或业绩一览表时，可另起一行输出 [[表:评分]] 或 [[表:业绩]]。除这些标记外不得输出其它系统指令。`,
	}, "\n\n")

	if len(in.children) > 0 {
		system += "\n\n本章包含独立子章节；这里只撰写承上启下的章节概述，不展开子章节的具体内容。"
	}

	lengthDesc := fmt.Sprintf("本章目标字数约 %d 字", in.specTargetWords())
	lengthDesc += fmt.Sprintf("（当前篇幅档位：%s）。正文应当写满目标字数，内容详实，但严禁用套话凑字数。", lengthTierLabel(in.length))

	chapterCtx := []string{"【章节标题】" + node.Title}
	if len(in.children) > 0 {
		chapterCtx = append(chapterCtx, "【直接子章节】"+strings.Join(in.children, "、"))
	}

	parts := []string{opening}
	if strings.TrimSpace(in.globalCtx) != "" {
		parts = append(parts, in.globalCtx)
	}
	parts = append(parts, strings.Join(chapterCtx, "\n"))

	if in.evidence != nil {
		if rendered := in.evidence.render(); rendered != "" {
			parts = append(parts, "【章节证据】\n"+rendered)
		}
	}
	if specSection := renderSpecSection(in.spec); specSection != "" {
		parts = append(parts, specSection)
	}
	if in.mode != GenModeWrite {
		if existing := strings.TrimSpace(in.existing); existing != "" {
			parts = append(parts, "【现有正文】\n"+existing)
		}
	}

	parts = append(parts, "【篇幅要求】"+lengthDesc)
	if in.mode != GenModeWrite {
		if instruction := strings.TrimSpace(in.instruction); instruction != "" {
			parts = append(parts, "【补充要求】"+instruction)
		}
	}
	parts = append(parts, "请开始：")
	return system, strings.Join(parts, "\n\n")
}

func (in chapterPromptInput) specTargetWords() int {
	if in.spec != nil && in.spec.TargetWords > 0 {
		return in.spec.TargetWords
	}
	if in.spec != nil && in.spec.TargetWords <= 0 {
		return minChapterWords
	}
	return minChapterWords
}

// renderSpecSection 渲染写作规格段落（要点、表格与评分项响应的规划）。
func renderSpecSection(spec *chapterSpec) string {
	if spec == nil || len(spec.Points) == 0 {
		return ""
	}
	lines := make([]string, 0, len(spec.Points)+4)
	for i, point := range spec.Points {
		line := fmt.Sprintf("%d. %s", i+1, point.Title)
		if point.Detail != "" {
			line += "：" + point.Detail
		}
		if len(point.Evidence) > 0 {
			line += "（依据：" + strings.Join(point.Evidence, "、") + "）"
		}
		lines = append(lines, line)
	}
	if len(spec.Tables) > 0 {
		tables := make([]string, 0, len(spec.Tables))
		for _, table := range spec.Tables {
			entry := table.Title
			if len(table.Columns) > 0 {
				entry += "（列：" + strings.Join(table.Columns, "、") + "）"
			}
			tables = append(tables, entry)
		}
		lines = append(lines, "需输出表格："+strings.Join(tables, "；"))
	}
	if len(spec.Scoring) > 0 {
		items := make([]string, 0, len(spec.Scoring))
		for _, row := range spec.Scoring {
			entry := row.Item
			if row.Score != "" {
				entry += "（" + row.Score + "）"
			}
			items = append(items, entry)
		}
		lines = append(lines, "本章需响应的评分项："+strings.Join(items, "；")+"。必须逐条给出明确的响应结论与支撑说明。")
	}
	if len(spec.MaterialRefs) > 0 {
		lines = append(lines, "本章应引用的素材卡片："+strings.Join(spec.MaterialRefs, "、"))
	}
	lines = append(lines, "请严格按上述要点顺序展开，要点必须全部覆盖。")
	return "【本章写作规格】\n" + strings.Join(lines, "\n")
}

// ── 兼容旧调用点 ────────────────────────────────────────────────

type bidSourceSnapshotPayload struct {
	Summary     json.RawMessage                  `json:"summary"`
	Fields      []*model.BidAnalysisV3Field      `json:"fields"`
	FieldValues []*model.BidAnalysisV3FieldValue `json:"field_values"`
	Clauses     []*model.BidAnalysisV3Clause     `json:"clauses"`
}

func (s *svcImpl) loadSourceSnapshot(ctx context.Context, bidProjectID int64) (*bidSourceSnapshotPayload, error) {
	record, err := s.repo.GetSourceSnapshot(ctx, bidProjectID)
	if err != nil {
		return nil, err
	}
	var payload bidSourceSnapshotPayload
	if err := json.Unmarshal([]byte(record.SnapshotJSON), &payload); err != nil {
		return nil, fmt.Errorf("解析投标书来源快照失败: %w", err)
	}
	return &payload, nil
}
