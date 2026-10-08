package sysllm

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/sysllm"
)

// itemView 全局模型配置视图（api_key 脱敏）。
type itemView struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	Model               string `json:"model"`
	EndpointPath        string `json:"endpoint_path"`
	ContextWindowTokens int32  `json:"context_window_tokens"`
	MaxOutputTokens     int32  `json:"max_output_tokens"`
	Sort                int32  `json:"sort"`
	Complete            bool   `json:"complete"`
	UpdateTime          string `json:"update_time"`
}

// itemRequest 新增 / 修改请求体。
type itemRequest struct {
	Name                string `json:"name"`
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	Model               string `json:"model"`
	EndpointPath        string `json:"endpoint_path"`
	ContextWindowTokens int32  `json:"context_window_tokens"`
	MaxOutputTokens     int32  `json:"max_output_tokens"`
	Sort                int32  `json:"sort"`
}

// List 配置列表。
func (s *svcImpl) List(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	items, err := s.repo.List(c.Request.Context())
	if err != nil {
		logger.Warnw("查询全局模型配置失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询全局模型配置失败", nil)
		return
	}
	out := make([]itemView, 0, len(items))
	for _, item := range items {
		out = append(out, toItemView(item))
	}
	handler.SendOKResp(c, out)
}

// Resolve 返回当前生效的配置（脱敏）。
func (s *svcImpl) Resolve(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	_, item, err := s.repo.Resolve(c.Request.Context())
	if err != nil {
		handler.SendOKResp(c, map[string]any{
			"source":  "none",
			"message": "当前没有可用的全局模型配置，后台任务将回退到配置文件默认模型",
		})
		return
	}
	if item == nil {
		// 命中配置文件兜底
		handler.SendOKResp(c, map[string]any{
			"source":  "config_file",
			"message": "当前使用 conf/llm-config.yml 中的默认模型",
		})
		return
	}
	handler.SendOKResp(c, map[string]any{
		"source": "database",
		"item":   toItemView(item),
	})
}

// Create 新增配置。
func (s *svcImpl) Create(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	var req itemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.APIKey) == "" || strings.TrimSpace(req.Model) == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "base_url / api_key / model 均为必填", nil)
		return
	}
	item := &model.SystemLlmConfig{
		Name:                strings.TrimSpace(req.Name),
		BaseURL:             strings.TrimSpace(req.BaseURL),
		APIKey:              strings.TrimSpace(req.APIKey),
		Model:               strings.TrimSpace(req.Model),
		EndpointPath:        strings.TrimSpace(req.EndpointPath),
		ContextWindowTokens: req.ContextWindowTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		Sort:                req.Sort,
	}
	if item.Sort == 0 {
		item.Sort = 100
	}
	if err := s.repo.Create(c.Request.Context(), item); err != nil {
		logger.Warnw("新增全局模型配置失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "新增全局模型配置失败", nil)
		return
	}
	handler.SendOKResp(c, toItemView(item))
}

// Update 修改配置。
func (s *svcImpl) Update(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "配置 ID 非法", nil)
		return
	}
	var req itemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	item := &model.SystemLlmConfig{
		ID:                  id,
		Name:                strings.TrimSpace(req.Name),
		BaseURL:             strings.TrimSpace(req.BaseURL),
		APIKey:              strings.TrimSpace(req.APIKey),
		Model:               strings.TrimSpace(req.Model),
		EndpointPath:        strings.TrimSpace(req.EndpointPath),
		ContextWindowTokens: req.ContextWindowTokens,
		MaxOutputTokens:     req.MaxOutputTokens,
		Sort:                req.Sort,
	}
	if err := s.repo.Update(c.Request.Context(), item); err != nil {
		logger.Warnw("更新全局模型配置失败", "id", id, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新全局模型配置失败", nil)
		return
	}
	updated, err := s.repo.Get(c.Request.Context(), id)
	if err != nil {
		handler.SendOKResp(c, map[string]any{"id": id})
		return
	}
	handler.SendOKResp(c, toItemView(updated))
}

// Delete 删除配置。
func (s *svcImpl) Delete(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "配置 ID 非法", nil)
		return
	}
	if err := s.repo.Delete(c.Request.Context(), id); err != nil {
		logger.Warnw("删除全局模型配置失败", "id", id, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除全局模型配置失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"id": id})
}

// Reorder 批量调整顺序（按传入数组顺序，sort 依次为 10/20/30…）。
func (s *svcImpl) Reorder(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误：需要 ids 数组", nil)
		return
	}
	if err := s.repo.Reorder(c.Request.Context(), req.IDs); err != nil {
		logger.Warnw("调整全局模型配置顺序失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "调整顺序失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"ids": req.IDs})
}

// TestConnection 连通性测试：可以直接测试已保存的配置（id），也可以测试提交的临时配置。
func (s *svcImpl) TestConnection(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	var req struct {
		ID                  int64  `json:"id"`
		Name                string `json:"name"`
		BaseURL             string `json:"base_url"`
		APIKey              string `json:"api_key"`
		Model               string `json:"model"`
		EndpointPath        string `json:"endpoint_path"`
		ContextWindowTokens int32  `json:"context_window_tokens"`
		MaxOutputTokens     int32  `json:"max_output_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}

	var cfg *model.SystemLlmConfig
	if req.ID > 0 {
		saved, err := s.repo.Get(c.Request.Context(), req.ID)
		if err != nil {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "配置不存在", nil)
			return
		}
		cfg = saved
		if strings.TrimSpace(req.APIKey) != "" {
			cfg.APIKey = strings.TrimSpace(req.APIKey)
		}
		if strings.TrimSpace(req.BaseURL) != "" {
			cfg.BaseURL = strings.TrimSpace(req.BaseURL)
		}
		if strings.TrimSpace(req.Model) != "" {
			cfg.Model = strings.TrimSpace(req.Model)
		}
		if strings.TrimSpace(req.EndpointPath) != "" {
			cfg.EndpointPath = strings.TrimSpace(req.EndpointPath)
		}
	} else {
		cfg = &model.SystemLlmConfig{
			Name:                strings.TrimSpace(req.Name),
			BaseURL:             strings.TrimSpace(req.BaseURL),
			APIKey:              strings.TrimSpace(req.APIKey),
			Model:               strings.TrimSpace(req.Model),
			EndpointPath:        strings.TrimSpace(req.EndpointPath),
			ContextWindowTokens: req.ContextWindowTokens,
			MaxOutputTokens:     req.MaxOutputTokens,
		}
	}

	result, err := s.llm.TestConnection(c.Request.Context(), s.repo.ToProviderConfig(cfg), "", 20*time.Second)
	if err != nil {
		s.logger.Warnw("全局模型配置连通性测试失败", append(entity.Ctx(c), "err", err)...)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "连通性测试失败："+err.Error(), map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	handler.SendOKResp(c, map[string]any{
		"ok":         true,
		"model":      result.Model,
		"latency_ms": result.Latency.Milliseconds(),
		"reply":      result.Reply,
	})
}

func requireSuperAdmin(c *gin.Context) bool {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return false
	}
	if !entity.IsSuperAdmin(c) {
		handler.SendForbiddenResp(c)
		return false
	}
	return true
}

func toItemView(item *model.SystemLlmConfig) itemView {
	return itemView{
		ID:                  item.ID,
		Name:                item.Name,
		BaseURL:             item.BaseURL,
		APIKey:              maskAPIKey(item.APIKey),
		Model:               item.Model,
		EndpointPath:        item.EndpointPath,
		ContextWindowTokens: item.ContextWindowTokens,
		MaxOutputTokens:     item.MaxOutputTokens,
		Sort:                item.Sort,
		Complete:            sysllm.IsComplete(item),
		UpdateTime:          formatUnix(item.UpdateTime),
	}
}

// maskAPIKey 脱敏：保留前 4 位与后 4 位。
func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", 6) + key[len(key)-4:]
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}
