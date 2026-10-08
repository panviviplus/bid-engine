package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	repollm "bid-engine/pkg/repo/llm"
)

type extractionMode uint8

const (
	extractionModeDynamicFields extractionMode = iota
	extractionModeClauses
	extractionModeUnified
)

func extractionResponseFormatForMode(mode extractionMode) map[string]any {
	evidence := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"kind", "ref"}, "properties": map[string]any{
		"kind": map[string]any{"type": "string", "enum": []string{"block", "table"}},
		"ref":  map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
	}}
	candidate := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"field_key", "display_name", "category_key", "origin", "value_type", "extract_status", "display_value", "confidence", "evidence_refs"}, "properties": map[string]any{
		"field_key": map[string]any{"type": "string", "minLength": 3, "maxLength": 64}, "display_name": map[string]any{"type": "string", "minLength": 1, "maxLength": 80},
		"category_key": map[string]any{"type": "string", "enum": keys(allowedCategories)}, "origin": map[string]any{"type": "string", "enum": []string{"system", "dynamic"}},
		"value_type":     map[string]any{"type": "string", "enum": []string{"text", "amount", "date", "datetime", "location", "organization", "list", "object", "table"}},
		"extract_status": map[string]any{"type": "string", "enum": []string{"found", "ambiguous"}}, "display_value": map[string]any{"type": "string", "maxLength": 6000},
		"confidence": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}}, "evidence_refs": map[string]any{"type": "array", "maxItems": 6, "items": evidence},
	}}
	clause := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"title", "content", "importance", "evidence_refs"}, "properties": map[string]any{
		"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "content": map[string]any{"type": "string", "minLength": 1, "maxLength": 8000},
		"importance": map[string]any{"type": "string", "enum": []string{"critical", "high", "medium", "low"}}, "evidence_refs": map[string]any{"type": "array", "minItems": 1, "maxItems": 6, "items": evidence},
	}}
	properties := map[string]any{}
	required := make([]string, 0, 2)
	if mode == extractionModeDynamicFields || mode == extractionModeUnified {
		properties["candidates"] = map[string]any{"type": "array", "maxItems": 20, "items": candidate}
		required = append(required, "candidates")
	}
	if mode == extractionModeClauses || mode == extractionModeUnified {
		properties["clauses"] = map[string]any{"type": "array", "maxItems": 20, "items": clause}
		required = append(required, "clauses")
	}
	schema := map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	name := "bid_analysis_chapter_facts"
	if mode == extractionModeDynamicFields {
		name += "_dynamic_fields"
	} else if mode == extractionModeClauses {
		name += "_clauses"
	} else if mode == extractionModeUnified {
		name += "_unified"
	}
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": name, "strict": true, "schema": schema}}
}

func decodeExtractionResult(content string, mode extractionMode) (*extractionResult, error) {
	var parsed extractionResult
	err := decodeStrictJSON(content, &parsed)
	if err != nil && strings.Contains(err.Error(), "unknown field") {
		decoder := json.NewDecoder(strings.NewReader(stripJSONFence(content)))
		if decodeErr := decoder.Decode(&parsed); decodeErr != nil {
			return nil, decodeErr
		}
		if trailingErr := decoder.Decode(&struct{}{}); trailingErr != io.EOF {
			return nil, fmt.Errorf("JSON 包含额外内容")
		}
		err = nil
	}
	if err != nil {
		if mode == extractionModeUnified {
			var direct extractionResult
			if directErr := decodeLocalJSON(content, &direct); directErr == nil {
				return restrictExtractionResult(&direct, mode)
			}
			if isUnexpectedEOF(err) {
				candidates, candidateErr := decodeCompleteArrayItems[extractedCandidate](content, "candidates")
				clauses, clauseErr := decodeCompleteArrayItems[extractedClause](content, "clauses")
				if candidateErr == nil && clauseErr == nil {
					return restrictExtractionResult(&extractionResult{Candidates: candidates, Clauses: clauses}, mode)
				}
			}
			return nil, err
		}
		if mode == extractionModeDynamicFields {
			var direct []extractedCandidate
			if directErr := decodeLocalJSON(content, &direct); directErr == nil {
				return restrictExtractionResult(&extractionResult{Candidates: direct}, mode)
			}
			if isUnexpectedEOF(err) {
				items, salvageErr := decodeCompleteArrayItems[extractedCandidate](content, "candidates")
				if salvageErr != nil {
					items, salvageErr = decodeCompleteTopLevelArrayItems[extractedCandidate](content)
				}
				if salvageErr == nil {
					return restrictExtractionResult(&extractionResult{Candidates: items}, mode)
				}
			}
		}
		if mode == extractionModeClauses {
			var direct []extractedClause
			if directErr := decodeLocalJSON(content, &direct); directErr == nil {
				return restrictExtractionResult(&extractionResult{Clauses: direct}, mode)
			}
			if isUnexpectedEOF(err) {
				items, salvageErr := decodeCompleteArrayItems[extractedClause](content, "clauses")
				if salvageErr != nil {
					items, salvageErr = decodeCompleteTopLevelArrayItems[extractedClause](content)
				}
				if salvageErr == nil {
					return restrictExtractionResult(&extractionResult{Clauses: items}, mode)
				}
			}
		}
		return nil, err
	}
	if mode == extractionModeDynamicFields && parsed.Candidates == nil {
		return nil, fmt.Errorf("响应缺少 candidates")
	}
	if mode == extractionModeClauses && parsed.Clauses == nil {
		return nil, fmt.Errorf("响应缺少 clauses")
	}
	if mode == extractionModeUnified && (parsed.Candidates == nil || parsed.Clauses == nil) {
		return nil, fmt.Errorf("统一抽取响应缺少 candidates 或 clauses")
	}
	return restrictExtractionResult(&parsed, mode)
}

// restrictExtractionResult prevents a response from one chapter lane from
// leaking into the other lane's persistence path. Fixed/system candidates are
// exclusively handled by the independent full-document extraction lane.
func restrictExtractionResult(result *extractionResult, mode extractionMode) (*extractionResult, error) {
	if result == nil {
		return nil, nil
	}
	filtered := &extractionResult{}
	var errs []error
	if mode == extractionModeUnified {
		for _, candidate := range result.Candidates {
			if strings.TrimSpace(candidate.Origin) != "dynamic" {
				errs = append(errs, fmt.Errorf("统一抽取返回了非 dynamic 候选"))
				continue
			}
			filtered.Candidates = append(filtered.Candidates, candidate)
		}
		filtered.Clauses = append(filtered.Clauses, result.Clauses...)
	} else if mode == extractionModeDynamicFields {
		for _, candidate := range result.Candidates {
			if strings.TrimSpace(candidate.Origin) != "dynamic" {
				errs = append(errs, fmt.Errorf("动态字段 lane 返回了非 dynamic 候选"))
				continue
			}
			filtered.Candidates = append(filtered.Candidates, candidate)
		}
	} else {
		if len(result.Candidates) > 0 {
			errs = append(errs, fmt.Errorf("条款 lane 返回了候选字段"))
		}
		filtered.Clauses = append(filtered.Clauses, result.Clauses...)
	}
	return filtered, errors.Join(errs...)
}

func isUnexpectedEOF(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "unexpected eof")
}

func decodeCompleteArrayItems[T any](content, key string) ([]T, error) {
	cleaned := stripJSONFence(content)
	keyOffset := strings.Index(cleaned, `"`+key+`"`)
	if keyOffset < 0 {
		return nil, fmt.Errorf("JSON 缺少 %s", key)
	}
	arrayOffset := strings.Index(cleaned[keyOffset:], "[")
	if arrayOffset < 0 {
		return nil, fmt.Errorf("JSON 的 %s 不是数组", key)
	}
	return decodeCompleteArrayFrom[T](cleaned[keyOffset+arrayOffset:])
}

func decodeCompleteTopLevelArrayItems[T any](content string) ([]T, error) {
	cleaned := strings.TrimSpace(stripJSONFence(content))
	if !strings.HasPrefix(cleaned, "[") {
		return nil, fmt.Errorf("JSON 不是顶层数组")
	}
	return decodeCompleteArrayFrom[T](cleaned)
}

func decodeCompleteArrayFrom[T any](arrayJSON string) ([]T, error) {
	decoder := json.NewDecoder(strings.NewReader(arrayJSON))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("JSON 数组无法读取")
	}
	items := make([]T, 0)
	for decoder.More() {
		var item T
		if err := decoder.Decode(&item); err != nil {
			break
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("JSON 数组没有完整条目")
	}
	return items, nil
}

func shouldSplitExtraction(result *repollm.ChatResult, parseErr error, maxOutput int) bool {
	if result != nil && strings.EqualFold(strings.TrimSpace(result.FinishReason), "length") {
		return true
	}
	if parseErr == nil || (!errors.Is(parseErr, io.ErrUnexpectedEOF) && !strings.Contains(strings.ToLower(parseErr.Error()), "unexpected eof")) {
		return false
	}
	if result == nil || result.Usage == nil {
		return true
	}
	return maxOutput <= 0 || result.Usage.CompletionTokens >= maxOutput*9/10
}

type adaptiveExtractionOutcome struct {
	Result *extractionResult
	Split  bool
	Err    error
	Note   string
	Lanes  map[extractionMode]adaptiveExtractionOutcome
}

// partialExtractionError marks an extraction unit whose useful, validated
// output must survive even though a peer lane failed. Callers may count the
// unit as usable while still exposing the underlying failure as a warning.
type partialExtractionError struct{ err error }

func (e *partialExtractionError) Error() string {
	return "部分 lane 已产生有效结果，但同批其它 lane 失败: " + e.err.Error()
}

func (e *partialExtractionError) Unwrap() error { return e.err }

type extractionPacketInvoke func(context.Context, extractionMode, extractionPacket) (*repollm.ChatResult, error)

type laneCallOutcome struct {
	result    *extractionResult
	err       error
	saturated bool
}

// extractionSaturationMaxDepth 单 lane 输出饱和时的最大递归二分深度。
// 每次二分都把章节包切成两半，输出空间随输入缩小而收缩；
// 达到深度上限后保留已抢救结果并按系统提示（info）说明，不再无限调用。
const extractionSaturationMaxDepth = 4

func runAdaptivePacketExtraction(ctx context.Context, packet extractionPacket, maxOutput int, invoke extractionPacketInvoke) adaptiveExtractionOutcome {
	if err := ctx.Err(); err != nil {
		lanes := map[extractionMode]adaptiveExtractionOutcome{
			extractionModeDynamicFields: {Err: err},
			extractionModeClauses:       {Err: err},
		}
		return adaptiveExtractionOutcome{Err: err, Lanes: lanes}
	}
	type laneResult struct {
		mode    extractionMode
		outcome adaptiveExtractionOutcome
	}
	results := make(chan laneResult, 2)
	var wg sync.WaitGroup
	for _, mode := range []extractionMode{extractionModeDynamicFields, extractionModeClauses} {
		mode := mode
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- laneResult{mode: mode, outcome: runAdaptiveExtractionLane(ctx, packet, maxOutput, mode, invoke)}
		}()
	}
	wg.Wait()
	close(results)

	var all []*extractionResult
	var errs []error
	var notes []string
	split := false
	lanes := make(map[extractionMode]adaptiveExtractionOutcome, 2)
	for lane := range results {
		lanes[lane.mode] = lane.outcome
		if lane.outcome.Result != nil {
			all = append(all, lane.outcome.Result)
		}
		if lane.outcome.Err != nil {
			errs = append(errs, lane.outcome.Err)
		}
		if lane.outcome.Note != "" {
			notes = append(notes, lane.outcome.Note)
		}
		split = split || lane.outcome.Split
	}
	if len(all) == 0 {
		return adaptiveExtractionOutcome{Split: split, Err: errors.Join(errs...), Note: strings.Join(notes, "; "), Lanes: lanes}
	}
	var outcomeErr error
	if joined := errors.Join(errs...); joined != nil {
		outcomeErr = &partialExtractionError{err: joined}
	}
	return adaptiveExtractionOutcome{Result: mergeExtractionResults(all...), Split: split, Err: outcomeErr, Note: strings.Join(notes, "; "), Lanes: lanes}
}

func runAdaptiveExtractionLane(ctx context.Context, packet extractionPacket, maxOutput int, mode extractionMode, invoke extractionPacketInvoke) adaptiveExtractionOutcome {
	return runAdaptiveExtractionLaneDepth(ctx, packet, maxOutput, mode, invoke, 0)
}

func runAdaptiveExtractionLaneDepth(ctx context.Context, packet extractionPacket, maxOutput int, mode extractionMode, invoke extractionPacketInvoke, depth int) adaptiveExtractionOutcome {
	initial := runExtractionLaneCall(ctx, packet, maxOutput, mode, invoke)
	if !initial.saturated {
		return adaptiveExtractionOutcome{Result: initial.result, Err: initial.err}
	}
	if depth >= extractionSaturationMaxDepth {
		// 已达最大二分深度：保留本次已抢救结果；只要还有可用输出就不视为失败，
		// 仅以系统提示说明可能未完整枚举，避免大章节反复触发“章节提取失败”告警。
		if initial.result != nil {
			return adaptiveExtractionOutcome{Result: initial.result, Split: true, Note: fmt.Sprintf("%s 输出达到上限，已保留已提取结果，剩余内容可能未完整枚举", extractionModeName(mode))}
		}
		return adaptiveExtractionOutcome{Result: nil, Split: true, Err: errors.Join(initial.err, fmt.Errorf("%s 输出饱和且无可用结果", extractionModeName(mode)))}
	}

	left, right, ok := bisectExtractionPacket(packet)
	if !ok {
		// 章节包只剩单个块/表（无法继续二分）：保留已提取结果，不再按失败上报。
		if initial.result != nil {
			return adaptiveExtractionOutcome{Result: initial.result, Split: true, Note: fmt.Sprintf("%s 输出达到上限且章节包无法继续拆分，已保留已提取结果", extractionModeName(mode))}
		}
		return adaptiveExtractionOutcome{Result: nil, Split: true, Err: errors.Join(initial.err, fmt.Errorf("%s 输出饱和且无可用结果", extractionModeName(mode)))}
	}

	type halfResult struct{ outcome adaptiveExtractionOutcome }
	halves := make(chan halfResult, 2)
	var wg sync.WaitGroup
	for _, half := range []extractionPacket{left, right} {
		half := half
		wg.Add(1)
		go func() {
			defer wg.Done()
			halves <- halfResult{outcome: runAdaptiveExtractionLaneDepth(ctx, half, maxOutput, mode, invoke, depth+1)}
		}()
	}
	wg.Wait()
	close(halves)

	results := make([]*extractionResult, 0, 3)
	errs := make([]error, 0, 3)
	notes := make([]string, 0, 3)
	incomplete := false
	for half := range halves {
		if half.outcome.Result != nil {
			results = append(results, half.outcome.Result)
		}
		if half.outcome.Err != nil {
			errs = append(errs, half.outcome.Err)
			incomplete = true
		}
		if half.outcome.Note != "" {
			notes = append(notes, half.outcome.Note)
		}
	}
	if incomplete && initial.result != nil {
		// Both complete halves are authoritative. Initial salvage is only useful
		// when one half remains incomplete after the one allowed bisection.
		results = append(results, initial.result)
	}
	return adaptiveExtractionOutcome{Result: mergeExtractionResults(results...), Split: true, Err: errors.Join(errs...), Note: strings.Join(notes, "; ")}
}

func runExtractionLaneCall(ctx context.Context, packet extractionPacket, maxOutput int, mode extractionMode, invoke extractionPacketInvoke) laneCallOutcome {
	if err := ctx.Err(); err != nil {
		return laneCallOutcome{err: err}
	}
	response, callErr := invoke(ctx, mode, packet)
	if callErr != nil {
		return laneCallOutcome{err: callErr}
	}
	if response == nil {
		return laneCallOutcome{err: fmt.Errorf("%s LLM 未返回响应", extractionModeName(mode))}
	}
	rawItemCount := rawExtractionLaneItemCount(response.Content, mode)
	parsed, parseErr := decodeExtractionResult(response.Content, mode)
	var strict extractionResult
	if strictErr := decodeStrictJSON(response.Content, &strict); parsed != nil && isUnexpectedEOF(strictErr) {
		parseErr = strictErr
	}
	saturationLimit := 20
	if mode == extractionModeUnified {
		saturationLimit = 40
	}
	return laneCallOutcome{result: parsed, err: parseErr, saturated: shouldSplitExtraction(response, parseErr, maxOutput) || rawItemCount == saturationLimit}
}

// rawExtractionLaneItemCount intentionally runs before lane filtering. A
// provider may cap a response at 20 items even if some of those items are
// invalid for this lane; filtering first would hide saturation and skip the
// one allowed packet bisection.
func rawExtractionLaneItemCount(content string, mode extractionMode) int {
	var object struct {
		Candidates []json.RawMessage `json:"candidates"`
		Clauses    []json.RawMessage `json:"clauses"`
	}
	cleaned := []byte(stripJSONFence(content))
	if err := json.Unmarshal(cleaned, &object); err == nil {
		if mode == extractionModeClauses {
			return len(object.Clauses)
		}
		if mode == extractionModeUnified {
			return len(object.Candidates) + len(object.Clauses)
		}
		return len(object.Candidates)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(cleaned, &items); err != nil {
		return 0
	}
	return len(items)
}

func extractionModeName(mode extractionMode) string {
	if mode == extractionModeClauses {
		return "chapter_clauses"
	}
	if mode == extractionModeUnified {
		return "chapter_extraction"
	}
	return "chapter_dynamic_fields"
}

func bisectExtractionPacket(packet extractionPacket) (extractionPacket, extractionPacket, bool) {
	type unit struct {
		page  int32
		kind  string
		block *packetBlock
		table *packetTable
	}
	units := make([]unit, 0, len(packet.Blocks)+len(packet.Tables))
	for i := range packet.Blocks {
		units = append(units, unit{page: packet.Blocks[i].Page, kind: "block", block: &packet.Blocks[i]})
	}
	for i := range packet.Tables {
		units = append(units, unit{page: packet.Tables[i].PageStart, kind: "table", table: &packet.Tables[i]})
	}
	if len(units) < 2 {
		return extractionPacket{}, extractionPacket{}, false
	}
	sort.SliceStable(units, func(i, j int) bool {
		if units[i].page != units[j].page {
			return units[i].page < units[j].page
		}
		return units[i].kind < units[j].kind
	})
	left, right := packet, packet
	left.Blocks, left.Tables = nil, nil
	right.Blocks, right.Tables = nil, nil
	for i, item := range units {
		target := &left
		if i >= len(units)/2 {
			target = &right
		}
		if item.block != nil {
			target.Blocks = append(target.Blocks, *item.block)
		} else {
			target.Tables = append(target.Tables, *item.table)
		}
	}
	setExtractionPacketPages(&left)
	setExtractionPacketPages(&right)
	return left, right, true
}

func setExtractionPacketPages(packet *extractionPacket) {
	var start, end int32
	for _, block := range packet.Blocks {
		if start == 0 || block.Page < start {
			start = block.Page
		}
		if block.Page > end {
			end = block.Page
		}
	}
	for _, table := range packet.Tables {
		if start == 0 || table.PageStart < start {
			start = table.PageStart
		}
		if table.PageEnd > end {
			end = table.PageEnd
		}
	}
	packet.PageStart, packet.PageEnd = start, end
}

func mergeExtractionResults(results ...*extractionResult) *extractionResult {
	merged := &extractionResult{}
	candidateIndex := make(map[string]int)
	clauseIndex := make(map[string]int)
	for _, result := range results {
		if result == nil {
			continue
		}
		for _, candidate := range result.Candidates {
			key := candidateResultKey(candidate)
			if index, ok := candidateIndex[key]; ok {
				merged.Candidates[index] = arbitrateDuplicateCandidate(merged.Candidates[index], candidate)
				continue
			}
			candidate.EvidenceRefs = unionEvidenceRefs(nil, candidate.EvidenceRefs)
			candidateIndex[key] = len(merged.Candidates)
			merged.Candidates = append(merged.Candidates, candidate)
		}
		for _, clause := range result.Clauses {
			key := clauseResultKey(clause)
			if index, ok := clauseIndex[key]; ok {
				merged.Clauses[index] = arbitrateDuplicateClause(merged.Clauses[index], clause)
				continue
			}
			clause.EvidenceRefs = unionEvidenceRefs(nil, clause.EvidenceRefs)
			clauseIndex[key] = len(merged.Clauses)
			merged.Clauses = append(merged.Clauses, clause)
		}
	}
	sort.Slice(merged.Candidates, func(i, j int) bool {
		return candidateResultKey(merged.Candidates[i]) < candidateResultKey(merged.Candidates[j])
	})
	sort.Slice(merged.Clauses, func(i, j int) bool { return clauseResultKey(merged.Clauses[i]) < clauseResultKey(merged.Clauses[j]) })
	return merged
}

func arbitrateDuplicateCandidate(left, right extractedCandidate) extractedCandidate {
	merged := left
	merged.FieldKey = strings.TrimSpace(left.FieldKey)
	merged.Origin = strings.TrimSpace(left.Origin)
	merged.ExtractStatus = strings.TrimSpace(left.ExtractStatus)
	merged.DisplayValue = strings.TrimSpace(left.DisplayValue)
	merged.DisplayName = preferDescriptiveMetadata(left.DisplayName, right.DisplayName, "")
	merged.CategoryKey = preferAllowedCandidateMetadata(left.CategoryKey, right.CategoryKey, allowedCategories, "other_important")
	merged.ValueType = preferAllowedCandidateMetadata(left.ValueType, right.ValueType, allowedValueTypes, "text")
	if confidenceRank(right.Confidence) > confidenceRank(left.Confidence) ||
		(confidenceRank(right.Confidence) == confidenceRank(left.Confidence) && strings.TrimSpace(right.Confidence) < strings.TrimSpace(left.Confidence)) {
		merged.Confidence = strings.TrimSpace(right.Confidence)
	} else {
		merged.Confidence = strings.TrimSpace(left.Confidence)
	}
	merged.EvidenceRefs = unionEvidenceRefs(left.EvidenceRefs, right.EvidenceRefs)
	return merged
}

func preferAllowedCandidateMetadata(left, right string, allowed map[string]bool, fallback string) string {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	leftAllowed, rightAllowed := allowed[left], allowed[right]
	if leftAllowed != rightAllowed {
		if rightAllowed {
			return right
		}
		return left
	}
	return preferDescriptiveMetadata(left, right, fallback)
}

func arbitrateDuplicateClause(left, right extractedClause) extractedClause {
	merged := left
	merged.Title = strings.TrimSpace(left.Title)
	merged.Content = strings.TrimSpace(left.Content)
	if importanceRank(right.Importance) > importanceRank(left.Importance) ||
		(importanceRank(right.Importance) == importanceRank(left.Importance) && strings.TrimSpace(right.Importance) < strings.TrimSpace(left.Importance)) {
		merged.Importance = strings.TrimSpace(right.Importance)
	} else {
		merged.Importance = strings.TrimSpace(left.Importance)
	}
	merged.EvidenceRefs = unionEvidenceRefs(left.EvidenceRefs, right.EvidenceRefs)
	return merged
}

func preferDescriptiveMetadata(left, right, fallback string) string {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	leftFallback, rightFallback := left == fallback, right == fallback
	if leftFallback != rightFallback {
		if leftFallback {
			return right
		}
		return left
	}
	leftLen, rightLen := len([]rune(left)), len([]rune(right))
	if rightLen > leftLen || (rightLen == leftLen && right < left) {
		return right
	}
	return left
}

func confidenceRank(value string) int {
	switch strings.TrimSpace(value) {
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

func importanceRank(value string) int {
	switch strings.TrimSpace(value) {
	case "critical":
		return 4
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

func candidateResultKey(candidate extractedCandidate) string {
	return strings.Join([]string{strings.TrimSpace(candidate.Origin), strings.TrimSpace(candidate.FieldKey), strings.TrimSpace(candidate.DisplayValue), strings.TrimSpace(candidate.ExtractStatus)}, "\x00")
}

func clauseResultKey(clause extractedClause) string {
	return strings.Join([]string{strings.TrimSpace(clause.Title), strings.TrimSpace(clause.Content)}, "\x00")
}

func unionEvidenceRefs(current, additional []evidenceRef) []evidenceRef {
	unique := make(map[string]evidenceRef, len(current)+len(additional))
	for _, ref := range append(append([]evidenceRef(nil), current...), additional...) {
		ref.Kind = strings.TrimSpace(ref.Kind)
		ref.Ref = strings.TrimSpace(ref.Ref)
		if ref.Kind == "" || ref.Ref == "" {
			continue
		}
		unique[ref.Kind+"\x00"+ref.Ref] = ref
	}
	refs := make([]evidenceRef, 0, len(unique))
	for _, ref := range unique {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		return refs[i].Ref < refs[j].Ref
	})
	return refs
}
