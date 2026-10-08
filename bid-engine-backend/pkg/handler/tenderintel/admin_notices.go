package tenderintel

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/tenderintel"
)

// 手工录入的公告统一挂在“系统录入”这个虚拟来源下。
//
// 语义：notice.url 允许留空——线下收集的公告可能只有标题与正文，
// 此时用 manual://{token} 生成一个不可跳转的占位链接，保证 url_hash 唯一键成立。
const (
	ManualSourceKey   = "manual"
	ManualSourceName  = "系统录入"
	manualURLScheme   = "manual://"
	noticeMaxImportMB = 5
)

// 公告管理状态。下架与隐藏都会让公告从情报大厅消失，区别在语义与可恢复性：
//   - hidden   临时隐藏：内容仍然有效，恢复后立即回到大厅
//   - archived 下架：确认为误采或失效信息，不再展示
const (
	NoticeStatusNormal   = "normal"
	NoticeStatusHidden   = "hidden"
	NoticeStatusArchived = "archived"
)

// noticeStatusNames 状态中文名，与前端展示保持一致。
var noticeStatusNames = map[string]string{
	NoticeStatusNormal:   "在架",
	NoticeStatusHidden:   "已隐藏",
	NoticeStatusArchived: "已下架",
}

// adminNoticeItem 情报管理列表项：在情报大厅字段之上补充管理态字段。
type adminNoticeItem struct {
	noticeItem
	AdminNote string `json:"admin_note"`
	PinnedAt  string `json:"pinned_at"`
}

// adminNoticeSummary 情报管理概览口径（统计全库，不受当前筛选影响）。
type adminNoticeSummary struct {
	Total    int64 `json:"total"`
	Normal   int64 `json:"normal"`
	Hidden   int64 `json:"hidden"`
	Archived int64 `json:"archived"`
	Pinned   int64 `json:"pinned"`
	Manual   int64 `json:"manual"`
}

func isNoticeStatus(value string) bool {
	switch value {
	case NoticeStatusNormal, NoticeStatusHidden, NoticeStatusArchived:
		return true
	default:
		return false
	}
}

// NoticeStatusName 返回状态中文名（提示文案与导出复用）。
func NoticeStatusName(value string) string {
	if name, ok := noticeStatusNames[value]; ok {
		return name
	}
	return value
}

// ListAdminNotices 情报管理列表（含隐藏与下架公告，供管理员检索与批量操作）。
func (s *svcImpl) ListAdminNotices(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	statuses := queryList(c, "statuses")
	for _, status := range statuses {
		if !isNoticeStatus(status) {
			handler.SendNormalResp(c, entity.ErrCodeParam, "statuses 只能是 normal / hidden / archived", nil)
			return
		}
	}

	filter := &tenderintel.NoticeFilter{
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		Industries:   queryList(c, "industries"),
		NoticeTypes:  queryList(c, "notice_types"),
		Regions:      queryList(c, "regions"),
		SourceKeys:   queryList(c, "source_keys"),
		Origins:      queryList(c, "origins"),
		ImportBatch:  strings.TrimSpace(c.Query("import_batch")),
		BudgetMin:    queryFloat(c, "budget_min"),
		BudgetMax:    queryFloat(c, "budget_max"),
		PublishFrom:  queryDate(c, "date_from"),
		PublishTo:    queryDate(c, "date_to"),
		CollectFrom:  collectFromQuery(c),
		Statuses:     statuses,
		PinnedOnly:   c.Query("pinned_only") == "true" || c.Query("pinned_only") == "1",
		PageNum:      queryInt(c, "pageNum", 1),
		PageSize:     queryInt(c, "pageSize", 20),
		OrderByField: strings.TrimSpace(c.Query("order")),
	}

	items, total, err := s.repo.PageNotices(c.Request.Context(), filter)
	if err != nil {
		logger.Warnw("查询情报管理列表失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询情报列表失败", nil)
		return
	}

	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	industryMap, err := s.repo.ListNoticeIndustryMap(c.Request.Context(), ids)
	if err != nil {
		// 行业标签缺失不影响列表主体，降级为空标签继续返回
		logger.Warnw("读取行业标签失败", "err", err)
	}

	out := make([]adminNoticeItem, 0, len(items))
	for _, item := range items {
		out = append(out, adminNoticeItem{
			noticeItem: toNoticeItem(item, industryMap[item.ID], false),
			AdminNote:  item.AdminNote,
			PinnedAt:   formatTimePtr(item.PinnedAt),
		})
	}

	summary, err := s.adminNoticeSummary(c)
	if err != nil {
		logger.Warnw("统计情报管理概览失败", "err", err)
		summary = &adminNoticeSummary{Total: total}
	}

	handler.SendPageRespV2Extra(c, out, total, filter.PageNum, filter.PageSize, map[string]interface{}{
		"summary": summary,
	})
}

// adminNoticeSummary 汇总情报管理概览数据（全库口径，不受筛选影响）。
func (s *svcImpl) adminNoticeSummary(c *gin.Context) (*adminNoticeSummary, error) {
	ctx := c.Request.Context()
	byStatus, err := s.repo.CountNoticesByStatus(ctx)
	if err != nil {
		return nil, err
	}
	pinned, err := s.repo.CountPinnedNotices(ctx)
	if err != nil {
		return nil, err
	}
	manual, err := s.repo.CountManualNotices(ctx)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, count := range byStatus {
		total += count
	}
	return &adminNoticeSummary{
		Total:    total,
		Normal:   byStatus[NoticeStatusNormal],
		Hidden:   byStatus[NoticeStatusHidden],
		Archived: byStatus[NoticeStatusArchived],
		Pinned:   pinned,
		Manual:   manual,
	}, nil
}

// noticeUpsertRequest 手工录入与编辑公告的请求体。
//
// 字段用指针：编辑时“未传”与“传空值”语义不同（未传表示不改）。
type noticeUpsertRequest struct {
	ID             *int64   `json:"id"`
	Title          *string  `json:"title"`
	Publisher      *string  `json:"publisher"`
	Agency         *string  `json:"agency"`
	ProjectCode    *string  `json:"project_code"`
	BudgetText     *string  `json:"budget_text"`
	BudgetAmount   *float64 `json:"budget_amount"` // 单位：元
	RegionProvince *string  `json:"region_province"`
	RegionCity     *string  `json:"region_city"`
	NoticeType     *string  `json:"notice_type"`
	PublishDate    *string  `json:"publish_date"`
	DeadlineAt     *string  `json:"deadline_at"`
	URL            *string  `json:"url"`
	SourceName     *string  `json:"source_name"`
	BodyText       *string  `json:"body_text"`
	BodyMarkdown   *string  `json:"body_markdown"`
	AdminNote      *string  `json:"admin_note"`
	ImportBatch    *string  `json:"import_batch"`
	Industries     []string `json:"industries"`
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// randomToken 生成 64 位随机十六进制串（用于手工公告的占位链接）。
func randomToken() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// 随机源异常时退化为时间戳，仍能保证唯一性；极小概率冲突由 url_hash 唯一键兜底
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// newImportBatch 生成手工录入或批量导入的批次号，便于按批次管理情报。
func newImportBatch(prefix string) string {
	return prefix + time.Now().Format("20060102150405") + "-" + randomToken()[:4]
}

// parseNoticeDateTime 解析 yyyy-MM-dd 或 yyyy-MM-dd HH:mm:ss 等常见写法。
func parseNoticeDateTime(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("%s 不是合法日期（示例 2026-09-22）", value)
}

// normalizeNoticeType 规整公告类型：留空按标题自动判定，非法取值直接报错。
func normalizeNoticeType(code, title string) (string, error) {
	value := strings.TrimSpace(code)
	if value == "" {
		detected, _ := ClassifyNoticeType(title, "")
		return detected, nil
	}
	if _, ok := noticeTypeNames[value]; ok {
		return value, nil
	}
	return "", fmt.Errorf("公告类型 %s 不存在，可选：%s", value, strings.Join(noticeTypeOrder, " / "))
}

// normalizeIndustryInput 规整管理员填写的行业：枚举编码、枚举名称都会归一为编码，
// 无法归一的文本按自定义行业原样保留（行业名称不可能全量枚举）。
func normalizeIndustryInput(items []string) []string {
	out := make([]string, 0, len(items))
	for _, raw := range NormalizeIndustries(items) {
		if IsKnownIndustry(raw) {
			out = append(out, raw)
			continue
		}
		if code, ok := IndustryCodeByName(raw); ok {
			out = append(out, code)
			continue
		}
		out = append(out, raw)
	}
	return out
}

// buildManualNotice 由请求体构造待入库公告。
//
// allowDuplicateURL=true 时跳过链接去重（当前仅编辑场景复用，录入时始终保持去重）。
func (s *svcImpl) buildManualNotice(
	c *gin.Context,
	req *noticeUpsertRequest,
	batch string,
) (*model.TenderIntelNotice, error) {
	title := strings.TrimSpace(derefString(req.Title))
	if title == "" {
		return nil, fmt.Errorf("标题不能为空")
	}
	noticeType, err := normalizeNoticeType(derefString(req.NoticeType), title)
	if err != nil {
		return nil, err
	}
	publishDate, err := parseNoticeDateTime(derefString(req.PublishDate))
	if err != nil {
		return nil, fmt.Errorf("发布时间：%w", err)
	}
	deadline, err := parseNoticeDateTime(derefString(req.DeadlineAt))
	if err != nil {
		return nil, fmt.Errorf("截止时间：%w", err)
	}
	if req.BudgetAmount != nil && *req.BudgetAmount < 0 {
		return nil, fmt.Errorf("预算金额不能为负数")
	}

	rawURL := strings.TrimSpace(derefString(req.URL))
	if rawURL != "" && !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("来源链接需以 http:// 或 https:// 开头")
	}
	canonical := rawURL
	if canonical == "" {
		canonical = manualURLScheme + randomToken()
	} else {
		canonical = CanonicalizeURL(rawURL)
	}
	urlHash := HashURL(canonical)
	if existing, err := s.repo.GetNoticeByURLHash(c.Request.Context(), urlHash); err == nil && existing != nil {
		return nil, fmt.Errorf("来源链接已存在（公告 #%d：%s）", existing.ID, existing.Title)
	}

	bodyText := strings.TrimSpace(derefString(req.BodyText))
	bodyMarkdown := strings.TrimSpace(derefString(req.BodyMarkdown))
	if bodyMarkdown == "" && bodyText != "" {
		bodyMarkdown = bodyText
	}
	sourceName := firstNonEmpty(strings.TrimSpace(derefString(req.SourceName)), ManualSourceName)

	return &model.TenderIntelNotice{
		SourceKey:         ManualSourceKey,
		SourceName:        truncateRunes(sourceName, 120),
		SourceCategory:    ManualSourceName,
		Origin:            "manual",
		ImportBatch:       truncateRunes(batch, 60),
		URL:               rawURL,
		CanonicalURL:      truncateRunes(canonical, 1000),
		URLHash:           urlHash,
		Title:             truncateRunes(title, 500),
		Publisher:         truncateRunes(derefString(req.Publisher), 250),
		Agency:            truncateRunes(derefString(req.Agency), 250),
		ProjectCode:       truncateRunes(derefString(req.ProjectCode), 120),
		BudgetText:        truncateRunes(derefString(req.BudgetText), 250),
		BudgetAmount:      req.BudgetAmount,
		RegionProvince:    truncateRunes(derefString(req.RegionProvince), 60),
		RegionCity:        truncateRunes(derefString(req.RegionCity), 60),
		RegionText:        truncateRunes(firstNonEmpty(derefString(req.RegionProvince), derefString(req.RegionCity)), 120),
		NoticeType:        noticeType,
		NoticeStage:       NoticeStageFor(noticeType),
		PublishDate:       publishDate,
		DeadlineAt:        deadline,
		BodyText:          bodyText,
		BodyMarkdown:      bodyMarkdown,
		ContentHash:       HashContent(bodyText),
		FetchStrategy:     "manual",
		ExtractConfidence: 1,
		TagStatus:         "done",
		Status:            NoticeStatusNormal,
		AdminNote:         truncateRunes(derefString(req.AdminNote), 250),
	}, nil
}

// saveManualIndustries 写入行业标签。
//
// 未指定行业时用关键词兜底并保留 pending，交给下一轮打标流程用模型细化；
// 明确指定行业时直接置 done，避免人工填写被模型覆盖。
// 返回真正落库的行业编码。
func (s *svcImpl) saveManualIndustries(c *gin.Context, notice *model.TenderIntelNotice, provided []string) []string {
	logger := s.logger.With(entity.Ctx(c)...)
	codes := normalizeIndustryInput(provided)
	if len(codes) == 0 {
		codes = RankIndustriesByKeyword(notice.Title, truncateRunes(notice.BodyText, tagBodyExcerptRunes))
		notice.TagStatus = "pending"
	}
	weights := make(map[string]int, len(codes))
	for idx, code := range codes {
		weights[code] = len(codes) - idx
	}
	if err := s.repo.ReplaceNoticeIndustries(c.Request.Context(), notice.ID, codes, weights); err != nil {
		logger.Warnw("写入情报行业标签失败", "notice_id", notice.ID, "err", err)
		return codes
	}
	if err := s.repo.UpdateNoticeFields(c.Request.Context(), notice.ID, map[string]interface{}{
		"tag_status": notice.TagStatus,
	}); err != nil {
		logger.Warnw("回写情报打标状态失败", "notice_id", notice.ID, "err", err)
	}
	return codes
}

// CreateAdminNotice 手工发布一条情报（来源标记为“系统录入”）。
func (s *svcImpl) CreateAdminNotice(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req noticeUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	batch := strings.TrimSpace(derefString(req.ImportBatch))
	if batch == "" {
		batch = newImportBatch("manual-")
	}
	notice, err := s.buildManualNotice(c, &req, batch)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if err := s.repo.CreateNotice(c.Request.Context(), notice); err != nil {
		logger.Warnw("手工发布情报失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "发布失败："+err.Error(), nil)
		return
	}

	industries := s.saveManualIndustries(c, notice, req.Industries)
	// 手工录入同样触发订阅匹配（走独立匹配队列，不阻塞发布接口本身）
	s.enqueueMatchTaskForNotices(c.Request.Context(), MatchTaskTypeManual, MatchScopeNotice,
		strconv.FormatInt(notice.ID, 10), notice.ID)

	handler.SendOKResp(c, map[string]any{
		"id":           notice.ID,
		"import_batch": notice.ImportBatch,
		"industries":   industries,
	})
}

// UpdateAdminNotice 编辑已入库公告（自动采集与手工录入的公告都可以修正）。
func (s *svcImpl) UpdateAdminNotice(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req noticeUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if req.ID == nil || *req.ID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "公告 ID 非法", nil)
		return
	}
	existing, err := s.repo.GetNotice(c.Request.Context(), *req.ID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公告不存在", nil)
		return
	}

	title := firstNonEmpty(strings.TrimSpace(derefString(req.Title)), existing.Title)
	updates := map[string]interface{}{}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			handler.SendNormalResp(c, entity.ErrCodeParam, "标题不能为空", nil)
			return
		}
		updates["title"] = truncateRunes(strings.TrimSpace(*req.Title), 500)
	}
	if req.Publisher != nil {
		updates["publisher"] = truncateRunes(strings.TrimSpace(*req.Publisher), 250)
	}
	if req.Agency != nil {
		updates["agency"] = truncateRunes(strings.TrimSpace(*req.Agency), 250)
	}
	if req.ProjectCode != nil {
		updates["project_code"] = truncateRunes(strings.TrimSpace(*req.ProjectCode), 120)
	}
	if req.BudgetText != nil {
		updates["budget_text"] = truncateRunes(strings.TrimSpace(*req.BudgetText), 250)
	}
	if req.BudgetAmount != nil {
		if *req.BudgetAmount < 0 {
			handler.SendNormalResp(c, entity.ErrCodeParam, "预算金额不能为负数", nil)
			return
		}
		updates["budget_amount"] = *req.BudgetAmount
	}
	if req.RegionProvince != nil {
		updates["region_province"] = truncateRunes(strings.TrimSpace(*req.RegionProvince), 60)
	}
	if req.RegionCity != nil {
		updates["region_city"] = truncateRunes(strings.TrimSpace(*req.RegionCity), 60)
	}
	if req.NoticeType != nil {
		noticeType, err := normalizeNoticeType(*req.NoticeType, title)
		if err != nil {
			handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
			return
		}
		updates["notice_type"] = noticeType
		updates["notice_stage"] = NoticeStageFor(noticeType)
	}
	if req.PublishDate != nil {
		value, err := parseNoticeDateTime(*req.PublishDate)
		if err != nil {
			handler.SendNormalResp(c, entity.ErrCodeParam, "发布时间："+err.Error(), nil)
			return
		}
		updates["publish_date"] = value
	}
	if req.DeadlineAt != nil {
		value, err := parseNoticeDateTime(*req.DeadlineAt)
		if err != nil {
			handler.SendNormalResp(c, entity.ErrCodeParam, "截止时间："+err.Error(), nil)
			return
		}
		updates["deadline_at"] = value
	}
	if req.SourceName != nil {
		updates["source_name"] = truncateRunes(strings.TrimSpace(*req.SourceName), 120)
	}
	if req.AdminNote != nil {
		updates["admin_note"] = truncateRunes(strings.TrimSpace(*req.AdminNote), 250)
	}
	if req.URL != nil {
		rawURL := strings.TrimSpace(*req.URL)
		if rawURL != "" && !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			handler.SendNormalResp(c, entity.ErrCodeParam, "来源链接需以 http:// 或 https:// 开头", nil)
			return
		}
		canonical := existing.CanonicalURL
		if rawURL != "" {
			canonical = CanonicalizeURL(rawURL)
			urlHash := HashURL(canonical)
			if other, err := s.repo.GetNoticeByURLHash(c.Request.Context(), urlHash); err == nil && other != nil && other.ID != existing.ID {
				handler.SendNormalResp(c, entity.ErrCodeParam, fmt.Sprintf("来源链接已存在（公告 #%d）", other.ID), nil)
				return
			}
			updates["url_hash"] = urlHash
		}
		updates["url"] = rawURL
		updates["canonical_url"] = truncateRunes(canonical, 1000)
	}
	if req.BodyText != nil || req.BodyMarkdown != nil {
		bodyText := firstNonEmpty(strings.TrimSpace(derefString(req.BodyText)), existing.BodyText)
		bodyMarkdown := firstNonEmpty(strings.TrimSpace(derefString(req.BodyMarkdown)), bodyText)
		if req.BodyText != nil {
			updates["body_text"] = bodyText
			updates["content_hash"] = HashContent(bodyText)
		}
		updates["body_html"] = ""
		updates["body_markdown"] = bodyMarkdown
	}

	if len(updates) > 0 {
		if err := s.repo.UpdateNoticeFields(c.Request.Context(), existing.ID, updates); err != nil {
			logger.Warnw("更新情报失败", "notice_id", existing.ID, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeDBWrite, "保存失败："+err.Error(), nil)
			return
		}
	}
	if req.Industries != nil {
		merged := *existing
		merged.Title = title
		merged.BodyText = firstNonEmpty(strings.TrimSpace(derefString(req.BodyText)), existing.BodyText)
		s.saveManualIndustries(c, &merged, req.Industries)
	}

	handler.SendOKResp(c, map[string]any{"id": existing.ID})
}

// noticeIDsBody 批量操作的请求体。
type noticeIDsBody struct {
	IDs []int64 `json:"ids"`
}

// SetNoticeStatus 隐藏 / 下架 / 恢复公告（支持批量）。
func (s *svcImpl) SetNoticeStatus(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req struct {
		noticeIDsBody
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if !isNoticeStatus(req.Status) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "状态只能是 normal / hidden / archived", nil)
		return
	}
	if len(req.IDs) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请先选择要操作的公告", nil)
		return
	}
	updates := map[string]interface{}{"status": req.Status}
	// 备注只在明确填写时覆盖：隐藏/下架时通常不需要备注，不应顺手清空已有备注
	if note := strings.TrimSpace(req.Note); note != "" {
		updates["admin_note"] = truncateRunes(note, 250)
	}
	affected, err := s.repo.UpdateNoticesByIDs(c.Request.Context(), req.IDs, updates)
	if err != nil {
		logger.Warnw("更新情报状态失败", "status", req.Status, "count", len(req.IDs), "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "操作失败："+err.Error(), nil)
		return
	}
	handler.SendOKResp(c, map[string]any{
		"affected": affected,
		"status":   req.Status,
		"label":    NoticeStatusName(req.Status),
	})
}

// SetNoticePinned 置顶 / 取消置顶（支持批量）。
func (s *svcImpl) SetNoticePinned(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req struct {
		noticeIDsBody
		Pinned bool `json:"pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if len(req.IDs) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请先选择要操作的公告", nil)
		return
	}
	updates := map[string]interface{}{"pinned": int32(0), "pinned_at": nil}
	if req.Pinned {
		updates = map[string]interface{}{"pinned": int32(1), "pinned_at": NowFunc()}
	}
	affected, err := s.repo.UpdateNoticesByIDs(c.Request.Context(), req.IDs, updates)
	if err != nil {
		logger.Warnw("更新情报置顶失败", "pinned", req.Pinned, "count", len(req.IDs), "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "操作失败："+err.Error(), nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"affected": affected, "pinned": req.Pinned})
}

// DeleteAdminNotices 删除公告及其派生数据（支持批量）。
func (s *svcImpl) DeleteAdminNotices(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	var req noticeIDsBody
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	if len(req.IDs) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请先选择要删除的公告", nil)
		return
	}
	affected, err := s.repo.DeleteNotices(c.Request.Context(), req.IDs)
	if err != nil {
		logger.Warnw("删除情报失败", "count", len(req.IDs), "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败："+err.Error(), nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"deleted": affected})
}

// ── 批量导入 ────────────────────────────────────────────────────

// noticeImportColumns 情报导入表格的列定义（顺序即模板列顺序）。
var noticeImportColumns = []string{
	"title",
	"publisher",
	"agency",
	"project_code",
	"budget_amount_wan",
	"budget_text",
	"region_province",
	"region_city",
	"notice_type",
	"publish_date",
	"deadline_at",
	"url",
	"industries",
	"body_text",
	"source_name",
}

// noticeTemplateRows 模板示例行，同时充当填写说明。
func noticeTemplateRows() [][]string {
	return [][]string{
		{
			"某某市政务云平台建设项目公开招标公告",                   // title：必填，公告标题
			"某某市大数据管理局",                            // publisher：采购人
			"某某招标代理有限公司",                           // agency：代理机构
			"ZB-2026-0001",                         // project_code：项目编号
			"860",                                  // budget_amount_wan：预算金额，单位万元（只填数字）
			"预算金额 860 万元",                          // budget_text：预算原文（可留空，留空时按万元自动生成）
			"广东省",                                  // region_province：省级地区
			"深圳市",                                  // region_city：市级地区
			"open_tender",                          // notice_type：公告类型编码，留空按标题自动判定
			"2026-09-22",                           // publish_date：发布时间，yyyy-MM-dd
			"2026-10-15 09:30:00",                  // deadline_at：投标截止时间（可留空）
			"https://example.gov.cn/notice/1.html", // url：来源链接（可留空）
			"it_informatization,政务",                // industries：行业编码或名称，逗号分隔（可留空，留空按标题自动打标）
			"此处填写公告正文或采购需求要点",                      // body_text：正文（可留空）
			"系统录入",                                 // source_name：来源名称（留空为“系统录入”）
		},
	}
}

// DownloadNoticeTemplate 下载情报导入模板（CSV，UTF-8 带 BOM，Excel 可直接打开）。
func (s *svcImpl) DownloadNoticeTemplate(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	var builder strings.Builder
	builder.WriteString("\ufeff")
	writer := csv.NewWriter(&builder)
	if err := writer.Write(noticeImportColumns); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "生成模板失败", nil)
		return
	}
	for _, row := range noticeTemplateRows() {
		if err := writer.Write(row); err != nil {
			handler.SendNormalResp(c, entity.ErrCodeInternal, "生成模板失败", nil)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "生成模板失败", nil)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"tender-intel-notices-template.csv\"")
	c.String(http.StatusOK, builder.String())
}

// builtNoticeRow 单行解析结果。
type builtNoticeRow struct {
	notice     *model.TenderIntelNotice
	industries []string
}

// buildNoticeFromRow 把一行表格转换为公告，失败时返回原因。
func buildNoticeFromRow(header, row []string) (*builtNoticeRow, string) {
	values := map[string]string{}
	for index, column := range header {
		key := strings.TrimSpace(strings.ToLower(column))
		if key == "" {
			continue
		}
		if index < len(row) {
			values[key] = strings.TrimSpace(row[index])
		}
	}

	title := values["title"]
	if title == "" {
		return nil, "title 不能为空"
	}
	noticeType, err := normalizeNoticeType(values["notice_type"], title)
	if err != nil {
		return nil, err.Error()
	}
	publishDate, err := parseNoticeDateTime(values["publish_date"])
	if err != nil {
		return nil, "publish_date：" + err.Error()
	}
	deadline, err := parseNoticeDateTime(values["deadline_at"])
	if err != nil {
		return nil, "deadline_at：" + err.Error()
	}

	var budgetAmount *float64
	if raw := values["budget_amount_wan"]; raw != "" {
		wan, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
		if err != nil || wan < 0 {
			return nil, "budget_amount_wan 需为非负数字（单位：万元）"
		}
		yuan := wan * 10000
		budgetAmount = &yuan
	}
	budgetText := values["budget_text"]
	if budgetText == "" && budgetAmount != nil {
		budgetText = fmt.Sprintf("预算金额 %.2f 万元", *budgetAmount/10000)
	}

	rawURL := values["url"]
	if rawURL != "" && !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, "url 需以 http:// 或 https:// 开头"
	}
	canonical := rawURL
	if canonical == "" {
		canonical = manualURLScheme + randomToken()
	} else {
		canonical = CanonicalizeURL(rawURL)
	}

	bodyText := values["body_text"]
	sourceName := firstNonEmpty(values["source_name"], ManualSourceName)

	industries := make([]string, 0, 4)
	for _, part := range strings.FieldsFunc(values["industries"], func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '|'
	}) {
		industries = append(industries, part)
	}
	industries = normalizeIndustryInput(industries)

	return &builtNoticeRow{
		notice: &model.TenderIntelNotice{
			SourceKey:         ManualSourceKey,
			SourceName:        truncateRunes(sourceName, 120),
			SourceCategory:    ManualSourceName,
			Origin:            "manual",
			URL:               rawURL,
			CanonicalURL:      truncateRunes(canonical, 1000),
			URLHash:           HashURL(canonical),
			Title:             truncateRunes(title, 500),
			Publisher:         truncateRunes(values["publisher"], 250),
			Agency:            truncateRunes(values["agency"], 250),
			ProjectCode:       truncateRunes(values["project_code"], 120),
			BudgetText:        truncateRunes(budgetText, 250),
			BudgetAmount:      budgetAmount,
			RegionProvince:    truncateRunes(values["region_province"], 60),
			RegionCity:        truncateRunes(values["region_city"], 60),
			RegionText:        truncateRunes(firstNonEmpty(values["region_province"], values["region_city"]), 120),
			NoticeType:        noticeType,
			NoticeStage:       NoticeStageFor(noticeType),
			PublishDate:       publishDate,
			DeadlineAt:        deadline,
			BodyText:          bodyText,
			BodyMarkdown:      bodyText,
			ContentHash:       HashContent(bodyText),
			FetchStrategy:     "manual",
			ExtractConfidence: 1,
			TagStatus:         "done",
			Status:            NoticeStatusNormal,
		},
		industries: industries,
	}, ""
}

// importedNoticeResult 单行导入结果。
type importedNoticeResult struct {
	Row     int    `json:"row"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// ImportNotices 解析管理员上传的情报表格（CSV / XLSX）并落库。
//
// 幂等：url 命中的行默认按“已存在”跳过（overwrite=true 时更新其内容），因此同一份
// 表格可以反复导入而不会产生重复公告；url 留空的行按新公告写入。
func (s *svcImpl) ImportNotices(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请上传情报表格（.csv 或 .xlsx）", nil)
		return
	}
	if fileHeader.Size > noticeMaxImportMB*1024*1024 {
		handler.SendNormalResp(c, entity.ErrCodeParam,
			fmt.Sprintf("文件超过 %dMB 限制", noticeMaxImportMB), nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "读取上传文件失败", nil)
		return
	}
	defer func() { _ = file.Close() }()

	rows, err := readSourceTable(fileHeader.Filename, file)
	if err != nil {
		logger.Warnw("解析情报表格失败", "file", fileHeader.Filename, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, "解析表格失败："+err.Error(), nil)
		return
	}
	if len(rows) <= 1 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "表格中没有数据行（第一行应为表头）", nil)
		return
	}

	overwrite := c.PostForm("overwrite") == "true" || c.PostForm("overwrite") == "1"
	batch := newImportBatch("import-")
	header := rows[0]
	failures := make([]importedNoticeResult, 0)
	created, updated, skipped := 0, 0, 0

	for index, row := range rows[1:] {
		rowNo := index + 2
		built, msg := buildNoticeFromRow(header, row)
		if msg != "" {
			failures = append(failures, importedNoticeResult{Row: rowNo, Message: msg})
			continue
		}
		built.notice.ImportBatch = batch

		existing, findErr := s.repo.GetNoticeByURLHash(c.Request.Context(), built.notice.URLHash)
		if findErr == nil && existing != nil {
			if !overwrite {
				skipped++
				continue
			}
			updates := map[string]interface{}{
				"title":           built.notice.Title,
				"publisher":       built.notice.Publisher,
				"agency":          built.notice.Agency,
				"project_code":    built.notice.ProjectCode,
				"budget_text":     built.notice.BudgetText,
				"budget_amount":   built.notice.BudgetAmount,
				"region_province": built.notice.RegionProvince,
				"region_city":     built.notice.RegionCity,
				"notice_type":     built.notice.NoticeType,
				"notice_stage":    built.notice.NoticeStage,
				"publish_date":    built.notice.PublishDate,
				"deadline_at":     built.notice.DeadlineAt,
				"body_text":       built.notice.BodyText,
				"body_markdown":   built.notice.BodyMarkdown,
				"content_hash":    built.notice.ContentHash,
				"import_batch":    batch,
			}
			if err := s.repo.UpdateNoticeFields(c.Request.Context(), existing.ID, updates); err != nil {
				failures = append(failures, importedNoticeResult{
					Row: rowNo, Title: built.notice.Title, Message: "更新失败：" + err.Error(),
				})
				continue
			}
			built.notice.ID = existing.ID
			s.saveManualIndustries(c, built.notice, built.industries)
			updated++
			continue
		}
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			failures = append(failures, importedNoticeResult{
				Row: rowNo, Title: built.notice.Title, Message: "查询失败：" + findErr.Error(),
			})
			continue
		}
		if err := s.repo.CreateNotice(c.Request.Context(), built.notice); err != nil {
			failures = append(failures, importedNoticeResult{
				Row: rowNo, Title: built.notice.Title, Message: "入库失败：" + err.Error(),
			})
			continue
		}
		s.saveManualIndustries(c, built.notice, built.industries)
		created++
	}

	// 整个导入批次只投一个匹配任务，范围就是这批新入库的情报
	if created > 0 {
		s.enqueueMatchTaskForNotices(c.Request.Context(), MatchTaskTypeImport, MatchScopeBatch, batch, 0)
	}

	logger.Infow("情报批量导入完成",
		"batch", batch, "created", created, "updated", updated,
		"skipped", skipped, "failed", len(failures))

	handler.SendOKResp(c, map[string]any{
		"created":   created,
		"updated":   updated,
		"skipped":   skipped,
		"failed":    failures,
		"total":     len(rows) - 1,
		"overwrite": overwrite,
		"batch":     batch,
		"notice":    "导入的情报默认在架；未填写行业时按标题自动打标，并在下一轮采集后由模型细化。",
	})
}
