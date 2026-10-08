package bidreview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	bidreviewRepo "bid-engine/pkg/repo/bidreview"
	"bid-engine/pkg/repo/docling"
)

// ================================================================
// 文件解析：PDF/DOCX → 逐页文本 + 页块（含坐标）
// ================================================================

// parseTarget 解析目标（来源对象 / 已转换 PDF）
type parseTarget struct {
	FileID        int64
	FileName      string
	FileObject    string
	PdfObject     string
	IncludeTables bool
	ExpectedPages int
}

type parsedBlock struct {
	PageNo    int
	Type      string
	Content   string
	Left      float64
	Top       float64
	Width     float64
	Height    float64
	SortOrder int32
}

type parsedPage struct {
	PageNo  int
	Content string
	Width   float64
	Height  float64
	Blocks  []parsedBlock
}

type parsedDoc struct {
	Pages []parsedPage
}

// parseTargetDoc 下载并解析目标文件（优先已转换 PDF，非 PDF 现场转换兜底）
func (s *svcImpl) parseTargetDoc(ctx context.Context, t *parseTarget) (*parsedDoc, error) {
	parseCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	_ = os.MkdirAll("./tmp", 0o777)
	ext := strings.ToLower(filepath.Ext(t.FileName))

	// 1. 已转换 PDF 优先（页码与预览一致，规避 .doc 等旧格式）
	if ext != ".pdf" && strings.TrimSpace(t.PdfObject) != "" {
		localPdf := filepath.Join("./tmp", fmt.Sprintf("bid-review-parse-%d-%d.pdf", t.FileID, time.Now().UnixNano()))
		if err := s.oss.Get(parseCtx, t.PdfObject, localPdf); err == nil {
			doc, perr := s.parseLocalDoc(parseCtx, t, localPdf)
			_ = os.Remove(localPdf)
			if perr == nil {
				return doc, nil
			}
		}
	}

	// 2. 原始文件
	localSrc := filepath.Join("./tmp", fmt.Sprintf("bid-review-parse-%d-%d%s", t.FileID, time.Now().UnixNano(), ext))
	if err := s.oss.Get(parseCtx, t.FileObject, localSrc); err != nil {
		return nil, fmt.Errorf("下载文件 %s 失败: %w", t.FileName, err)
	}
	defer func() { _ = os.Remove(localSrc) }()
	doc, perr := s.parseLocalDoc(parseCtx, t, localSrc)
	if perr == nil {
		return doc, nil
	}

	// 3. 非 PDF 且原文件解析为空 → 现场转 PDF 再解析
	if ext != ".pdf" {
		localPdf := filepath.Join("./tmp", fmt.Sprintf("bid-review-parse-%d-%d.pdf", t.FileID, time.Now().UnixNano()))
		gc := makeGinCtx(parseCtx)
		convErr := s.pdf.Convert2Pdf(gc, localSrc, localPdf)
		if convErr != nil {
			convErr = s.pdf.Convert2PdfBySoffice(gc, localSrc, localPdf)
		}
		if convErr == nil {
			doc2, perr2 := s.parseLocalDoc(parseCtx, t, localPdf)
			_ = os.Remove(localPdf)
			if perr2 == nil {
				return doc2, nil
			}
		}
	}
	return nil, perr
}

// parseLocalDoc 解析本地文件为逐页文本与页块（Docling 优先，空结果按全文分块兜底）
func (s *svcImpl) parseLocalDoc(ctx context.Context, t *parseTarget, localPath string) (*parsedDoc, error) {
	parseCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var opts *docling.ParseOptions
	if t.IncludeTables {
		opts = &docling.ParseOptions{ToFormats: []string{"json"}, TableMode: "accurate", DocumentTimeout: 600}
	}
	result, err := s.docling.Parse(parseCtx, localPath, opts)
	if err != nil {
		return nil, fmt.Errorf("解析文件 %s 失败: %w", t.FileName, err)
	}
	if result == nil {
		return nil, fmt.Errorf("解析文件 %s 返回空结果", t.FileName)
	}
	if t.ExpectedPages > 0 {
		parsed, parseErr := result.ParseDocument()
		textChars := 0
		if parsed != nil {
			for _, item := range parsed.Texts {
				textChars += len([]rune(strings.TrimSpace(item.Text)))
			}
			for _, table := range parsed.Tables {
				for _, cell := range table.Data.TableCells {
					textChars += len([]rune(strings.TrimSpace(cell.Text)))
				}
			}
		}
		if result.Status != "success" || parseErr != nil || !tenderChunkHasEveryPage(parsed, t.ExpectedPages) || textChars < t.ExpectedPages*40 {
			ocr := true
			ocrOpts := &docling.ParseOptions{ToFormats: []string{"json"}, TableMode: "accurate", DoOCR: &ocr, OCRLang: []string{"zh", "en"}, DocumentTimeout: 600}
			result, err = s.docling.Parse(parseCtx, localPath, ocrOpts)
			if err != nil {
				return nil, fmt.Errorf("招标文件页块 OCR 失败: %w", err)
			}
			if result == nil {
				return nil, fmt.Errorf("招标文件页块 OCR 返回空结果")
			}
			if result.Status != "success" {
				return nil, fmt.Errorf("招标文件页块 OCR 未完成: %s", result.Status)
			}
			parsed, parseErr = result.ParseDocument()
			if parseErr != nil {
				return nil, fmt.Errorf("读取招标文件页块结果失败: %w", parseErr)
			}
			if !tenderChunkHasEveryPage(parsed, t.ExpectedPages) {
				return nil, fmt.Errorf("招标文件页块缺页，期望 %d 页", t.ExpectedPages)
			}
		}
	}

	doc := &parsedDoc{}
	if parsed, derr := result.ParseDocument(); derr == nil {
		for _, pt := range parsed.ToPageTexts() {
			page := parsedPage{PageNo: pt.PageNum, Width: pt.Width, Height: pt.Height}
			var sb strings.Builder
			for idx, item := range pt.Items {
				if strings.TrimSpace(item.Text) == "" {
					continue
				}
				blockType := "text"
				switch strings.ToLower(strings.TrimSpace(item.Label)) {
				case "title", "section_header", "page_header":
					blockType = "title"
				case "table":
					blockType = "table"
				}
				page.Blocks = append(page.Blocks, parsedBlock{
					PageNo: pt.PageNum, Type: blockType, Content: item.Text,
					Left: item.BBox.L, Top: item.BBox.T,
					Width:     item.BBox.R - item.BBox.L,
					Height:    item.BBox.B - item.BBox.T,
					SortOrder: int32(idx),
				})
				sb.WriteString(item.Text)
				sb.WriteString("\n")
			}
			page.Content = strings.TrimSpace(sb.String())
			if page.Content != "" {
				doc.Pages = append(doc.Pages, page)
			}
		}
		if t.IncludeTables {
			appendTenderTables(doc, parsed.Tables)
		}
		// 页眉/页脚/水印等 furniture 文本单独落块（暗标版式检查需要）
		for _, item := range parsed.Texts {
			layer := strings.ToLower(strings.TrimSpace(item.ContentLayer))
			label := strings.ToLower(strings.TrimSpace(item.Label))
			isFurniture := layer == "furniture" || label == "page_header" || label == "page_footer"
			if !isFurniture || len(item.Prov) == 0 || strings.TrimSpace(item.Text) == "" {
				continue
			}
			pageNo := item.Prov[0].PageNo
			for idx := range doc.Pages {
				if doc.Pages[idx].PageNo == pageNo {
					doc.Pages[idx].Blocks = append(doc.Pages[idx].Blocks, parsedBlock{
						PageNo: pageNo, Type: "furniture", Content: item.Text, SortOrder: 9000,
					})
					break
				}
			}
		}
	}

	// 兜底：使用全文纯文本按 ~3000 字模拟分页（无坐标）
	if len(doc.Pages) == 0 && strings.TrimSpace(result.Text) != "" {
		const chunkSize = 3000
		runes := []rune(result.Text)
		total := (len(runes) + chunkSize - 1) / chunkSize
		for i := 0; i < total; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			content := strings.TrimSpace(string(runes[start:end]))
			if content == "" {
				continue
			}
			doc.Pages = append(doc.Pages, parsedPage{PageNo: i + 1, Content: content})
		}
	}
	if len(doc.Pages) == 0 {
		return nil, fmt.Errorf("文件 %s 解析内容为空", t.FileName)
	}
	return doc, nil
}

// ================================================================
// 分块与术语倒排
// ================================================================

// buildDocumentPayload 把解析结果转成落库载荷（页 / 块 / 分块 / 术语索引）
func buildDocumentPayload(projectID, fileID int64, doc *parsedDoc, chunkChars, overlap int) *bidreviewRepo.DocumentPayload {
	if chunkChars <= 0 {
		chunkChars = 1800
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkChars {
		overlap = chunkChars / 5
	}
	payload := &bidreviewRepo.DocumentPayload{
		Pages:  make([]*model.BidReviewV2DocumentPage, 0, len(doc.Pages)),
		Blocks: make([]*model.BidReviewV2DocumentBlock, 0),
		Chunks: make([]*model.BidReviewV2DocumentChunk, 0),
		Terms:  make([]*bidreviewRepo.TermSeed, 0),
	}
	for _, p := range doc.Pages {
		payload.Pages = append(payload.Pages, &model.BidReviewV2DocumentPage{
			ProjectID: projectID, FileID: fileID, PageNo: int32(p.PageNo),
			Content: p.Content, CharCount: int32(len([]rune(p.Content))), Width: p.Width, Height: p.Height,
		})
		for _, b := range p.Blocks {
			payload.Blocks = append(payload.Blocks, &model.BidReviewV2DocumentBlock{
				ProjectID: projectID, FileID: fileID, PageNo: int32(b.PageNo), BlockType: b.Type,
				Content: b.Content, BBoxLeft: b.Left, BBoxTop: b.Top, BBoxWidth: b.Width, BBoxHeight: b.Height,
				SortOrder: b.SortOrder,
			})
		}
	}

	// 按页顺序拼接分块（保持页码区间，便于回原文定位）
	type pageUnit struct {
		pageNo  int
		content string
	}
	units := make([]pageUnit, 0, len(doc.Pages))
	for _, p := range doc.Pages {
		if strings.TrimSpace(p.Content) == "" {
			continue
		}
		units = append(units, pageUnit{pageNo: p.PageNo, content: p.Content})
	}

	chunkNo := int32(0)
	var buf strings.Builder
	startPage, endPage := 0, 0
	flush := func() {
		content := strings.TrimSpace(buf.String())
		if content == "" {
			return
		}
		chunkNo++
		payload.Chunks = append(payload.Chunks, &model.BidReviewV2DocumentChunk{
			ProjectID: projectID, FileID: fileID, ChunkNo: chunkNo,
			PageStart: int32(startPage), PageEnd: int32(endPage),
			Content: content, CharCount: int32(len([]rune(content))),
		})
		for _, term := range extractWeightedTerms(content) {
			payload.Terms = append(payload.Terms, &bidreviewRepo.TermSeed{
				ChunkNo: chunkNo, Term: term.text, PageNo: int32(startPage), Weight: term.weight,
			})
		}
		// 保留尾部重叠，避免跨块切断语义
		tail := tailRunes(content, overlap)
		buf.Reset()
		if tail != "" {
			buf.WriteString(tail)
		}
	}

	for _, u := range units {
		if endPage == 0 {
			startPage = u.pageNo
		}
		endPage = u.pageNo
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(u.content)
		if len([]rune(buf.String())) >= chunkChars {
			flush()
			startPage = endPage
		}
	}
	flush()
	return payload
}

func tailRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return ""
	}
	return string(runes[len(runes)-n:])
}

// ================================================================
// 术语抽取（中文 2/3-gram + 英文数字词，无需外部分词器）
// ================================================================

type termWeight struct {
	text   string
	weight int32
}

var reviewStopwords = map[string]struct{}{
	"我们": {}, "他们": {}, "以及": {}, "并且": {}, "或者": {}, "如果": {}, "因此": {},
	"进行": {}, "相关": {}, "有关": {}, "上述": {}, "如下": {}, "以下": {}, "根据": {},
	"可以": {}, "应当": {}, "必须": {}, "需要": {}, "要求": {}, "我方": {}, "贵方": {},
}

// extractTerms 抽取可索引术语（去重，最多返回 240 个）
func extractTerms(text string) []string {
	weighted := extractWeightedTerms(text)
	out := make([]string, 0, len(weighted))
	for _, t := range weighted {
		out = append(out, t.text)
	}
	return out
}

// extractWeightedTerms 带权重版本（用于写倒排索引）
func extractWeightedTerms(text string) []termWeight {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	seen := make(map[string]int32, 256)
	var order []string
	add := func(term string, weight int32) {
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		if _, stop := reviewStopwords[term]; stop {
			return
		}
		if old, ok := seen[term]; ok {
			if weight > old {
				seen[term] = weight
			}
			return
		}
		seen[term] = weight
		order = append(order, term)
	}

	runes := []rune(text)
	var cjkRun []rune
	var asciiRun []rune
	flushCJK := func() {
		if len(cjkRun) >= 2 {
			for i := 0; i+2 <= len(cjkRun); i++ {
				add(string(cjkRun[i:i+2]), 1)
			}
			for i := 0; i+3 <= len(cjkRun); i++ {
				add(string(cjkRun[i:i+3]), 3)
			}
			if len(cjkRun) <= 8 {
				add(string(cjkRun), 4)
			}
		}
		cjkRun = cjkRun[:0]
	}
	flushASCII := func() {
		if len(asciiRun) >= 2 {
			add(strings.ToLower(string(asciiRun)), 3)
		}
		asciiRun = asciiRun[:0]
	}

	for _, r := range runes {
		switch {
		case unicode.Is(unicode.Han, r):
			flushASCII()
			cjkRun = append(cjkRun, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			asciiRun = append(asciiRun, r)
		default:
			flushCJK()
			flushASCII()
		}
	}
	flushCJK()
	flushASCII()

	out := make([]termWeight, 0, len(order))
	for _, term := range order {
		out = append(out, termWeight{text: term, weight: seen[term]})
	}
	if len(out) > 240 {
		out = out[:240]
	}
	return out
}

// selectQueryTerms 为清单项挑选召回关键词（优先长词/高权重词）
func selectQueryTerms(text string, limit int) []string {
	weighted := extractWeightedTerms(text)
	if limit <= 0 {
		limit = 16
	}
	// 按权重降序 + 词长降序，取前 limit 个
	for i := 1; i < len(weighted); i++ {
		for j := i; j > 0; j-- {
			a, b := weighted[j-1], weighted[j]
			if a.weight < b.weight || (a.weight == b.weight && len([]rune(a.text)) < len([]rune(b.text))) {
				weighted[j-1], weighted[j] = b, a
				continue
			}
			break
		}
	}
	if len(weighted) > limit {
		weighted = weighted[:limit]
	}
	out := make([]string, 0, len(weighted))
	for _, t := range weighted {
		out = append(out, t.text)
	}
	return out
}

// makeGinCtx 创建最小化 gin.Context（供 PDF 转换等内部方法复用）
func makeGinCtx(ctx context.Context) *gin.Context {
	w := httptest.NewRecorder()
	ac, _ := gin.CreateTestContext(w)
	ac.Request, _ = http.NewRequestWithContext(ctx, "POST", "/internal/bid-review", nil)
	return ac
}
