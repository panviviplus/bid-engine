package bidgen

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/repo/converter"
	"bid-engine/pkg/repo/docling"
	"bid-engine/pkg/service/biddoc"
)

// pageMapEntry 单个大纲章节在导出文档中的起始页码
type pageMapEntry struct {
	OutlineID int64  `json:"outlineId"`
	Title     string `json:"title"`
	Level     int32  `json:"level"`
	Page      int    `json:"page"`
}

// ExportPageMap 两遍导出的第一遍：接收前端生成的 DOCX，转 PDF 后回读每个章节的起始页码，
// 供第二遍导出写入目录真实页码。页码只在正文区匹配，避免命中目录页自身。
func (s *svcImpl) ExportPageMap(c *gin.Context) {
	projectID := parseInt64(c.PostForm("projectId"))
	if projectID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "projectId 不能为空"})
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	proj, err := s.repo.GetProjectForUser(c, userID, projectID)
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "项目不存在"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请上传 DOCX 文件"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".docx" && ext != ".doc" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "仅支持 docx/doc 文件"})
		return
	}

	_ = os.MkdirAll("./tmp", 0777)
	localDoc := filepath.Join("./tmp", fmt.Sprintf("bidgen-pagemap-%d%s", time.Now().UnixNano(), ext))
	if err := c.SaveUploadedFile(file, localDoc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "文件保存失败: " + err.Error()})
		return
	}
	defer os.Remove(localDoc)

	pdfPath, err := converter.GetInstance().ConvertToPDF(entity.ConvertContext(c), localDoc)
	if err != nil {
		s.logger.Warnw("导出页码定位失败：DOCX 转 PDF 失败", "project_id", projectID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "页码定位失败，请确认文档转换服务（doc-converter）已启动"})
		return
	}
	defer os.Remove(pdfPath)

	pageTexts, err := s.pdfPageTexts(c, pdfPath)
	if err != nil {
		s.logger.Warnw("导出页码定位失败：PDF 文本解析失败", "project_id", projectID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "页码定位失败: " + err.Error()})
		return
	}

	nodes, err := s.repo.GetOutlineByProjectID(c, proj.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加载大纲失败"})
		return
	}
	ordered, err := biddoc.OrderOutlineTree(nodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "大纲结构异常"})
		return
	}
	entries := locateChapterPages(ordered, pageTexts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"pages": entries, "totalPages": len(pageTexts)}})
}

// pdfPageTexts 用 Docling 解析 PDF 并按页聚合文本（页码从 1 开始）。
func (s *svcImpl) pdfPageTexts(c *gin.Context, pdfPath string) ([]string, error) {
	doOCR := false
	result, err := s.docling.Parse(c.Request.Context(), pdfPath, &docling.ParseOptions{DoOCR: &doOCR})
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.JSON) == 0 {
		return nil, fmt.Errorf("PDF 无解析结果")
	}
	var document docling.DoclingDocument
	if err := json.Unmarshal(result.JSON, &document); err != nil {
		return nil, err
	}
	maxPage := 0
	type pageText struct {
		page   int
		values []string
	}
	collected := make([]pageText, 0, len(document.Pages))
	for _, item := range document.Texts {
		if strings.TrimSpace(item.Text) == "" || len(item.Prov) == 0 {
			continue
		}
		page := item.Prov[0].PageNo
		if page <= 0 {
			continue
		}
		if page > maxPage {
			maxPage = page
		}
		collected = append(collected, pageText{page: page, values: []string{item.Text}})
	}
	if maxPage == 0 {
		return nil, fmt.Errorf("PDF 未解析出带页码的文本")
	}
	pages := make([]string, maxPage)
	for _, item := range collected {
		pages[item.page-1] += strings.Join(item.values, "")
	}
	return pages, nil
}

// locateChapterPages 在正文区匹配章节标题并返回起始页码。
// 目录页会包含大量章节标题，因此先识别出目录页，再从其后开始匹配。
func locateChapterPages(ordered []*model.BidGenOutline, pageTexts []string) []pageMapEntry {
	// 需要定位的章节：跳过系统根节点
	targets := make([]*model.BidGenOutline, 0, len(ordered))
	for _, node := range ordered {
		if biddoc.IsDocumentRoot(node) {
			continue
		}
		targets = append(targets, node)
	}
	if len(targets) == 0 || len(pageTexts) == 0 {
		return nil
	}
	normalizedPages := make([]string, len(pageTexts))
	for i, text := range pageTexts {
		normalizedPages[i] = normalizeForMatch(text)
	}

	// 目录页识别：前若干页中命中章节标题数量异常多的页
	threshold := len(targets) / 4
	if threshold < 3 {
		threshold = 3
	}
	if threshold > 6 {
		threshold = 6
	}
	scanLimit := len(normalizedPages)
	if scanLimit > 8 {
		scanLimit = 8
	}
	frontMatterEnd := 0
	for page := 0; page < scanLimit; page++ {
		hits := 0
		for _, node := range targets {
			if matchTitle(normalizedPages[page], node.Title) {
				hits++
			}
		}
		if hits >= threshold {
			frontMatterEnd = page + 1
		}
	}

	entries := make([]pageMapEntry, 0, len(targets))
	for _, node := range targets {
		page := findTitlePage(normalizedPages, node.Title, frontMatterEnd)
		if page <= 0 {
			// 目录页未识别成功时，退回全篇匹配
			page = findTitlePage(normalizedPages, node.Title, 0)
		}
		if page <= 0 {
			continue
		}
		entries = append(entries, pageMapEntry{
			OutlineID: node.ID,
			Title:     node.Title,
			Level:     node.Level,
			Page:      page,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Page < entries[j].Page })
	return entries
}

// matchTitle 判断页面文本是否包含章节标题（按归一化文本匹配）。
func matchTitle(normalizedPage, title string) bool {
	target := normalizeForMatch(title)
	if target == "" || normalizedPage == "" {
		return false
	}
	if strings.Contains(normalizedPage, target) {
		return true
	}
	// 标题过长时按前缀匹配（PDF 中标题可能被换行或分栏打断）
	runes := []rune(target)
	if len(runes) > 8 {
		prefix := string(runes[:8])
		if strings.Contains(normalizedPage, prefix) {
			return true
		}
	}
	return false
}

func findTitlePage(normalizedPages []string, title string, fromPage int) int {
	for page := fromPage; page < len(normalizedPages); page++ {
		if matchTitle(normalizedPages[page], title) {
			return page + 1
		}
	}
	return 0
}
