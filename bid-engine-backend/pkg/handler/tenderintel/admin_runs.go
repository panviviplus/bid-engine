package tenderintel

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// RetryRun 重试某个历史批次：按原批次的范围（全部源或指定源）重新发起一轮采集。
//
// 采集任务是不可变的执行历史，因此重试不修改原批次，而是新建一个批次并记录 retry_of，
// 这样失败原因与重试结果都能追溯。
func (s *svcImpl) RetryRun(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	runID := strings.TrimSpace(c.Param("runId"))
	if runID == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "批次 ID 不能为空", nil)
		return
	}

	run, err := s.repo.GetRun(c.Request.Context(), runID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "批次不存在", nil)
		return
	}

	sourceKeys := s.repo.GetRunSourceKeys(run)
	newRunID, err := s.startCollectRound(c.Request.Context(), "manual", sourceKeys, run.RunID)
	if err != nil {
		logger.Warnw("重试采集批次失败", "run_id", runID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "重试失败："+err.Error(), nil)
		return
	}
	logger.Infow("已重试采集批次", "origin_run_id", runID, "new_run_id", newRunID, "scope", run.Scope)
	handler.SendOKResp(c, map[string]any{
		"run_id":       newRunID,
		"retry_of":     runID,
		"scope":        run.Scope,
		"source_keys":  sourceKeys,
		"trigger_type": "manual",
	})
}

// DeleteRun 删除某个批次及其源明细（只删执行历史，已入库公告不受影响）。
func (s *svcImpl) DeleteRun(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	runID := strings.TrimSpace(c.Param("runId"))
	if runID == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "批次 ID 不能为空", nil)
		return
	}

	err := s.repo.DeleteRun(c.Request.Context(), runID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "批次不存在", nil)
		return
	}
	if err != nil {
		logger.Warnw("删除采集批次失败", "run_id", runID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除采集批次失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"run_id": runID})
}
