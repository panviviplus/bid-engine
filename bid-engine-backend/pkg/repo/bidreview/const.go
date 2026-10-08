package bidreview

import "encoding/json"

// ================================================================
// 标书审核 V2 常量与阶段状态机
// ================================================================

// 项目状态
const (
	ProjectStatusRunning   = "running"
	ProjectStatusSucceed   = "succeed"
	ProjectStatusFailed    = "failed"
	ProjectStatusCancelled = "cancelled"
)

// 阶段状态
const (
	StageStatusPending   = "pending"
	StageStatusRunning   = "running"
	StageStatusSucceeded = "succeeded"
	StageStatusFailed    = "failed"
	StageStatusSkipped   = "skipped"
)

// 审核流水线阶段（8 阶段 + 终态 completed）
//
// 阶段顺序与业务语义对齐：先分别解析招标文件与投标文件，再执行审核内部环节。
const (
	StageTenderParse    = "tender_parse"    // 1 招标文件解析（绑定/触发解析 + 冻结审核依据）
	StageBidParse       = "bid_parse"       // 2 投标文件解析（页码/页块/分块/术语索引）
	StageChecklistBuild = "checklist_build" // 3 审核清单生成（解析依据 + 规则库）
	StageEvidenceMatch  = "evidence_match"  // 4 逐条证据召回
	StageVerdict        = "verdict"         // 5 分层判定（高危逐条 + 低危批量）
	StageFormatScan     = "format_scan"     // 6 暗标/版式确定性扫描
	StageScoring        = "scoring"         // 7 竞争力评分对标
	StageFinalize       = "finalize"        // 8 汇总与整改清单
	StageCompleted      = "completed"       // 终态
)

// StageOrder 阶段顺序（断点续跑判断用）
var StageOrder = []string{
	StageTenderParse,
	StageBidParse,
	StageChecklistBuild,
	StageEvidenceMatch,
	StageVerdict,
	StageFormatScan,
	StageScoring,
	StageFinalize,
	StageCompleted,
}

// StageDefinition 阶段定义（供接口下发阶段目录与进度权重）
type StageDefinition struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Weight int32  `json:"weight"`
	Order  int32  `json:"order"`
}

// StageDefinitions 阶段目录（顺序 + 权重 + 中文名）
var StageDefinitions = []StageDefinition{
	{Name: StageTenderParse, Label: "招标文件解析", Weight: 40, Order: 1},
	{Name: StageBidParse, Label: "投标文件解析", Weight: 25, Order: 2},
	{Name: StageChecklistBuild, Label: "审核清单生成", Weight: 5, Order: 3},
	{Name: StageEvidenceMatch, Label: "证据定位", Weight: 4, Order: 4},
	{Name: StageVerdict, Label: "逐条判定", Weight: 20, Order: 5},
	{Name: StageFormatScan, Label: "暗标版式检查", Weight: 2, Order: 6},
	{Name: StageScoring, Label: "评分对标", Weight: 2, Order: 7},
	{Name: StageFinalize, Label: "汇总完成", Weight: 2, Order: 8},
}

// StageProgressWeights 各阶段进度权重（总和 = 100）
var StageProgressWeights = func() map[string]int32 {
	m := make(map[string]int32, len(StageDefinitions))
	for _, def := range StageDefinitions {
		m[def.Name] = def.Weight
	}
	return m
}()

// StageLabel 阶段中文名
func StageLabel(stage string) string {
	for _, def := range StageDefinitions {
		if def.Name == stage {
			return def.Label
		}
	}
	if stage == StageCompleted {
		return "已完成"
	}
	return stage
}

// StageIndex 阶段序号（不存在返回 -1）
func StageIndex(stage string) int {
	for i, name := range StageOrder {
		if name == stage {
			return i
		}
	}
	return -1
}

// IsValidStage 是否为可执行阶段（排除 completed）
func IsValidStage(stage string) bool {
	idx := StageIndex(stage)
	return idx >= 0 && stage != StageCompleted
}

// legacyStageAlias 早期阶段命名 → 现役阶段命名（兼容已存在的审核项目）
var legacyStageAlias = map[string]string{
	"source_bind": StageTenderParse,
	"bid_ingest":  StageBidParse,
}

// NormalizeStage 归一化阶段名（兼容旧命名；未知阶段原样返回）
func NormalizeStage(stage string) string {
	if alias, ok := legacyStageAlias[stage]; ok {
		return alias
	}
	return stage
}

// NormalizeStageStatus 归一化 stage_status（旧键迁移到新键，保留 completed 等终态键）
func NormalizeStageStatus(m map[string]string) map[string]string {
	if m == nil {
		return m
	}
	for legacy, current := range legacyStageAlias {
		value, ok := m[legacy]
		if !ok {
			continue
		}
		delete(m, legacy)
		if _, exists := m[current]; !exists {
			m[current] = value
		}
	}
	return m
}

// StageOrderFrom 返回 stage 及其后续可执行阶段（断点重跑需要逐阶段清理产物）
func StageOrderFrom(stage string) []string {
	idx := StageIndex(stage)
	if idx < 0 {
		return nil
	}
	out := make([]string, 0, len(StageOrder)-idx)
	for _, name := range StageOrder[idx:] {
		if name == StageCompleted {
			continue
		}
		out = append(out, name)
	}
	return out
}

// BaselineProgress 返回“从某阶段开始执行”时的进度基线
// （= 该阶段之前所有已成功/跳过阶段的权重之和）
func BaselineProgress(stageStatus map[string]string, stage string) int32 {
	var total int32
	for _, name := range StageOrder {
		if name == StageCompleted || name == stage {
			break
		}
		status := stageStatus[name]
		if status == StageStatusSucceeded || status == StageStatusSkipped {
			total += StageProgressWeights[name]
		}
	}
	return total
}

// ResetStageStatusFrom 把 stage 及其后续阶段重置为 pending（断点重跑语义）
func ResetStageStatusFrom(stageStatus map[string]string, stage string) map[string]string {
	out := make(map[string]string, len(stageStatus))
	for k, v := range stageStatus {
		out[k] = v
	}
	started := false
	for _, name := range StageOrder {
		if name == StageCompleted {
			continue
		}
		if name == stage {
			started = true
		}
		if !started {
			continue
		}
		out[name] = StageStatusPending
	}
	return out
}

// 审核维度
const (
	DimensionCompliance      = "compliance"      // 合规性（资格/否决项/响应性）
	DimensionCompleteness    = "completeness"    // 完整性（材料齐套/章节覆盖）
	DimensionCompetitiveness = "competitiveness" // 竞争力（评分对标）
	DimensionFormat          = "format"          // 版式/暗标
)

// Dimensions 维度展示顺序
var Dimensions = []string{
	DimensionCompliance,
	DimensionCompleteness,
	DimensionCompetitiveness,
	DimensionFormat,
}

// 判定结论
const (
	FindingPass     = "pass"
	FindingWarning  = "warning"
	FindingError    = "error"
	FindingNA       = "na"
	FindingNotFound = "not_found"
)

// 人工确认状态
const (
	CheckPending   = "pending"
	CheckConfirmed = "confirmed"
	CheckRejected  = "rejected"
)

// 整改状态
const (
	RemediationTodo    = "todo"
	RemediationDoing   = "doing"
	RemediationDone    = "done"
	RemediationIgnored = "ignored"
)

// 清单项来源
const (
	SourcePreset   = "preset"
	SourceAnalysis = "analysis"
	SourceRule     = "rule"
	SourceUser     = "user"
	SourceFormat   = "format"
)

// DimensionLabel 维度中文名
func DimensionLabel(dimension string) string {
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

// CalculateProgress 根据 stage_status 动态计算整体进度
func CalculateProgress(stageStatus map[string]string) int32 {
	var total int32
	for _, stage := range StageOrder {
		if stage == StageCompleted {
			continue
		}
		status, ok := stageStatus[stage]
		if !ok {
			continue
		}
		if status == StageStatusSucceeded || status == StageStatusSkipped {
			if w, ok := StageProgressWeights[stage]; ok {
				total += w
			}
		}
	}
	if total > 100 {
		total = 100
	}
	return total
}

// ProgressWithActiveStage 把当前阶段内真实进度换算为审核总进度。
func ProgressWithActiveStage(stageStatus map[string]string, stage string, percent int32) int32 {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	base := CalculateProgress(stageStatus)
	if stageStatus[stage] != StageStatusRunning {
		return base
	}
	return base + StageProgressWeights[stage]*percent/100
}

// NewStageStatusMap 初始化全 pending 的阶段状态映射
func NewStageStatusMap() map[string]string {
	m := make(map[string]string, len(StageOrder))
	for _, stage := range StageOrder {
		m[stage] = StageStatusPending
	}
	return m
}

// FindFirstFailedStage 按 StageOrder 返回第一个 failed 阶段（无则返回空串）
func FindFirstFailedStage(stageStatus map[string]string) string {
	for _, stage := range StageOrder {
		if stage == StageCompleted {
			break
		}
		if stageStatus[stage] == StageStatusFailed {
			return stage
		}
	}
	return ""
}

// HasFailedStage 是否存在 failed 阶段
func HasFailedStage(stageStatus map[string]string) bool {
	return FindFirstFailedStage(stageStatus) != ""
}

// ResolveStartStage 解析流水线起始阶段：
//   - resumeFromStage 非空 → 直接使用
//   - 否则定位首个 failed 阶段（Worker 失败重试场景）
//   - 无 failed → 空串（从头开始）
func ResolveStartStage(stageStatus map[string]string, resumeFromStage string) string {
	if resumeFromStage != "" {
		return resumeFromStage
	}
	return FindFirstFailedStage(stageStatus)
}

// ResolveRetryableStage 计算可重试阶段，供前端“重试”入口使用：
//   - 优先首个 failed 阶段
//   - 其次：存在 running 残留（进程中断/重试中断）时回退到项目当前阶段
//   - 都没有则返回空串（无需重试）
func ResolveRetryableStage(stageStatus map[string]string, currentStage string) string {
	if stage := FindFirstFailedStage(stageStatus); stage != "" {
		return stage
	}
	for _, stage := range StageOrder {
		if stage == StageCompleted {
			break
		}
		if stageStatus[stage] == StageStatusRunning {
			if currentStage != "" && stageStatus[currentStage] == StageStatusRunning {
				return currentStage
			}
			return stage
		}
	}
	return ""
}

// SeverityRank 风险级别排序权重（数值越大越严重）
func SeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// MarshalStageStatus 序列化阶段状态
func MarshalStageStatus(m map[string]string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
