package tenderintel

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// subscriptionView 订阅规则视图。
type subscriptionView struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Keywords      []string `json:"keywords"`
	MatchMode     string   `json:"match_mode"`
	Industries    []string `json:"industries"`
	IndustryNames []string `json:"industry_names"`
	Regions       []string `json:"regions"`
	NoticeTypes   []string `json:"notice_types"`
	BudgetMin     *float64 `json:"budget_min"`
	BudgetMax     *float64 `json:"budget_max"`
	Enabled       bool     `json:"enabled"`
	LastMatchedAt string   `json:"last_matched_at"`
	MatchedCount  int32    `json:"matched_count"`
	UnreadCount   int64    `json:"unread_count"`
	CreatedAt     string   `json:"created_at"`
}

// subscriptionRequest 新建 / 修改订阅的请求体。
type subscriptionRequest struct {
	Name        string   `json:"name"`
	Keywords    []string `json:"keywords"`
	MatchMode   string   `json:"match_mode"`
	Industries  []string `json:"industries"`
	Regions     []string `json:"regions"`
	NoticeTypes []string `json:"notice_types"`
	BudgetMin   *float64 `json:"budget_min"`
	BudgetMax   *float64 `json:"budget_max"`
	Enabled     *bool    `json:"enabled"`
}

// ListSubscriptions 我的订阅列表。
func (s *svcImpl) ListSubscriptions(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	items, err := s.repo.ListSubscriptions(c.Request.Context(), userID)
	if err != nil {
		logger.Warnw("查询订阅失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询订阅失败", nil)
		return
	}
	out := make([]subscriptionView, 0, len(items))
	// 未读数用一次聚合查询取回（避免逐条 COUNT 形成 N+1）；聚合失败不阻断列表，
	// 此时未读统一显示为 0，抽屉里仍能拿到真实数据。
	unreadBySub, err := s.repo.CountUnreadAlertsBySubscription(c.Request.Context(), userID)
	if err != nil {
		logger.Warnw("统计订阅未读数失败", "err", err)
		unreadBySub = nil
	}
	for _, item := range items {
		view := toSubscriptionView(item)
		view.UnreadCount = unreadBySub[item.ID]
		out = append(out, view)
	}
	handler.SendOKResp(c, out)
}

// CreateSubscription 新建订阅。
func (s *svcImpl) CreateSubscription(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req subscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	keywords := trimNonEmpty(req.Keywords)
	industries := NormalizeIndustries(req.Industries)
	regions := trimNonEmpty(req.Regions)
	noticeTypes := filterKnownNoticeTypes(req.NoticeTypes)
	if len(keywords) == 0 && len(industries) == 0 && len(regions) == 0 && len(noticeTypes) == 0 &&
		req.BudgetMin == nil && req.BudgetMax == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "订阅条件不能为空：至少填写关键词、行业、地区、公告类型或预算区间之一", nil)
		return
	}

	item := &model.TenderIntelSubscription{
		UserID:      userID,
		Name:        defaultSubscriptionName(req.Name, keywords, industries),
		Keywords:    EncodeStringList(keywords),
		MatchMode:   normalizeMatchMode(req.MatchMode),
		Industries:  EncodeStringList(industries),
		Regions:     EncodeStringList(regions),
		NoticeTypes: EncodeStringList(noticeTypes),
		BudgetMin:   req.BudgetMin,
		BudgetMax:   req.BudgetMax,
		Enabled:     1,
	}
	if req.Enabled != nil && !*req.Enabled {
		item.Enabled = 0
	}
	if err := s.repo.CreateSubscription(c.Request.Context(), item); err != nil {
		logger.Warnw("创建订阅失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "创建订阅失败", nil)
		return
	}
	// 静默回溯近 30 天在架情报：走独立匹配队列，用户侧不感知、不阻塞创建接口
	s.enqueueSubscriptionBackfill(c.Request.Context(), item)
	handler.SendOKResp(c, toSubscriptionView(item))
}

// UpdateSubscription 修改订阅。
func (s *svcImpl) UpdateSubscription(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "订阅 ID 非法", nil)
		return
	}
	current, err := s.repo.GetSubscription(c.Request.Context(), userID, id)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "订阅不存在", nil)
		return
	}
	var req subscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	keywords := trimNonEmpty(req.Keywords)
	industries := NormalizeIndustries(req.Industries)
	regions := trimNonEmpty(req.Regions)
	noticeTypes := filterKnownNoticeTypes(req.NoticeTypes)
	if len(keywords) == 0 && len(industries) == 0 && len(regions) == 0 && len(noticeTypes) == 0 &&
		req.BudgetMin == nil && req.BudgetMax == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "订阅条件不能为空", nil)
		return
	}
	current.Name = defaultSubscriptionName(req.Name, keywords, industries)
	current.Keywords = EncodeStringList(keywords)
	current.MatchMode = normalizeMatchMode(req.MatchMode)
	current.Industries = EncodeStringList(industries)
	current.Regions = EncodeStringList(regions)
	current.NoticeTypes = EncodeStringList(noticeTypes)
	current.BudgetMin = req.BudgetMin
	current.BudgetMax = req.BudgetMax
	if req.Enabled != nil {
		if *req.Enabled {
			current.Enabled = 1
		} else {
			current.Enabled = 0
		}
	}
	if err := s.repo.UpdateSubscription(c.Request.Context(), current); err != nil {
		logger.Warnw("更新订阅失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新订阅失败", nil)
		return
	}
	handler.SendOKResp(c, toSubscriptionView(current))
}

// DeleteSubscription 删除订阅。
func (s *svcImpl) DeleteSubscription(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "订阅 ID 非法", nil)
		return
	}
	if err := s.repo.DeleteSubscription(c.Request.Context(), userID, id); err != nil {
		logger.Warnw("删除订阅失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除订阅失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"id": id})
}

// SetSubscriptionEnabled 启停订阅。
func (s *svcImpl) SetSubscriptionEnabled(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	var req struct {
		ID      int64 `json:"id"`
		Enabled bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if err := s.repo.SetSubscriptionEnabled(c.Request.Context(), userID, req.ID, req.Enabled); err != nil {
		logger.Warnw("启停订阅失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "启停订阅失败", nil)
		return
	}
	// 首次启用才回溯（backfilled_at 为空）；反复停启不会重复扫库
	if req.Enabled {
		if sub, err := s.repo.GetSubscriptionByID(c.Request.Context(), req.ID); err == nil && sub != nil {
			s.enqueueSubscriptionBackfill(c.Request.Context(), sub)
		}
	}
	handler.SendOKResp(c, map[string]any{"id": req.ID, "enabled": req.Enabled})
}

// ParseSubscription 自然语言解析订阅草稿（不落库）。
func (s *svcImpl) ParseSubscription(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请输入订阅需求描述", nil)
		return
	}
	draft, err := s.ParseSubscriptionText(c.Request.Context(), req.Text)
	if err != nil {
		logger.Warnw("解析订阅描述失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "解析失败："+err.Error(), nil)
		return
	}
	names := make([]string, 0, len(draft.Industries))
	for _, code := range draft.Industries {
		// 与订阅列表口径一致：自定义行业原样展示，避免被统一成“其他”
		names = append(names, IndustryLabel(code))
	}
	handler.SendOKResp(c, map[string]any{
		"name":           draft.Name,
		"keywords":       draft.Keywords,
		"match_mode":     draft.MatchMode,
		"industries":     draft.Industries,
		"industry_names": names,
		"regions":        draft.Regions,
		"notice_types":   draft.NoticeTypes,
		"budget_min":     draft.BudgetMin,
		"budget_max":     draft.BudgetMax,
		"summary":        draft.Summary,
	})
}

func toSubscriptionView(item *model.TenderIntelSubscription) subscriptionView {
	industries := DecodeStringList(item.Industries)
	names := make([]string, 0, len(industries))
	for _, code := range industries {
		// 自定义行业原样展示，避免被统一成“其他”
		names = append(names, IndustryLabel(code))
	}
	return subscriptionView{
		ID:            item.ID,
		Name:          item.Name,
		Keywords:      DecodeStringList(item.Keywords),
		MatchMode:     normalizeMatchMode(item.MatchMode),
		Industries:    industries,
		IndustryNames: names,
		Regions:       DecodeStringList(item.Regions),
		NoticeTypes:   DecodeStringList(item.NoticeTypes),
		BudgetMin:     item.BudgetMin,
		BudgetMax:     item.BudgetMax,
		Enabled:       item.Enabled == 1,
		LastMatchedAt: formatTimePtr(item.LastMatchedAt),
		MatchedCount:  item.MatchedCount,
		CreatedAt:     formatTime(item.CreatedAt),
	}
}

func normalizeMatchMode(mode string) string {
	if strings.TrimSpace(mode) == "all" {
		return "all"
	}
	return "any"
}

func defaultSubscriptionName(name string, keywords, industries []string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return truncateRunes(name, 60)
	}
	if len(keywords) > 0 {
		return truncateRunes("关键词："+strings.Join(keywords, "、"), 60)
	}
	if len(industries) > 0 {
		names := make([]string, 0, len(industries))
		for _, code := range industries {
			names = append(names, IndustryLabel(code))
		}
		return truncateRunes("行业："+strings.Join(names, "、"), 60)
	}
	return "未命名订阅"
}
