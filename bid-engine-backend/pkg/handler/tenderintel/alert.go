package tenderintel

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// alertView 站内提醒视图。
type alertView struct {
	ID               int64    `json:"id"`
	SubscriptionID   int64    `json:"subscription_id"`
	SubscriptionName string   `json:"subscription_name"`
	NoticeID         int64    `json:"notice_id"`
	NoticeTitle      string   `json:"notice_title"`
	MatchedKeywords  []string `json:"matched_keywords"`
	MatchedReason    string   `json:"matched_reason"`
	IsRead           bool     `json:"is_read"`
	CreatedAt        string   `json:"created_at"`
}

// normalizeIDs 去重并丢弃非法（<=0）的提醒 ID，保持调用方传入顺序。
func normalizeIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ListAlerts 提醒列表（可按订阅收敛）。
func (s *svcImpl) ListAlerts(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	unreadOnly := c.Query("unread_only") == "true" || c.Query("unread_only") == "1"
	// subscription_id 把提醒收敛到单个订阅（订阅卡片的提醒抽屉）；缺省表示全部订阅
	subID := int64(0)
	if raw := c.Query("subscription_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			handler.SendNormalResp(c, entity.ErrCodeParam, "订阅 ID 非法", nil)
			return
		}
		subID = parsed
	}
	pageNum := queryInt(c, "pageNum", 1)
	pageSize := queryInt(c, "pageSize", 20)

	items, total, err := s.repo.ListAlerts(c.Request.Context(), userID, subID, unreadOnly, pageNum, pageSize)
	if err != nil {
		logger.Warnw("查询提醒失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询提醒失败", nil)
		return
	}
	out := make([]alertView, 0, len(items))
	for _, item := range items {
		out = append(out, toAlertView(item))
	}
	handler.SendPageRespV2(c, out, total, pageNum, pageSize)
}

// SetAlertsStatus 标记已读 / 未读（单条、批量，或全部已读）。
//
// all=true 只允许与 is_read=true 组合；取消已读必须显式给出 ids，
// 避免一次请求把全部提醒都变成未读。带上 subscription_id 时，all 只作用于
// 该订阅（订阅抽屉里的“全部已读”不应波及其他订阅）。
func (s *svcImpl) SetAlertsStatus(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req struct {
		IDs            []int64 `json:"ids"`
		All            bool    `json:"all"`
		IsRead         *bool   `json:"is_read"`
		SubscriptionID int64   `json:"subscription_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if req.IsRead == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误：缺少 is_read", nil)
		return
	}
	isRead := *req.IsRead
	subID := req.SubscriptionID
	if subID < 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "订阅 ID 非法", nil)
		return
	}
	ids := normalizeIDs(req.IDs)
	if req.All {
		if !isRead {
			handler.SendNormalResp(c, entity.ErrCodeParam, "取消已读需要指定具体的提醒", nil)
			return
		}
		ids = nil
	} else if len(ids) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请指定要操作的提醒或使用 all=true", nil)
		return
	}
	if err := s.repo.SetAlertsStatus(c.Request.Context(), userID, subID, ids, isRead); err != nil {
		logger.Warnw("更新提醒状态失败", "err", err, "is_read", isRead, "subscription_id", subID)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新提醒状态失败", nil)
		return
	}
	unread, _ := s.repo.CountUnreadAlerts(c.Request.Context(), userID, 0)
	handler.SendOKResp(c, map[string]any{"unread": unread})
}

// DeleteAlerts 删除提醒（硬删除，仅限当前用户自己的提醒）。
func (s *svcImpl) DeleteAlerts(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	ids := normalizeIDs(req.IDs)
	if len(ids) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请指定要删除的提醒", nil)
		return
	}
	deleted, err := s.repo.DeleteAlerts(c.Request.Context(), userID, ids)
	if err != nil {
		logger.Warnw("删除提醒失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除提醒失败", nil)
		return
	}
	unread, _ := s.repo.CountUnreadAlerts(c.Request.Context(), userID, 0)
	handler.SendOKResp(c, map[string]any{"deleted": deleted, "unread": unread})
}

// UnreadCount 未读数（供导航角标与抽屉头部）；可按订阅收敛。
func (s *svcImpl) UnreadCount(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	subID := int64(0)
	if raw := c.Query("subscription_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			handler.SendNormalResp(c, entity.ErrCodeParam, "订阅 ID 非法", nil)
			return
		}
		subID = parsed
	}
	unread, err := s.repo.CountUnreadAlerts(c.Request.Context(), userID, subID)
	if err != nil {
		logger.Warnw("统计未读失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "统计未读失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"unread": unread})
}

// UnreadAlertsFor 首页卡片使用。
func (s *svcImpl) UnreadAlertsFor(ctx context.Context, userID int64) (int64, error) {
	return s.repo.CountUnreadAlerts(ctx, userID, 0)
}

// SubscriptionCountFor 首页卡片使用。
func (s *svcImpl) SubscriptionCountFor(ctx context.Context, userID int64) (int64, error) {
	return s.repo.CountSubscriptions(ctx, userID)
}

// CountTodayNew 首页卡片使用。
func (s *svcImpl) CountTodayNew(ctx context.Context) (int64, error) {
	return s.repo.CountTodayNew(ctx)
}

func toAlertView(item *model.TenderIntelAlert) alertView {
	return alertView{
		ID:               item.ID,
		SubscriptionID:   item.SubscriptionID,
		SubscriptionName: item.SubscriptionName,
		NoticeID:         item.NoticeID,
		NoticeTitle:      item.NoticeTitle,
		MatchedKeywords:  DecodeStringList(item.MatchedKeywords),
		MatchedReason:    item.MatchedReason,
		IsRead:           item.IsRead == 1,
		CreatedAt:        formatTime(item.CreatedAt),
	}
}
