package bidanalysisv3

import (
	"encoding/json"
	"errors"
	"fmt"

	repollm "bid-engine/pkg/repo/llm"
)

const (
	// llmMaxOutputCeiling 当前模型网关（AgnesAI）对 max_tokens 的硬上限，
	// 超出会返回 HTTP 500（max_tokens exceeds the limit of 65536）。
	llmMaxOutputCeiling = 65536
	// 各功能固定的输出上限（tokens）。输出长度由本系统按功能定死，
	// 不随模型配置漂移：小模型同样能生成，也不会因请求超大输出而拖死单次调用。
	llmOutputCapExtraction  = 16384
	llmOutputCapSummary     = 8192
	llmOutputCapConsolidate = 4096
	llmOutputCapBlueprint   = 8192
	llmOutputCapInterpret   = 8192
	// defaultLLMInputChunkCeiling 是单次 LLM 输入分块的默认硬上限（tokens）。
	// 实际值由 bid_analysis_v3.llm_input_chunk_ceiling 配置注入。
	defaultLLMInputChunkCeiling = 120000
	// llmMinInputBudget 输入分块的最小可用 token 容量；再小的模型也按此粒度分批。
	llmMinInputBudget = 1000
	// llmContextReserve 为输出与 prompt 开销预留的 token。
	llmContextReserve = 4000
	// llmContextCeiling 配置上下文的有效上限，避免 1M 这类异常配置撑爆输入 token 容量。
	llmContextCeiling = 200000
	// llmMinContextWindow 支持的最小有效上下文（tokens），低于此值视为模型不可用。
	llmMinContextWindow = 8192
)

// resolveMaxOutput 输出上限由本系统按功能固定（cap），最多尊重一个更小的显式配置；
// 模型配置里过大的 max_output_tokens 一律钳到功能上限，避免打崩流水线或拖死单次调用。
func resolveMaxOutput(configured, cap int) int {
	if configured > 0 && configured < cap {
		return configured
	}
	return cap
}

// llmBudget 描述一次 LLM 处理的容量：输入分块大小、输出上限与有效上下文。
type llmBudget struct {
	ContextWindow      int
	MaxOutput          int
	InputTokenCapacity int
}

// resolveLLMBudget 计算与模型无关的分批容量：
//   - 输入分块恒有硬上限 llmInputChunkCeiling，超长文档自动切成多个小包；
//   - 输出上限同时受网关上限与上下文约束，小模型自动降低输出上限、扩大输入空间；
//   - 输入可用 token 容量始终不低于 llmMinInputBudget，绝不因“模型太小”直接报错。
//
// 小模型只是调用次数变多（分批更细），而不是“没法用”。
func resolveLLMBudget(contextWindow, configuredMaxOutput, outputCap, inputCeiling int) llmBudget {
	effectiveContext := contextWindow
	if effectiveContext <= 0 {
		effectiveContext = 32768
	}
	if effectiveContext > llmContextCeiling {
		effectiveContext = llmContextCeiling
	}
	maxOutput := resolveMaxOutput(configuredMaxOutput, outputCap)
	available := effectiveContext - llmContextReserve
	if maxOutput > available-llmMinInputBudget {
		maxOutput = available - llmMinInputBudget
	}
	if maxOutput < 512 {
		maxOutput = 512
	}
	budget := available - maxOutput
	if inputCeiling < llmMinInputBudget {
		inputCeiling = defaultLLMInputChunkCeiling
	}
	if budget > inputCeiling {
		budget = inputCeiling
	}
	return llmBudget{ContextWindow: effectiveContext, MaxOutput: maxOutput, InputTokenCapacity: budget}
}

// warningChapterRef 告警详情中受影响章节的最小描述。
type warningChapterRef struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	PageStart int32  `json:"page_start"`
	PageEnd   int32  `json:"page_end"`
}

// isLLMInfraError 判断错误是否属于 LLM 基础设施故障（不可达/限流/超时/通用不可用）。
// 内容质量问题（如 JSON 解析失败、空输出）不属于基础设施故障。
func isLLMInfraError(err error) bool {
	return errors.Is(err, repollm.ErrLLMURLUnreachable) ||
		errors.Is(err, repollm.ErrLLMRateLimit) ||
		errors.Is(err, repollm.ErrLLMTimeout) ||
		errors.Is(err, repollm.ErrLLMUnavailable)
}

func isLLMStageFatal(err error) bool {
	return isLLMInfraError(err) ||
		errors.Is(err, repollm.ErrLLMNotConfigured) ||
		errors.Is(err, repollm.ErrLLMKeyUnauthorized) ||
		errors.Is(err, repollm.ErrLLMModelNotFound)
}

// llmInfraGroupKey 生成基础设施类告警的去重分组键，同一故障期只保留一条告警。
func llmInfraGroupKey(err error) string {
	code, _ := repollm.LLMErrorMeta(err)
	return fmt.Sprintf("llm_infra:%d", code)
}

// warningSeverityFor 按告警代码与根因返回告警级别：
// critical=LLM 基础设施故障；warning=内容质量/流程问题；info=提示性信息。
func warningSeverityFor(code string, err error) string {
	switch code {
	case "chapter_extract_failed", "dynamic_second_pass_failed", "summary_failed":
		if isLLMInfraError(err) {
			return "critical"
		}
		return "warning"
	case "dynamic_fields_ranked", "summary_llm_fallback":
		return "info"
	default:
		return "warning"
	}
}

// buildWarningDetail 构造告警结构化详情 JSON：累计次数、受影响章节、关联任务。
func buildWarningDetail(chapters []warningChapterRef, tasks []int64) *string {
	detail := map[string]any{"occurrences": 1}
	if len(chapters) > 0 {
		detail["chapters"] = chapters
	}
	if len(tasks) > 0 {
		detail["tasks"] = tasks
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	out := string(raw)
	return &out
}

// buildChapterExtractWarningDetail 构造章节提取失败告警的合并详情：每次调用携带该章节
// 与失败原因（reasons），AddWarning 按 code:chapter_extract_failed 幂等合并成一条告警，
// 避免“章节提取失败”按章节拆分出多条雷同告警。
func buildChapterExtractWarningDetail(chapters []warningChapterRef, reason string) *string {
	detail := map[string]any{"occurrences": 1, "reasons": map[string]int{reason: 1}}
	if len(chapters) > 0 {
		detail["chapters"] = chapters
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	out := string(raw)
	return &out
}

// buildValidationWarningDetail 构造字段校验失败告警的结构化详情：
// occurrences=被忽略候选数，reasons=各校验原因分布，chapters=受影响章节，tasks=关联任务。
func buildValidationWarningDetail(chapters []warningChapterRef, taskIDs []int64, reasons map[string]int, occurrences int) *string {
	detail := map[string]any{"occurrences": occurrences}
	if len(chapters) > 0 {
		detail["chapters"] = chapters
	}
	if len(taskIDs) > 0 {
		detail["tasks"] = taskIDs
	}
	if len(reasons) > 0 {
		detail["reasons"] = reasons
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	out := string(raw)
	return &out
}
