package tenderintel

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/tenderintel"
)

// noticeItem 情报大厅列表项。
type noticeItem struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	URL            string   `json:"url"`
	SourceKey      string   `json:"source_key"`
	SourceName     string   `json:"source_name"`
	SourceCategory string   `json:"source_category"`
	Publisher      string   `json:"publisher"`
	Agency         string   `json:"agency"`
	ProjectCode    string   `json:"project_code"`
	BudgetText     string   `json:"budget_text"`
	BudgetAmount   *float64 `json:"budget_amount"`
	RegionProvince string   `json:"region_province"`
	RegionCity     string   `json:"region_city"`
	NoticeType     string   `json:"notice_type"`
	NoticeTypeName string   `json:"notice_type_name"`
	NoticeStage    string   `json:"notice_stage"`
	PublishDate    string   `json:"publish_date"`
	DeadlineAt     string   `json:"deadline_at"`
	Industries     []string `json:"industries"`
	IndustryNames  []string `json:"industry_names"`
	TagStatus      string   `json:"tag_status"`
	Favorited      bool     `json:"favorited"`
	Pinned         bool     `json:"pinned"`
	Origin         string   `json:"origin"`
	Status         string   `json:"status"`
	ImportBatch    string   `json:"import_batch"`
	FirstSeenAt    string   `json:"first_seen_at"`
}

// noticeDetail 公告详情。
type noticeDetail struct {
	noticeItem
	BodyHTML          string                `json:"body_html"`
	BodyMarkdown      string                `json:"body_markdown"`
	BodyText          string                `json:"body_text"`
	Attachments       []collectorAttachment `json:"attachments"`
	FetchStrategy     string                `json:"fetch_strategy"`
	ExtractConfidence float64               `json:"extract_confidence"`
	ExtractWarnings   []string              `json:"extract_warnings"`
	Insight           *insightView          `json:"insight"`
}

type insightView struct {
	ContentMD string `json:"content_md"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListNotices 情报大厅列表。
func (s *svcImpl) ListNotices(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	filter := &tenderintel.NoticeFilter{
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		Industries:   queryList(c, "industries"),
		NoticeTypes:  queryList(c, "notice_types"),
		Regions:      queryList(c, "regions"),
		SourceKeys:   queryList(c, "source_keys"),
		BudgetMin:    queryFloat(c, "budget_min"),
		BudgetMax:    queryFloat(c, "budget_max"),
		PublishFrom:  queryDate(c, "date_from"),
		PublishTo:    queryDate(c, "date_to"),
		CollectFrom:  collectFromQuery(c),
		PageNum:      queryInt(c, "pageNum", 1),
		PageSize:     queryInt(c, "pageSize", 20),
		OrderByField: strings.TrimSpace(c.Query("order")),
		OnlyValid:    true,
	}

	favoriteOnly := c.Query("favorite_only") == "true" || c.Query("favorite_only") == "1"
	var favoriteIDs []int64
	if favoriteOnly {
		ids, err := s.repo.ListFavoriteNoticeIDs(c.Request.Context(), userID)
		if err != nil {
			logger.Warnw("读取收藏失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeDBRead, "读取收藏失败", nil)
			return
		}
		favoriteIDs = ids
		filter.FavoriteIDs = ids
	}

	items, total, err := s.repo.PageNotices(c.Request.Context(), filter)
	if err != nil {
		logger.Warnw("查询情报列表失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询情报列表失败", nil)
		return
	}

	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	industryMap, err := s.repo.ListNoticeIndustryMap(c.Request.Context(), ids)
	if err != nil {
		logger.Warnw("读取行业标签失败", "err", err)
	}
	favoriteSet := make(map[int64]struct{}, len(favoriteIDs))
	for _, id := range favoriteIDs {
		favoriteSet[id] = struct{}{}
	}
	if !favoriteOnly {
		for _, id := range ids {
			ok, err := s.repo.IsFavorite(c.Request.Context(), userID, id)
			if err != nil {
				logger.Warnw("读取收藏状态失败", "notice_id", id, "err", err)
				continue
			}
			if ok {
				favoriteSet[id] = struct{}{}
			}
		}
	}

	out := make([]noticeItem, 0, len(items))
	for _, item := range items {
		_, favorited := favoriteSet[item.ID]
		out = append(out, toNoticeItem(item, industryMap[item.ID], favorited))
	}
	handler.SendPageRespV2(c, out, total, filter.PageNum, filter.PageSize)
}

// GetNotice 公告详情。
func (s *svcImpl) GetNotice(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	noticeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || noticeID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "公告 ID 非法", nil)
		return
	}

	notice, err := s.repo.GetNotice(c.Request.Context(), noticeID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公告不存在", nil)
		return
	}
	industryMap, _ := s.repo.ListNoticeIndustryMap(c.Request.Context(), []int64{noticeID})
	favorited, err := s.repo.IsFavorite(c.Request.Context(), userID, noticeID)
	if err != nil {
		logger.Warnw("读取收藏状态失败", "err", err)
	}

	detail := noticeDetail{
		noticeItem:        toNoticeItem(notice, industryMap[noticeID], favorited),
		BodyHTML:          notice.BodyHTML,
		BodyMarkdown:      notice.BodyMarkdown,
		BodyText:          notice.BodyText,
		FetchStrategy:     notice.FetchStrategy,
		ExtractConfidence: notice.ExtractConfidence,
		ExtractWarnings:   DecodeStringList(notice.ExtractWarnings),
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(notice.Attachments)), &detail.Attachments)

	if insight, err := s.repo.GetInsight(c.Request.Context(), noticeID); err == nil && insight != nil {
		detail.Insight = &insightView{
			ContentMD: insight.ContentMd,
			Status:    insight.Status,
			CreatedAt: formatTime(insight.CreatedAt),
		}
	}
	handler.SendOKResp(c, detail)
}

// SetFavorite 收藏 / 取消收藏。
func (s *svcImpl) SetFavorite(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	noticeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || noticeID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "公告 ID 非法", nil)
		return
	}
	var req struct {
		Favorite *bool `json:"favorite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Favorite == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误：需要 favorite 布尔值", nil)
		return
	}
	if _, err := s.repo.GetNotice(c.Request.Context(), noticeID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公告不存在", nil)
		return
	}
	if err := s.repo.SetFavorite(c.Request.Context(), userID, noticeID, *req.Favorite); err != nil {
		logger.Warnw("更新收藏失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新收藏失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"favorited": *req.Favorite})
}

// Filters 返回筛选项元数据。
func (s *svcImpl) Filters(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	industries, err := s.repo.ListIndustries(c.Request.Context())
	if err != nil {
		logger.Warnw("读取行业枚举失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "读取筛选项失败", nil)
		return
	}
	observedRegions, err := s.repo.ListRegions(c.Request.Context())
	if err != nil {
		logger.Warnw("读取地区列表失败", "err", err)
	}
	sources, err := s.repo.ListSources(c.Request.Context())
	if err != nil {
		logger.Warnw("读取来源列表失败", "err", err)
	}

	type industryOption struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}

	industryOptions := make([]industryOption, 0, len(industries))
	for _, item := range industries {
		industryOptions = append(industryOptions, industryOption{Code: item.Code, Name: item.Name})
	}
	sourceOptions := make([]gin.H, 0, len(sources)+1)
	for _, item := range sources {
		sourceOptions = append(sourceOptions, gin.H{
			"source_key": item.SourceKey,
			"name":       item.Name,
			"category":   item.Category,
		})
	}
	// 手工录入的公告不属于任何采集源，单独给出一个“系统录入”选项，
	// 否则管理员录入的情报无法按来源筛出来。
	sourceOptions = append(sourceOptions, gin.H{
		"source_key": ManualSourceKey,
		"name":       ManualSourceName,
		"category":   ManualSourceName,
	})

	// 地区：省级枚举为准（保证新部署也有可选项），再并入库中已出现过的取值
	regions := ProvinceList()
	seenRegion := make(map[string]struct{}, len(regions))
	for _, item := range regions {
		seenRegion[item] = struct{}{}
	}
	for _, item := range observedRegions {
		if _, ok := seenRegion[item]; ok {
			continue
		}
		seenRegion[item] = struct{}{}
		regions = append(regions, item)
	}

	handler.SendOKResp(c, map[string]any{
		"industries":   industryOptions,
		"regions":      regions,
		"notice_types": NoticeTypeOptions(),
		"sources":      sourceOptions,
	})
}

// GenerateInsight 生成或读取公告 AI 解读缓存。
func (s *svcImpl) GenerateInsight(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	noticeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || noticeID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "公告 ID 非法", nil)
		return
	}
	notice, err := s.repo.GetNotice(c.Request.Context(), noticeID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公告不存在", nil)
		return
	}
	if insight, err := s.repo.GetInsight(c.Request.Context(), noticeID); err == nil && insight != nil && insight.Status == "succeeded" {
		handler.SendOKResp(c, insightView{
			ContentMD: insight.ContentMd,
			Status:    insight.Status,
			CreatedAt: formatTime(insight.CreatedAt),
		})
		return
	}

	content, modelName, err := s.BuildInsight(c.Request.Context(), notice)
	if err != nil {
		logger.Warnw("生成 AI 解读失败", "notice_id", noticeID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeGeneral, "AI 解读生成失败："+err.Error(), nil)
		return
	}
	cache := &model.TenderIntelNoticeInsight{
		NoticeID:  noticeID,
		ContentMd: content,
		Model:     modelName,
		Status:    "succeeded",
	}
	if err := s.repo.SaveInsight(c.Request.Context(), cache); err != nil {
		logger.Warnw("写入 AI 解读缓存失败", "notice_id", noticeID, "err", err)
	}
	handler.SendOKResp(c, insightView{
		ContentMD: content,
		Status:    "succeeded",
		CreatedAt: formatTime(time.Now()),
	})
}

// ParseLink 返回“发起招标解析”的预填参数。
func (s *svcImpl) ParseLink(c *gin.Context) {
	noticeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || noticeID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "公告 ID 非法", nil)
		return
	}
	notice, err := s.repo.GetNotice(c.Request.Context(), noticeID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公告不存在", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{
		"name":         notice.Title,
		"publisher":    notice.Publisher,
		"source_url":   notice.URL,
		"project_code": notice.ProjectCode,
		"budget_text":  notice.BudgetText,
		"region":       firstNonEmpty(notice.RegionProvince, notice.RegionCity),
	})
}

// ── 工具函数 ────────────────────────────────────────────────────

func toNoticeItem(notice *model.TenderIntelNotice, industries []string, favorited bool) noticeItem {
	names := make([]string, 0, len(industries))
	for _, code := range industries {
		// 用 IndustryLabel 而不是 IndustryName：管理员在情报管理里加的自定义行业
		// 存的是原始文本（如“软件开发”），IndustryName 会把未知编码统一成“其他”，
		// 卡片上就会变成一排“其他”，看不出这条情报到底打了哪些标签。
		names = append(names, IndustryLabel(code))
	}
	return noticeItem{
		ID:             notice.ID,
		Title:          notice.Title,
		URL:            notice.URL,
		SourceKey:      notice.SourceKey,
		SourceName:     notice.SourceName,
		SourceCategory: notice.SourceCategory,
		Publisher:      notice.Publisher,
		Agency:         notice.Agency,
		ProjectCode:    notice.ProjectCode,
		BudgetText:     notice.BudgetText,
		BudgetAmount:   notice.BudgetAmount,
		RegionProvince: notice.RegionProvince,
		RegionCity:     notice.RegionCity,
		NoticeType:     notice.NoticeType,
		NoticeTypeName: NoticeTypeName(notice.NoticeType),
		NoticeStage:    notice.NoticeStage,
		PublishDate:    formatDate(notice.PublishDate),
		DeadlineAt:     formatTimePtr(notice.DeadlineAt),
		Industries:     industries,
		IndustryNames:  names,
		TagStatus:      notice.TagStatus,
		Favorited:      favorited,
		Pinned:         notice.Pinned == 1,
		Origin:         notice.Origin,
		Status:         notice.Status,
		ImportBatch:    notice.ImportBatch,
		FirstSeenAt:    formatTime(notice.FirstSeenAt),
	}
}

func formatDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

// queryList 支持 `a=1,2` 与 `a=1&a=2` 两种写法。
func queryList(c *gin.Context, key string) []string {
	raw := c.QueryArray(key)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryFloat(c *gin.Context, key string) *float64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func queryDate(c *gin.Context, key string) *time.Time {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	layout := "2006-01-02"
	if len(raw) > 10 {
		layout = "2006-01-02 15:04:05"
	}
	value, err := time.ParseInLocation(layout, raw, time.Local)
	if err != nil {
		return nil
	}
	return &value
}

// collectFromQuery 解析“采集时间下限”。
//
// 支持两种写法：
//   - collect_from=2026-09-01：明确指定起始日期
//   - collect_within_days=7：最近 N 天（含今天），由后端按服务器时区换算，
//     避免前端各自算日期导致口径不一致
func collectFromQuery(c *gin.Context) *time.Time {
	if value := queryDate(c, "collect_from"); value != nil {
		return value
	}
	raw := strings.TrimSpace(c.Query("collect_within_days"))
	if raw == "" {
		return nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 || days > 365 {
		return nil
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, -(days - 1))
	return &start
}
