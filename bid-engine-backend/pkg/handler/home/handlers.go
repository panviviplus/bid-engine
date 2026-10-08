package home

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/handler/tenderintel"
)

const defaultListLimit = 6

func currentUserID(c *gin.Context) (int64, bool) {
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return 0, false
	}
	return userID, true
}

func parseLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultListLimit)))
	if err != nil || limit <= 0 {
		return defaultListLimit
	}
	if limit > 12 {
		return 12
	}
	return limit
}

func parsePeriod(c *gin.Context) (string, time.Time, time.Time, error) {
	period := strings.TrimSpace(c.DefaultQuery("period", "month"))
	now := time.Now()
	startOfDay := func(value time.Time) time.Time {
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
	}
	switch period {
	case "today":
		return period, startOfDay(now), now, nil
	case "week":
		return period, startOfDay(now.AddDate(0, 0, -6)), now, nil
	case "month":
		return period, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), now, nil
	case "year":
		return period, time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()), now, nil
	default:
		return "", time.Time{}, time.Time{}, fmt.Errorf("不支持的统计周期: %s", period)
	}
}

func (s *svcImpl) Stats(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	period, start, end, err := parsePeriod(c)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	modules, err := s.repo.Stats(c.Request.Context(), userID, start, end)
	if err != nil {
		s.logger.Errorw("home stats failed", "userID", userID, "period", period, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "首页统计加载失败，请稍后重试", nil)
		return
	}
	handler.SendOKResp(c, gin.H{
		"period":       period,
		"modules":      modules,
		"tender_intel": s.tenderIntelStats(c, userID),
	})
}

// Stats 返回的 modules 中额外挂载招标情报站卡片数据（全平台共享情报 + 用户个人关注数）。
func (s *svcImpl) tenderIntelStats(c *gin.Context, userID int64) gin.H {
	intel := tenderintel.GetInstance()
	ctx := c.Request.Context()

	todayNew, err := intel.CountTodayNew(ctx)
	if err != nil {
		s.logger.Warnw("home tender intel today new failed", "err", err)
	}
	unread, err := intel.UnreadAlertsFor(ctx, userID)
	if err != nil {
		s.logger.Warnw("home tender intel unread failed", "err", err)
	}
	subscriptions, err := intel.SubscriptionCountFor(ctx, userID)
	if err != nil {
		s.logger.Warnw("home tender intel subscription failed", "err", err)
	}
	return gin.H{
		"today_new":          todayNew,
		"unread_alerts":      unread,
		"subscription_count": subscriptions,
	}
}

func (s *svcImpl) RecentWork(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	items, err := s.repo.RecentWork(c.Request.Context(), userID, parseLimit(c))
	if err != nil {
		s.logger.Errorw("home recent work failed", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "最近工作加载失败，请稍后重试", nil)
		return
	}
	handler.SendOKResp(c, gin.H{"items": items})
}

func (s *svcImpl) Attention(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	items, err := s.repo.Attention(c.Request.Context(), userID, parseLimit(c))
	if err != nil {
		s.logger.Errorw("home attention failed", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "待关注任务加载失败，请稍后重试", nil)
		return
	}
	handler.SendOKResp(c, gin.H{"items": items})
}

func (s *svcImpl) LLMConfigStatus(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	status, err := s.repo.LLMConfigStatus(c.Request.Context(), userID)
	if err != nil {
		s.logger.Errorw("home llm config status failed", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "模型配置状态加载失败，请稍后重试", nil)
		return
	}
	handler.SendOKResp(c, status)
}
