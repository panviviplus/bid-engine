package bidhub

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// StatsResp 统计响应
type StatsResp struct {
	Count  int64  `json:"count"`
	Period string `json:"period"`
}

// parsePeriod 解析周期参数，返回 startTime, endTime (unix timestamp), periodLabel
func parsePeriod(c *gin.Context) (int64, int64, string) {
	period := c.DefaultQuery("period", "today")
	now := time.Now()

	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix(), "today"
	case "week":
		start := now.AddDate(0, 0, -7)
		return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location()).Unix(), now.Unix(), "week"
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix(), "month"
	case "year":
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix(), "year"
	case "custom":
		startStr := c.DefaultQuery("startDate", "")
		endStr := c.DefaultQuery("endDate", "")
		if startStr != "" && endStr != "" {
			startTs, _ := strconv.ParseInt(startStr, 10, 64)
			endTs, _ := strconv.ParseInt(endStr, 10, 64)
			if startTs > 0 && endTs > 0 {
				return startTs, endTs, "custom"
			}
		}
		// fallback to today if custom params missing
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix(), "today"
	default:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return start.Unix(), now.Unix(), "today"
	}
}

// GetAnalysisStats 获取招标解析 V3 项目统计
// GET /zb/stats/analysis?period=today|week|month|year|custom&startDate=xxx&endDate=xxx
func (s *svcImpl) GetAnalysisStats(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	startTime, endTime, periodLabel := parsePeriod(c)

	count, err := s.bidanalysisRepo.CountProjectsByTime(
		c.Request.Context(), userID,
		time.Unix(startTime, 0), time.Unix(endTime, 0),
	)
	if err != nil {
		logger.Warnw("GetAnalysisStats failed", "err", err)
		handler.SendOKResp(c, StatsResp{Count: 0, Period: periodLabel})
		return
	}

	handler.SendOKResp(c, StatsResp{Count: count, Period: periodLabel})
}

// GetGenerationStats 获取投标生成项目统计（bid_gen_project）
// GET /zb/stats/generation?period=today|week|month|year|custom&startDate=xxx&endDate=xxx
func (s *svcImpl) GetGenerationStats(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	startTime, endTime, periodLabel := parsePeriod(c)

	count, err := s.bidgenRepo.CountProjectsByTime(c, userID, startTime, endTime)
	if err != nil {
		logger.Warnw("GetGenerationStats failed", "err", err)
		handler.SendOKResp(c, StatsResp{Count: 0, Period: periodLabel})
		return
	}

	handler.SendOKResp(c, StatsResp{Count: count, Period: periodLabel})
}
