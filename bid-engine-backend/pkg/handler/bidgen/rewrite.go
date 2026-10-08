package bidgen

import (
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	repoLLM "bid-engine/pkg/repo/llm"
)

// llmFeatureTextPolish AI 改写选中片段（feature → ModuleBidGeneration，复用标书生成 LLM 配置）
const llmFeatureTextPolish = "bid_text_polish"

// sseRewriteDelta 改写流式增量（原始文本 chunk）
type sseRewriteDelta struct {
	Text string `json:"text"`
}

// sseRewriteDone 改写完成（完整改写文本）
type sseRewriteDone struct {
	Text string `json:"text"`
}

// Rewrite AI 重写选中片段（SSE，无落库）。
// 仅用于终态文档审阅场景，与“整篇/章节生成”完全独立：不写任务表、不占用生成锁。
func (s *svcImpl) Rewrite(c *gin.Context) {
	var req entity.BidGenRewriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		sendSSEError(c, "请求参数错误")
		return
	}
	if req.ProjectID <= 0 {
		sendSSEError(c, "projectId 不能为空")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		sendSSEError(c, "text 不能为空")
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	ctx := repoLLM.WithUserID(c.Request.Context(), userID)

	// 1. LLM 配置检查（按用户解析）
	if err := s.llm.LlmConfigAvailable(ctx, repoLLM.ModuleBidGeneration); err != nil {
		s.logger.Warnw("AI 改写前置 LLM 配置检查失败", "userID", userID, "err", err)
		code, msg := repoLLM.LLMErrorMeta(err)
		sendSSEErrorWithCode(c, code, msg)
		return
	}

	// 2. 项目状态守卫：生成中禁止改写
	proj, err := s.repo.GetProjectForUser(c, entity.GetUserIDFromCtx(c), req.ProjectID)
	if err != nil || proj == nil {
		sendSSEError(c, "项目不存在")
		return
	}
	if proj.Status == ProjectStatusGenerating {
		sendSSEError(c, "项目正在生成中，请勿改写")
		return
	}

	system, prompt := buildRewritePrompt(req)
	chatReq := &repoLLM.ChatRequest{
		System:         system,
		Prompt:         prompt,
		Temperature:    float64Ptr(0.7),
		ResponseFormat: map[string]string{"type": "text"},
	}
	deltaC, errC := s.llm.ChatOnceStreamByFeature(ctx, llmFeatureTextPolish, chatReq)

	setSSEHeaders(c)
	var buf strings.Builder
	streamDone := false
	var streamErr error
	for !streamDone {
		select {
		case d, ok := <-deltaC:
			if !ok {
				streamDone = true
				break
			}
			buf.WriteString(d.ContentDelta)
			sendSSEEvent(c, "delta", sseRewriteDelta{Text: d.ContentDelta})
		case err := <-errC:
			streamErr = err
			streamDone = true
		case <-ctx.Done():
			// 客户端断开/取消：无落库，直接结束
			return
		}
	}
	if streamErr != nil {
		s.logger.Warnw("AI 改写失败", "err", streamErr)
		sendSSEEvent(c, "error", sseError{Msg: repoLLM.FriendlyMessage(streamErr)})
		return
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		sendSSEEvent(c, "error", sseError{Msg: "改写内容为空"})
		return
	}
	sendSSEEvent(c, "done", sseRewriteDone{Text: text})
}

// buildRewritePrompt 构造改写 prompt：保持原意/专业语气/招标响应性，
// 选中片段非完整句子时扩展为完整句子/若干句；只输出改写后的内容。
func buildRewritePrompt(req entity.BidGenRewriteReq) (string, string) {
	system := `你是资深投标文件审阅与润色专家，精通商务标书写作。
任务：重写用户选中的片段（可能是半句话、整句或若干句），使其更专业、精炼、通顺。

` + bidWritingRules() + `

改写要求：
1. 保持原意、事实、数据与承诺不变，不添加原文没有的资质、业绩、金额或承诺；不得删除原文中对招标要求的响应表述与关键数据。
2. 若选中片段不是完整句子，将其扩展为完整句子（必要时覆盖前后若干句），保证上下文连贯。
3. 只输出改写后的内容本身，不要输出任何解释、前缀、引号或 Markdown 标题。
4. 段落内保持纯文本，不要使用 ### 等标题标记。`

	parts := []string{"请重写以下选中的文本片段。"}
	parts = append(parts, "【选中的片段】"+req.Text)
	if ctx := strings.TrimSpace(req.Context); ctx != "" {
		parts = append(parts, "【所在段落上下文】"+ctx)
	}
	if inst := strings.TrimSpace(req.Instruction); inst != "" {
		parts = append(parts, "【改写要求】"+inst)
	}
	parts = append(parts, "请直接输出改写后的内容：")
	return system, strings.Join(parts, "\n\n")
}
