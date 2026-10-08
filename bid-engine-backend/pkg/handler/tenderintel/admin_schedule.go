package tenderintel

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// scheduleView 自动采集任务配置视图。
type scheduleView struct {
	CronExpr  string   `json:"cron_expr"`
	Enabled   bool     `json:"enabled"`
	NextRuns  []string `json:"next_runs"`
	Example   string   `json:"example"`
	UpdatedAt string   `json:"updated_at"`
	UpdatedBy int64    `json:"updated_by"`
}

// GetSchedule 读取自动采集任务配置（超管）。
func (s *svcImpl) GetSchedule(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	cfg, err := s.repo.GetCollectSchedule(c.Request.Context())
	if err != nil {
		logger.Warnw("读取自动采集任务配置失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "读取自动采集任务配置失败", nil)
		return
	}
	handler.SendOKResp(c, scheduleView{
		CronExpr:  cfg.CronExpr,
		Enabled:   cfg.Enabled == 1,
		NextRuns:  NextRunTimes(cfg.CronExpr, 5),
		Example:   "每天 06:00 写作 0 0 6 * * *；每天 06:00 与 18:00 写作 0 0 6,18 * * *",
		UpdatedAt: formatTime(cfg.UpdatedAt),
		UpdatedBy: cfg.UpdatedBy,
	})
}

// UpdateSchedule 保存自动采集任务配置并热生效（超管）。
func (s *svcImpl) UpdateSchedule(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req struct {
		CronExpr *string `json:"cron_expr"`
		Enabled  *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if req.CronExpr == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请填写 cron 表达式", nil)
		return
	}

	// 未显式传 enabled 时保持当前开关状态
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	} else if cfg, err := s.repo.GetCollectSchedule(c.Request.Context()); err == nil {
		enabled = cfg.Enabled == 1
	}

	nextRuns, err := s.saveScheduleConfig(
		c.Request.Context(),
		strings.TrimSpace(*req.CronExpr),
		enabled,
		entity.GetUserIDFromCtx(c),
	)
	if err != nil {
		logger.Warnw("保存自动采集任务配置失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	handler.SendOKResp(c, scheduleView{
		CronExpr: strings.TrimSpace(*req.CronExpr),
		Enabled:  enabled,
		NextRuns: nextRuns,
		Example:  "每天 06:00 写作 0 0 6 * * *；每天 06:00 与 18:00 写作 0 0 6,18 * * *",
	})
}

// parseBoolValue 解析表格里的布尔写法，兼容 0/1、true/false、是/否、启用/停用；
// 空值使用 fallback（例如 enabled 默认启用、needs_browser 默认不需要浏览器）。
func parseBoolValue(raw string, fallback bool) (bool, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "1", "true", "yes", "y", "是", "启用", "开启":
		return true, true
	case "0", "false", "no", "n", "否", "停用", "关闭":
		return false, true
	case "":
		return fallback, true
	default:
		return false, false
	}
}

// parsePriorityValue 解析优先级，空值使用默认值 100。
func parsePriorityValue(raw string) (int32, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 100, true
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 9999 {
		return 0, false
	}
	return int32(n), true
}
