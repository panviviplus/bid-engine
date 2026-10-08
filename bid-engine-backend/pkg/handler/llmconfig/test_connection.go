package llmconfig

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	repoLLM "bid-engine/pkg/repo/llm"
)

// testAIRequest 测试连接请求（提交当前表单未保存的配置）
type testAIRequest struct {
	Module              string `json:"module"`
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	Model               string `json:"model"`
	EndpointPath        string `json:"endpoint_path"`
	ContextWindowTokens int    `json:"context_window_tokens"`
	MaxOutputTokens     int    `json:"max_output_tokens"`
}

// ── POST /system/test-ai ────────────────────────────────────────

// TestConnection 用提交的配置发送极简 prompt 做真实连通性测试（不落库、不写用量记录）。
// 成功返回 {success, model, latency_ms, reply}；失败返回友好中文错误（不含原始堆栈/细节）。
func (s *svcImpl) TestConnection(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}

	var req testAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误: "+err.Error(), nil)
		return
	}

	mod := strings.TrimSpace(req.Module)
	if !repoLLM.IsModuleValid(mod) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "非法模块: "+mod, nil)
		return
	}

	// api_key 为脱敏值时按 module 还原库内真实 key（与 SaveModule 一致）
	apiKey := strings.TrimSpace(req.APIKey)
	if repoLLM.IsMasked(apiKey) {
		q := query.Use(storage.GetDB())
		existing, err := q.UserLlmConfig.WithContext(c.Request.Context()).
			Where(q.UserLlmConfig.UserID.Eq(userID), q.UserLlmConfig.Module.Eq(mod)).
			First()
		if err != nil || existing == nil {
			handler.SendNormalResp(c, entity.ErrCodeParam, "API Key 已脱敏且无法还原，请重新输入完整 API Key", nil)
			return
		}
		apiKey = existing.APIKey
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	model := strings.TrimSpace(req.Model)
	endpoint := strings.TrimSpace(req.EndpointPath)
	if endpoint == "" {
		endpoint = "/chat/completions"
	}
	if baseURL == "" || apiKey == "" || model == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请填写完整的配置后再测试", nil)
		return
	}

	cfg := &repoLLM.ProviderConfig{
		BaseURL:             baseURL,
		EndpointPath:        endpoint,
		APIKey:              apiKey,
		Model:               model,
		TimeoutSeconds:      30,
		ContextWindowTokens: req.ContextWindowTokens,
		DefaultMaxTokens:    req.MaxOutputTokens,
	}

	ctx := c.Request.Context()
	result, err := repoLLM.GetInstance().TestConnection(ctx, cfg, "", 30*time.Second)
	if err != nil {
		s.logger.Warnw("TestConnection failed",
			"userID", userID,
			"module", mod,
			"base_url", baseURL,
			"model", model,
			"err", err,
		)
		// 只返回分类后的友好文案与错误码，原始错误细节仅进服务端日志
		code, msg := repoLLM.LLMErrorMeta(err)
		handler.SendNormalResp(c, entity.ErrCodeModel, msg, gin.H{"llm_code": code})
		return
	}

	handler.SendOKResp(c, gin.H{
		"success":    true,
		"model":      result.Model,
		"latency_ms": result.Latency.Milliseconds(),
		"reply":      result.Reply,
	})
}
