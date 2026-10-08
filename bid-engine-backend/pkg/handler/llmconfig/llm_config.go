package llmconfig

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	repoLLM "bid-engine/pkg/repo/llm"
)

// ── 请求/响应 ────────────────────────────────────────────────────

type llmConfigField struct {
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	Model               string `json:"model"`
	EndpointPath        string `json:"endpoint_path"`
	ContextWindowTokens int32  `json:"context_window_tokens"`
	MaxOutputTokens     int32  `json:"max_output_tokens"`
}

type moduleItem struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type llmConfigResponse map[string]*llmConfigField // key: module name; null = 未独立配置

// ── GET /system/modules ─────────────────────────────────────────

// Modules 获取业务功能模块列表（不含全局 all）
func (s *svcImpl) Modules(c *gin.Context) {
	items := make([]moduleItem, 0, len(repoLLM.BusinessModules))
	for _, key := range repoLLM.BusinessModules {
		items = append(items, moduleItem{
			Key:  key,
			Name: repoLLM.ModuleNames[key],
		})
	}
	handler.SendOKResp(c, items)
}

// ── GET /system/llm-config ──────────────────────────────────────

func (s *svcImpl) Get(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}

	q := query.Use(storage.GetDB())
	result := make(llmConfigResponse)

	for _, mod := range repoLLM.AllModules {
		cfg, err := q.UserLlmConfig.WithContext(c.Request.Context()).
			Where(q.UserLlmConfig.UserID.Eq(userID), q.UserLlmConfig.Module.Eq(mod)).
			First()
		if err != nil || cfg == nil {
			result[mod] = nil
			continue
		}
		result[mod] = &llmConfigField{
			BaseURL: cfg.BaseURL, APIKey: repoLLM.MaskAPIKey(cfg.APIKey), Model: cfg.Model,
			EndpointPath: cfg.EndpointPath, ContextWindowTokens: cfg.ContextWindowTokens,
			MaxOutputTokens: cfg.MaxOutputTokens,
		}
	}

	handler.SendOKResp(c, result)
}

// ── PUT /system/llm-config/:module ──────────────────────────────

func (s *svcImpl) SaveModule(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}

	mod := c.Param("module")
	if !repoLLM.IsModuleValid(mod) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "非法模块: "+mod, nil)
		return
	}

	var field llmConfigField
	if err := c.ShouldBindJSON(&field); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误: "+err.Error(), nil)
		return
	}
	if field.ContextWindowTokens == 0 {
		field.ContextWindowTokens = 32768
	}
	if field.MaxOutputTokens == 0 {
		field.MaxOutputTokens = 8192
	}
	if field.ContextWindowTokens < 8192 || field.MaxOutputTokens < 512 || field.MaxOutputTokens+4096 > field.ContextWindowTokens {
		handler.SendNormalResp(c, entity.ErrCodeParam, "上下文窗口与最大输出配置无效：上下文至少 8192，输出至少 512，并需预留 4096 tokens 输入与安全余量", nil)
		return
	}

	q := query.Use(storage.GetDB())
	db := storage.GetDB()
	now := time.Now().Unix()
	ctx := c.Request.Context()

	existing, _ := q.UserLlmConfig.WithContext(ctx).
		Where(q.UserLlmConfig.UserID.Eq(userID), q.UserLlmConfig.Module.Eq(mod)).
		First()

	if existing != nil {
		// 全量覆盖该模块（api_key 为脱敏值时保留原值；空串=清掉 key）
		apiKey := field.APIKey
		if repoLLM.IsMasked(apiKey) {
			apiKey = existing.APIKey
		}
		updates := map[string]interface{}{
			"base_url":              field.BaseURL,
			"api_key":               apiKey,
			"model":                 field.Model,
			"endpoint_path":         field.EndpointPath,
			"context_window_tokens": field.ContextWindowTokens,
			"max_output_tokens":     field.MaxOutputTokens,
			"update_time":           now,
		}
		if _, err := q.UserLlmConfig.WithContext(ctx).
			Where(q.UserLlmConfig.ID.Eq(existing.ID)).
			Updates(updates); err != nil {
			s.logger.Warnw("SaveLLMConfig update failed", "userID", userID, "module", mod, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeParam, "保存失败: "+err.Error(), nil)
			return
		}
	} else {
		row := &model.UserLlmConfig{
			UserID:              userID,
			Module:              mod,
			BaseURL:             field.BaseURL,
			APIKey:              field.APIKey,
			Model:               field.Model,
			EndpointPath:        field.EndpointPath,
			ContextWindowTokens: field.ContextWindowTokens,
			MaxOutputTokens:     field.MaxOutputTokens,
			CreateTime:          now,
			UpdateTime:          now,
		}
		if err := db.Create(row).Error; err != nil {
			s.logger.Warnw("SaveLLMConfig insert failed", "userID", userID, "module", mod, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeParam, "保存失败: "+err.Error(), nil)
			return
		}
	}

	repoLLM.InvalidateCache(userID)

	handler.SendOKResp(c, "保存成功")
}

// ── DELETE /system/llm-config/:module ───────────────────────────

func (s *svcImpl) ClearModule(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}

	mod := c.Param("module")
	if !repoLLM.IsModuleValid(mod) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "非法模块: "+mod, nil)
		return
	}

	q := query.Use(storage.GetDB())
	if _, err := q.UserLlmConfig.WithContext(c.Request.Context()).
		Where(q.UserLlmConfig.UserID.Eq(userID), q.UserLlmConfig.Module.Eq(mod)).
		Delete(); err != nil {
		s.logger.Warnw("ClearModule failed", "userID", userID, "module", mod, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, "清空失败: "+err.Error(), nil)
		return
	}

	repoLLM.InvalidateCache(userID)

	handler.SendOKResp(c, "已清空")
}

// ── DELETE /system/llm-config ───────────────────────────────────

func (s *svcImpl) Clear(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}

	q := query.Use(storage.GetDB())
	if _, err := q.UserLlmConfig.WithContext(c.Request.Context()).
		Where(q.UserLlmConfig.UserID.Eq(userID)).
		Delete(); err != nil {
		s.logger.Warnw("ClearLLMConfig failed", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, "清空失败: "+err.Error(), nil)
		return
	}

	repoLLM.InvalidateCache(userID)

	handler.SendOKResp(c, "已清空")
}

// ── GET /system/llm-config/exist ───────────────────────────────

// Exist 校验指定业务模块的 LLM 配置是否存在（纯 DB 查询，无 LLM 调用，不消耗 Token）
// 遵循“模块 → 全局”策略：module 行或 module=all 行任一存在即为已配置。
// GET /system/llm-config/exist?module=bid_generation → {"exists": true|false}
func (s *svcImpl) Exist(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID == 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "未登录", nil)
		return
	}
	module := strings.TrimSpace(c.DefaultQuery("module", ""))
	if module == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "module 不能为空", nil)
		return
	}
	if !repoLLM.IsModuleValid(module) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "非法模块: "+module, nil)
		return
	}
	err := repoLLM.GetInstance().LlmConfigExist(
		repoLLM.WithUserID(c.Request.Context(), userID),
		module,
	)
	exists := err == nil
	handler.SendOKResp(c, gin.H{"module": module, "exists": exists})
}
