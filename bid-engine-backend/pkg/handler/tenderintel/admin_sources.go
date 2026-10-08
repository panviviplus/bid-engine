package tenderintel

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// 采集源导入表格的列定义（顺序即模板列顺序）。
var sourceImportColumns = []string{
	"source_key",
	"name",
	"homepage_url",
	"list_url",
	"category",
	"region",
	"industry_hint",
	"discovery_mode",
	"needs_browser",
	"enabled",
	"priority",
	"params",
}

// sourceTemplateRows 模板示例行，同时充当填写说明。
func sourceTemplateRows() [][]string {
	return [][]string{
		{
			"demo_city_ggzy",                             // source_key：小写字母/数字/下划线，2-64 位，全平台唯一
			"某某市公共资源交易中心",                                // name：展示名称
			"https://ggzy.example.gov.cn/",               // homepage_url：门户地址
			"https://ggzy.example.gov.cn/jyxx/zbgg.html", // list_url：招标公告列表页（必填）
			"地方级",  // category：国家级/地方级/国央企/银行/高校/民营/其他
			"某某市",  // region：地区
			"通用",   // industry_hint：网站行业属性（可留空）
			"list", // discovery_mode：api=站内接口，list=列表页，browser=需要渲染
			"0",    // needs_browser：1=必须用无头浏览器渲染
			"1",    // enabled：1=启用，0=停用
			"200",  // priority：数字越小越先执行
			`{"linkPattern":"/zbgg/\\d+","contentSelectors":[".article-content"]}`, // params：可选，覆盖采集规则
		},
	}
}

// DownloadSourceTemplate 下载采集源导入模板（CSV，UTF-8 带 BOM，Excel 可直接打开）。
func (s *svcImpl) DownloadSourceTemplate(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	var builder strings.Builder
	builder.WriteString("\ufeff") // BOM：保证 Excel 正确识别 UTF-8
	writer := csv.NewWriter(&builder)
	if err := writer.Write(sourceImportColumns); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "生成模板失败", nil)
		return
	}
	for _, row := range sourceTemplateRows() {
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
	c.Header("Content-Disposition", "attachment; filename=\"tender-intel-sources-template.csv\"")
	c.String(http.StatusOK, builder.String())
}

// importedSourceResult 单行导入结果。
type importedSourceResult struct {
	Row     int    `json:"row"`
	Key     string `json:"source_key"`
	Message string `json:"message"`
}

// ImportSources 解析管理员上传的采集源表格（CSV / XLSX）并落库。
//
// 语义：source_key 已存在时默认跳过（overwrite=true 则更新），因此同一份表格可以反复导入。
func (s *svcImpl) ImportSources(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请上传采集源表格（.csv 或 .xlsx）", nil)
		return
	}
	if fileHeader.Size > 5*1024*1024 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "文件超过 5MB 限制", nil)
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
		logger.Warnw("解析采集源表格失败", "file", fileHeader.Filename, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, "解析表格失败："+err.Error(), nil)
		return
	}
	if len(rows) <= 1 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "表格中没有数据行（第一行应为表头）", nil)
		return
	}

	overwrite := c.PostForm("overwrite") == "true" || c.PostForm("overwrite") == "1"
	header := rows[0]
	parsed := make([]*model.TenderIntelSource, 0, len(rows)-1)
	failures := make([]importedSourceResult, 0)
	seenKeys := make(map[string]int, len(rows))

	for index, row := range rows[1:] {
		rowNo := index + 2 // 表格行号（含表头）
		item, msg := buildSourceFromRow(header, row)
		if msg != "" {
			failures = append(failures, importedSourceResult{Row: rowNo, Message: msg})
			continue
		}
		if first, dup := seenKeys[item.SourceKey]; dup {
			failures = append(failures, importedSourceResult{
				Row: rowNo, Key: item.SourceKey,
				Message: fmt.Sprintf("同一份表格中 source_key 重复（与第 %d 行冲突）", first),
			})
			continue
		}
		seenKeys[item.SourceKey] = rowNo
		parsed = append(parsed, item)
	}

	created, updated, skipped, err := s.repo.UpsertSources(c.Request.Context(), parsed, overwrite)
	if err != nil {
		logger.Warnw("导入采集源失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "导入采集源失败："+err.Error(), nil)
		return
	}

	handler.SendOKResp(c, map[string]any{
		"created":   created,
		"updated":   updated,
		"skipped":   skipped,
		"failed":    failures,
		"total":     len(parsed) + len(failures),
		"overwrite": overwrite,
		"notice":    "导入成功不代表立即可用：新源需要能被采集服务识别，可在采集源状态里点“探测”验证列表页是否可抓。",
	})
}

// readSourceTable 读取 CSV 或 XLSX 为二维字符串切片。
func readSourceTable(filename string, file io.Reader) ([][]string, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return readCSVTable(file)
	case strings.HasSuffix(lower, ".xlsx"):
		return readXLSXTable(file)
	default:
		return nil, fmt.Errorf("仅支持 .csv 或 .xlsx，当前文件为 %s", filename)
	}
}

func readCSVTable(file io.Reader) ([][]string, error) {
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允许列数不一致，由逐行校验兜底
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	// 去掉 UTF-8 BOM，避免第一列表头读成 "\ufeffsource_key"
	if len(rows) > 0 && len(rows[0]) > 0 {
		rows[0][0] = strings.TrimPrefix(rows[0][0], "\ufeff")
	}
	return rows, nil
}

func readXLSXTable(file io.Reader) ([][]string, error) {
	reader, err := excelize.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	sheets := reader.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("工作簿中没有工作表")
	}
	return reader.GetRows(sheets[0])
}

// buildSourceFromRow 把一行表格转换为采集源，失败时返回原因。
func buildSourceFromRow(header, row []string) (*model.TenderIntelSource, string) {
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

	sourceKey := strings.ToLower(values["source_key"])
	if sourceKey == "" {
		return nil, "source_key 不能为空"
	}
	if len(sourceKey) < 2 || len(sourceKey) > 64 {
		return nil, "source_key 长度需在 2-64 位之间"
	}
	for _, r := range sourceKey {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return nil, "source_key 只能包含小写字母、数字与下划线"
	}

	name := values["name"]
	if name == "" {
		return nil, "name 不能为空"
	}
	listURL := values["list_url"]
	if listURL == "" {
		return nil, "list_url 不能为空"
	}
	if !strings.HasPrefix(listURL, "http://") && !strings.HasPrefix(listURL, "https://") {
		return nil, "list_url 需以 http:// 或 https:// 开头"
	}
	homepage := values["homepage_url"]
	if homepage != "" && !strings.HasPrefix(homepage, "http://") && !strings.HasPrefix(homepage, "https://") {
		return nil, "homepage_url 需以 http:// 或 https:// 开头"
	}

	mode := strings.ToLower(values["discovery_mode"])
	if mode == "" {
		mode = "list"
	}
	switch mode {
	case "api", "list", "browser":
	default:
		return nil, "discovery_mode 只能是 api / list / browser"
	}

	// 空值语义：enabled 默认启用；needs_browser 默认不需要浏览器（避免导入后全部走浏览器渲染）
	needsBrowser, ok := parseBoolValue(values["needs_browser"], false)
	if !ok {
		return nil, "needs_browser 只能是 1/0、是/否"
	}
	enabled, ok := parseBoolValue(values["enabled"], true)
	if !ok {
		return nil, "enabled 只能是 1/0、是/否"
	}
	priority, ok := parsePriorityValue(values["priority"])
	if !ok {
		return nil, "priority 需为 0-9999 的整数"
	}

	params := values["params"]
	if params != "" && !json.Valid([]byte(params)) {
		return nil, "params 需为合法 JSON（可留空）"
	}

	needsBrowserValue := int32(0)
	if needsBrowser {
		needsBrowserValue = 1
	}
	enabledValue := int32(0)
	if enabled {
		enabledValue = 1
	}

	return &model.TenderIntelSource{
		SourceKey:     sourceKey,
		Name:          truncateRunes(name, 120),
		HomepageURL:   truncateRunes(homepage, 500),
		ListURL:       truncateRunes(listURL, 1000),
		Category:      truncateRunes(values["category"], 60),
		Region:        truncateRunes(values["region"], 60),
		IndustryHint:  truncateRunes(values["industry_hint"], 120),
		DiscoveryMode: mode,
		NeedsBrowser:  needsBrowserValue,
		Enabled:       enabledValue,
		Priority:      priority,
		Params:        params,
	}, ""
}

// UpdateSource 编辑采集源（超管）。仅传需要变更的字段。
func (s *svcImpl) UpdateSource(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	sourceKey := strings.TrimSpace(c.Param("key"))
	if sourceKey == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "source_key 不能为空", nil)
		return
	}
	if _, err := s.repo.GetSource(c.Request.Context(), sourceKey); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "采集源不存在", nil)
		return
	}

	var req struct {
		Name          *string `json:"name"`
		HomepageURL   *string `json:"homepage_url"`
		ListURL       *string `json:"list_url"`
		Category      *string `json:"category"`
		Region        *string `json:"region"`
		IndustryHint  *string `json:"industry_hint"`
		DiscoveryMode *string `json:"discovery_mode"`
		NeedsBrowser  *bool   `json:"needs_browser"`
		Enabled       *bool   `json:"enabled"`
		Priority      *int32  `json:"priority"`
		Params        *string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			handler.SendNormalResp(c, entity.ErrCodeParam, "名称不能为空", nil)
			return
		}
		updates["name"] = truncateRunes(strings.TrimSpace(*req.Name), 120)
	}
	if req.HomepageURL != nil {
		updates["homepage_url"] = truncateRunes(strings.TrimSpace(*req.HomepageURL), 500)
	}
	if req.ListURL != nil {
		value := strings.TrimSpace(*req.ListURL)
		if value == "" {
			handler.SendNormalResp(c, entity.ErrCodeParam, "列表地址不能为空", nil)
			return
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			handler.SendNormalResp(c, entity.ErrCodeParam, "列表地址需以 http:// 或 https:// 开头", nil)
			return
		}
		updates["list_url"] = truncateRunes(value, 1000)
	}
	if req.Category != nil {
		updates["category"] = truncateRunes(strings.TrimSpace(*req.Category), 60)
	}
	if req.Region != nil {
		updates["region"] = truncateRunes(strings.TrimSpace(*req.Region), 60)
	}
	if req.IndustryHint != nil {
		updates["industry_hint"] = truncateRunes(strings.TrimSpace(*req.IndustryHint), 120)
	}
	if req.DiscoveryMode != nil {
		mode := strings.ToLower(strings.TrimSpace(*req.DiscoveryMode))
		switch mode {
		case "api", "list", "browser":
			updates["discovery_mode"] = mode
		default:
			handler.SendNormalResp(c, entity.ErrCodeParam, "discovery_mode 只能是 api / list / browser", nil)
			return
		}
	}
	if req.NeedsBrowser != nil {
		value := int32(0)
		if *req.NeedsBrowser {
			value = 1
		}
		updates["needs_browser"] = value
	}
	if req.Enabled != nil {
		value := int32(0)
		if *req.Enabled {
			value = 1
		}
		updates["enabled"] = value
	}
	if req.Priority != nil {
		if *req.Priority < 0 || *req.Priority > 9999 {
			handler.SendNormalResp(c, entity.ErrCodeParam, "priority 需为 0-9999", nil)
			return
		}
		updates["priority"] = *req.Priority
	}
	if req.Params != nil {
		value := strings.TrimSpace(*req.Params)
		if value != "" && !json.Valid([]byte(value)) {
			handler.SendNormalResp(c, entity.ErrCodeParam, "params 需为合法 JSON（可留空）", nil)
			return
		}
		updates["params"] = value
	}

	if len(updates) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "没有需要更新的字段", nil)
		return
	}
	if err := s.repo.UpdateSourceFields(c.Request.Context(), sourceKey, updates); err != nil {
		logger.Warnw("更新采集源失败", "source_key", sourceKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新采集源失败", nil)
		return
	}
	item, err := s.repo.GetSource(c.Request.Context(), sourceKey)
	if err != nil {
		handler.SendOKResp(c, map[string]any{"source_key": sourceKey})
		return
	}
	handler.SendOKResp(c, toSourceView(item))
}

// DeleteSource 删除采集源（超管）。
func (s *svcImpl) DeleteSource(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	sourceKey := strings.TrimSpace(c.Param("key"))
	err := s.repo.DeleteSource(c.Request.Context(), sourceKey)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "采集源不存在", nil)
		return
	}
	if err != nil {
		logger.Warnw("删除采集源失败", "source_key", sourceKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除采集源失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"source_key": sourceKey})
}

// ProbeSource 采集源联通性探测（超管）：调用采集服务的发现接口，看看列表页能否抓到候选公告。
func (s *svcImpl) ProbeSource(c *gin.Context) {
	if !requireSuperAdmin(c) {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	sourceKey := strings.TrimSpace(c.Param("key"))
	src, err := s.repo.GetSource(c.Request.Context(), sourceKey)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "采集源不存在", nil)
		return
	}
	listURL := strings.TrimSpace(src.ListURL)
	if listURL == "" {
		listURL = strings.TrimSpace(src.HomepageURL)
	}
	if listURL == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "该采集源没有配置列表地址，无法探测", nil)
		return
	}

	result, err := s.collector.Discover(c.Request.Context(), collectorDiscoverRequest{
		SourceKey:     src.SourceKey,
		ListURL:       listURL,
		DiscoveryMode: src.DiscoveryMode,
		NeedsBrowser:  src.NeedsBrowser == 1,
		MaxPages:      1,
		Params:        src.Params,
	})
	if err != nil {
		logger.Warnw("采集源探测失败", "source_key", sourceKey, "err", err)
		handler.SendOKResp(c, map[string]any{
			"reachable": false,
			"message":   err.Error(),
			"probed_at": formatTime(NowFunc()),
		})
		return
	}

	titles := make([]string, 0, 3)
	for _, item := range result.Items {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		titles = append(titles, truncateRunes(item.Title, 80))
		if len(titles) >= 3 {
			break
		}
	}
	messages := make([]string, 0, len(result.Errors))
	for _, item := range result.Errors {
		messages = append(messages, item.Reason)
		if len(messages) >= 3 {
			break
		}
	}
	handler.SendOKResp(c, map[string]any{
		"reachable":   true,
		"items_found": len(result.Items),
		"sample":      titles,
		"errors":      messages,
		"cursor":      result.Cursor,
		"probed_at":   formatTime(NowFunc()),
	})
}
