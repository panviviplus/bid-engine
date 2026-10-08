package bidanalysisv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bid-engine/pkg/db/model"
	repollm "bid-engine/pkg/repo/llm"
)

// extractFixedFields performs the full-document, catalog-owned fixed-field
// pass after the chapter pass. Dynamic fields and clauses remain owned by the
// existing chapter extraction path.
func (s *Service) extractFixedFields(ctx context.Context, projectID, runID, userID int64, blocks []*model.BidAnalysisV3DocumentBlock, tables []*model.BidAnalysisV3SourceTable, sources *evidenceSourceIndex, attempt int32, specs *extractionSpecs) (bool, error) {
	if specs == nil || len(specs.list) == 0 {
		return false, nil
	}
	document, err := buildPlainTextDocument(blocks, tables)
	if err != nil {
		return false, err
	}
	llmCtx := repollm.WithUserID(ctx, userID)
	budget := llmBudget{ContextWindow: 32768, MaxOutput: 8192, InputTokenCapacity: 2000}
	if cfg := repollm.ResolveConfig(llmCtx, llmFeatureFactExtract); cfg != nil {
		budget = resolveLLMBudget(cfg.ContextWindowTokens, cfg.DefaultMaxTokens, llmOutputCapExtraction, s.llmInputCeiling)
	}
	plan, err := planFixedFieldExtraction(specs.list, document, budget.InputTokenCapacity)
	if err != nil {
		return false, err
	}
	mergedCandidates := make([]extractedCandidate, 0, len(specs.list))
	evidenceIssues := make([]fixedFieldEvidenceIssue, 0)
	degraded := false
	for batchIndex, batchPlan := range plan {
		segmentResults := make([]fixedFieldSegmentResult, 0, len(batchPlan.Segments))
		for segmentIndex, segment := range batchPlan.Segments {
			prompt, err := marshalFixedFieldPrompt(segment)
			if err != nil {
				return false, err
			}
			startedAt := time.Now()
			task := &model.BidAnalysisV3StageTask{
				ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting", TaskType: "fixed_fields",
				UnitKey:   fmt.Sprintf("fixed_fields:batch:%02d:segment:%03d:attempt:%02d", batchIndex+1, segmentIndex+1, attempt),
				PageStart: segment.Packet.PageStart, PageEnd: segment.Packet.PageEnd, Status: "running", Attempts: attempt,
				PayloadHash: hashText(prompt), StartedAt: &startedAt,
			}
			if err := s.repo.CreateStageTask(ctx, task); err != nil {
				return false, err
			}
			decisions, callErr := s.callFixedFieldLLM(llmCtx, projectID, runID, task.ID, prompt, budget.MaxOutput, batchPlan.Fields)
			if callErr != nil {
				if statusErr := s.updateStageTaskTerminal(ctx, task.ID, map[string]any{"status": "failed", "last_error": callErr.Error(), "completed_at": time.Now()}); statusErr != nil {
					return false, fmt.Errorf("固定字段抽取失败（%v），且任务状态保存失败: %w", callErr, statusErr)
				}
				// 取消与基础设施故障保持原语义终止；内容级失败整段降级为待核验并继续，
				// 不让单个分段的模型输出问题中断整个解析流程。
				if errors.Is(callErr, context.Canceled) || isLLMStageFatal(callErr) {
					return false, callErr
				}
				degraded = true
				candidates, issues := degradeFixedFieldBatch(batchPlan.Fields, segment, "分段提取失败："+callErr.Error())
				evidenceIssues = append(evidenceIssues, issues...)
				segmentResults = append(segmentResults, fixedFieldSegmentResult{Completed: true, Candidates: candidates})
				continue
			}
			candidates, issues, validationErr := validateFixedFieldDecisions(batchPlan.Fields, decisions, segment)
			if validationErr != nil {
				if statusErr := s.updateStageTaskTerminal(ctx, task.ID, map[string]any{"status": "failed", "last_error": validationErr.Error(), "completed_at": time.Now()}); statusErr != nil {
					return false, fmt.Errorf("固定字段结果校验失败（%v），且任务状态保存失败: %w", validationErr, statusErr)
				}
				// 防御性兜底：校验层原则上不再返回致命错误，若出现则同样降级继续。
				degraded = true
				candidates, issues := degradeFixedFieldBatch(batchPlan.Fields, segment, "分段校验失败："+validationErr.Error())
				evidenceIssues = append(evidenceIssues, issues...)
				segmentResults = append(segmentResults, fixedFieldSegmentResult{Completed: true, Candidates: candidates})
				continue
			}
			evidenceIssues = append(evidenceIssues, issues...)
			resultHash, hashErr := fixedFieldResultHash(candidates)
			if hashErr != nil {
				return false, hashErr
			}
			if err := s.updateStageTaskTerminal(ctx, task.ID, map[string]any{"status": "succeeded", "result_hash": resultHash, "completed_at": time.Now()}); err != nil {
				return false, err
			}
			segmentResults = append(segmentResults, fixedFieldSegmentResult{Completed: true, Candidates: candidates})
		}
		merged, err := mergeFixedFieldDecisions(batchPlan.Fields, segmentResults)
		if err != nil {
			return false, err
		}
		mergedCandidates = append(mergedCandidates, merged...)
	}
	finalStatus := make(map[string]string, len(mergedCandidates))
	for _, candidate := range mergedCandidates {
		finalStatus[candidate.FieldKey] = candidate.ExtractStatus
	}
	absentIssues, anomalyIssues := splitFixedFieldUnresolved(evidenceIssues, finalStatus)
	if len(absentIssues) > 0 {
		// 招标文件本身缺少该字段值（文档不规范）→ 系统提示（info），不参与告警计数。
		fieldKeys := make(map[string]bool, len(absentIssues))
		reasons := make(map[string]int)
		for _, issue := range absentIssues {
			fieldKeys[issue.FieldKey] = true
			reasons[issue.Reason]++
		}
		detailBytes, marshalErr := json.Marshal(map[string]any{
			"occurrences": len(absentIssues),
			"reasons":     reasons,
			"fields":      absentIssues,
		})
		if marshalErr != nil {
			return false, fmt.Errorf("序列化固定字段缺失提示失败: %w", marshalErr)
		}
		detail := string(detailBytes)
		notice := &model.BidAnalysisV3Warning{
			ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting",
			Code: "fixed_field_not_found_in_document", GroupKey: fmt.Sprintf("code:fixed_field_not_found_in_document:attempt:%02d", attempt), Severity: "info",
			Message:    fmt.Sprintf("%d 个固定字段在招标文件中未找到明确值，已降级为待核验（招标文件可能未包含或表述不规范）", len(fieldKeys)),
			DetailJSON: &detail,
		}
		if err := s.repo.AddWarning(ctx, notice); err != nil {
			return false, err
		}
	}
	if len(anomalyIssues) > 0 {
		// 提取/校验异常（证据无法定位、空值、语义矛盾、分段失败等）→ 解析告警。
		fieldKeys := make(map[string]bool, len(anomalyIssues))
		reasons := make(map[string]int)
		for _, issue := range anomalyIssues {
			fieldKeys[issue.FieldKey] = true
			reasons[issue.Reason]++
		}
		detailBytes, marshalErr := json.Marshal(map[string]any{
			"occurrences": len(anomalyIssues),
			"reasons":     reasons,
			"fields":      anomalyIssues,
		})
		if marshalErr != nil {
			return false, fmt.Errorf("序列化固定字段证据告警失败: %w", marshalErr)
		}
		detail := string(detailBytes)
		warning := &model.BidAnalysisV3Warning{
			ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting",
			Code: "fixed_field_evidence_invalid", GroupKey: fmt.Sprintf("code:fixed_field_evidence_invalid:attempt:%02d", attempt), Severity: "warning",
			Message:    fmt.Sprintf("%d 个固定字段提取异常，已降级为待核验，其他字段已正常保留", len(fieldKeys)),
			DetailJSON: &detail,
		}
		if err := s.repo.AddWarning(ctx, warning); err != nil {
			return false, err
		}
	}
	encoded, err := json.Marshal(mergedCandidates)
	if err != nil {
		return false, fmt.Errorf("序列化固定字段归并结果失败: %w", err)
	}
	startedAt := time.Now()
	mergeTask := &model.BidAnalysisV3StageTask{
		ProjectID: projectID, RunID: runID, Stage: "chapter_fact_extracting", TaskType: "fixed_fields_merge",
		UnitKey: fmt.Sprintf("fixed_fields:merge:attempt:%02d", attempt), PageStart: document.Packet.PageStart, PageEnd: document.Packet.PageEnd,
		Status: "running", Attempts: attempt, PayloadHash: hashText(string(encoded)), StartedAt: &startedAt,
	}
	if err := s.repo.CreateStageTask(ctx, mergeTask); err != nil {
		return false, err
	}
	if err := s.validateAndPersistExtraction(ctx, projectID, runID, nil, mergeTask.ID, document.Packet, &extractionResult{Candidates: mergedCandidates}, specs, sources); err != nil {
		if statusErr := s.updateStageTaskTerminal(ctx, mergeTask.ID, map[string]any{"status": "failed", "last_error": err.Error(), "completed_at": time.Now()}); statusErr != nil {
			return false, fmt.Errorf("固定字段归并落库失败（%v），且任务状态保存失败: %w", err, statusErr)
		}
		return false, err
	}
	if err := s.updateStageTaskTerminal(ctx, mergeTask.ID, map[string]any{"status": "succeeded", "result_hash": hashText(string(encoded)), "completed_at": time.Now()}); err != nil {
		return false, err
	}
	// 仅提取/校验异常才把阶段标记为 partial；文档本身缺少字段值不视为失败。
	return len(anomalyIssues) > 0 || degraded, nil
}

// splitFixedFieldUnresolved 把最终为 not_found 的固定字段问题拆成两类：
//   - absent：招标文件本身缺少该字段值（干净 not_found 或模型遗漏决策），
//     属于文档不规范，按系统提示（info）处理；
//   - anomaly：提取/校验异常（证据无法定位、空值、语义矛盾、分段失败等），
//     按解析告警（warning）处理。
//
// 同一字段只要出现任意异常即归入 anomaly，避免异常被“文档缺失”掩盖。
func splitFixedFieldUnresolved(issues []fixedFieldEvidenceIssue, finalStatus map[string]string) (absent, anomaly []fixedFieldEvidenceIssue) {
	absentByField := make(map[string]bool)
	anomalyByField := make(map[string]bool)
	for _, issue := range issues {
		if finalStatus[issue.FieldKey] != "not_found" {
			continue
		}
		if issue.Reason == fixedFieldAbsentReason || issue.Reason == fixedFieldOmittedReason {
			absentByField[issue.FieldKey] = true
		} else {
			anomalyByField[issue.FieldKey] = true
		}
	}
	for fieldKey := range anomalyByField {
		delete(absentByField, fieldKey)
	}
	for _, issue := range issues {
		if finalStatus[issue.FieldKey] != "not_found" {
			continue
		}
		if absentByField[issue.FieldKey] {
			absent = append(absent, issue)
		} else if anomalyByField[issue.FieldKey] {
			anomaly = append(anomaly, issue)
		}
	}
	return absent, anomaly
}

func (s *Service) callFixedFieldLLM(ctx context.Context, projectID, runID, taskID int64, prompt string, maxOutput int, specs []systemFieldSpec) ([]fixedFieldDecision, error) {
	temperature := 0.1
	req := &repollm.ChatRequest{
		System: buildFixedFieldSystemPrompt(specs), Prompt: prompt, Temperature: &temperature, MaxTokens: &maxOutput,
		ResponseFormat: fixedFieldResponseFormat(specs),
	}
	result, err := s.invokeStructuredLLM(ctx, projectID, runID, taskID, llmFeatureFactExtract, prompt, req)
	if err != nil {
		return nil, err
	}
	return decodeFixedFieldResult(result.Content)
}
