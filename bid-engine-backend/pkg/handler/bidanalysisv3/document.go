package bidanalysisv3

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"bid-engine/pkg/db/model"
	repov3 "bid-engine/pkg/repo/bidanalysisv3"
	"bid-engine/pkg/repo/converter"
	"bid-engine/pkg/repo/docling"
	"bid-engine/pkg/utils"
)

const (
	maxSourceBytes    = int64(100 << 20)
	maxSourcePages    = 1000
	defaultChunkPages = 10
	maxChunkBytes     = int64(20 << 20)
)

type sourceRegion struct {
	PageNo int32   `json:"page_no"`
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func validateUploadedDocument(path, filename string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() <= 0 {
		return "", fmt.Errorf("文件为空")
	}
	if info.Size() > maxSourceBytes {
		return "", fmt.Errorf("文件超过 100MB 限制")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".pdf" && ext != ".doc" && ext != ".docx" {
		return "", fmt.Errorf("仅支持 PDF、DOC、DOCX")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	header = header[:n]
	detected := http.DetectContentType(header)
	switch ext {
	case ".pdf":
		if len(header) < 5 || string(header[:5]) != "%PDF-" {
			return "", fmt.Errorf("文件扩展名与 PDF 文件头不一致")
		}
		if detected != "application/pdf" {
			return "", fmt.Errorf("PDF MIME 类型无效: %s", detected)
		}
	case ".doc":
		if len(header) < 8 || !(header[0] == 0xD0 && header[1] == 0xCF && header[2] == 0x11 && header[3] == 0xE0) {
			return "", fmt.Errorf("文件扩展名与 DOC 文件头不一致")
		}
		if detected != "application/octet-stream" && detected != "application/x-cfb" && detected != "application/CDFV2" {
			return "", fmt.Errorf("DOC MIME 类型无效: %s", detected)
		}
	case ".docx":
		if len(header) < 4 || string(header[:2]) != "PK" {
			return "", fmt.Errorf("文件扩展名与 DOCX 文件头不一致")
		}
		if detected != "application/zip" && detected != "application/octet-stream" {
			return "", fmt.Errorf("DOCX MIME 类型无效: %s", detected)
		}
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", fmt.Errorf("DOCX 压缩结构无效: %w", err)
		}
		defer zr.Close()
		found := false
		for _, zf := range zr.File {
			if zf.Name == "word/document.xml" {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("DOCX 缺少 word/document.xml")
		}
	}
	return detected, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizeToPDF(ctx context.Context, sourcePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext == ".pdf" {
		return sourcePath, nil
	}
	return converter.GetInstance().ConvertToPDF(ctx, sourcePath)
}

func (s *Service) prepareDocument(ctx context.Context, project *model.BidAnalysisV3Project, run *model.BidAnalysisV3ParseRun, workDir string) ([]*model.BidAnalysisV3DocumentChunk, error) {
	if err := s.repo.SetStage(ctx, project.ID, run.ID, "document_preprocessing", repov3.StageRunning, 1, 0, 0, ""); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pdfPath := filepath.Join(workDir, "normalized.pdf")
	normalizedObject := project.NormalizedPdfObject
	if normalizedObject != "" && project.PageCount > 0 {
		// DOC/DOCX 只转换一次；后续重新解析直接复用项目级规范化 PDF。
		if err := s.oss.Get(ctx, normalizedObject, pdfPath); err != nil {
			return nil, fmt.Errorf("下载已缓存规范化 PDF 失败: %w", err)
		}
	} else {
		sourcePath := filepath.Join(workDir, "source"+strings.ToLower(filepath.Ext(project.SourceFileName)))
		if err := s.oss.Get(ctx, project.SourceObject, sourcePath); err != nil {
			return nil, fmt.Errorf("下载源文件失败: %w", err)
		}
		convertedPath, err := normalizeToPDF(ctx, sourcePath)
		if err != nil {
			return nil, err
		}
		if err := copyFileContext(ctx, convertedPath, pdfPath); err != nil {
			return nil, fmt.Errorf("缓存规范化 PDF 失败: %w", err)
		}
		normalizedObject = fmt.Sprintf("bid-analysis-v3/%d/normalized/source.pdf", project.ID)
		if s.projectCancelled(ctx, project.ID) {
			return nil, fmt.Errorf("项目已删除，停止上传规范化 PDF")
		}
		if err := s.oss.Put(ctx, normalizedObject, pdfPath); err != nil {
			return nil, fmt.Errorf("上传规范化 PDF 失败: %w", err)
		}
		info, err := os.Stat(pdfPath)
		if err != nil {
			s.deleteObjectBestEffort(ctx, normalizedObject)
			return nil, fmt.Errorf("读取规范化 PDF 信息失败: %w", err)
		}
		sha, err := fileSHA256(pdfPath)
		if err != nil {
			s.deleteObjectBestEffort(ctx, normalizedObject)
			return nil, fmt.Errorf("计算规范化 PDF 摘要失败: %w", err)
		}
		asset := &model.BidAnalysisV3DocumentAsset{ProjectID: project.ID, RunID: run.ID, AssetType: "normalized_pdf", Bucket: s.oss.GetDefaultBucketName(), ObjectKey: normalizedObject, FileName: "source.pdf", MimeType: "application/pdf", Sha256: sha}
		asset.SizeBytes = info.Size()
		if err := s.repo.CreateAsset(ctx, asset); err != nil {
			s.deleteObjectBestEffort(ctx, normalizedObject)
			return nil, err
		}
	}
	pageCount, err := utils.PageCount(pdfPath)
	if err != nil {
		return nil, err
	}
	if pageCount > maxSourcePages {
		return nil, fmt.Errorf("文档共 %d 页，超过 1000 页限制", pageCount)
	}
	if pageCount <= 0 {
		return nil, fmt.Errorf("规范化 PDF 没有可解析页面")
	}
	if err := s.repo.UpdateNormalizedPDF(ctx, project.ID, s.oss.GetDefaultBucketName(), normalizedObject, pageCount); err != nil {
		return nil, err
	}

	files, err := utils.SplitPDFByRangesContext(ctx, pdfPath, defaultChunkPages, int64(s.chunkSplitBytes))
	if err != nil {
		return nil, err
	}
	assets := make([]*model.BidAnalysisV3DocumentAsset, 0, len(files))
	chunks := make([]*model.BidAnalysisV3DocumentChunk, 0, len(files))
	for i, f := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		object := fmt.Sprintf("bid-analysis-v3/%d/run-%d/chunks/%04d-%04d.pdf", project.ID, run.ID, f.PageStart, f.PageEnd)
		if s.projectCancelled(ctx, project.ID) {
			for _, uploaded := range assets {
				s.deleteObjectBestEffort(ctx, uploaded.ObjectKey)
			}
			return nil, fmt.Errorf("项目已删除，停止上传 PDF 页块")
		}
		if err := s.oss.Put(ctx, object, f.Path); err != nil {
			for _, uploaded := range assets {
				s.deleteObjectBestEffort(ctx, uploaded.ObjectKey)
			}
			return nil, fmt.Errorf("上传页块 %d-%d 失败: %w", f.PageStart, f.PageEnd, err)
		}
		chunkSHA, err := fileSHA256(f.Path)
		if err != nil {
			s.deleteObjectBestEffort(ctx, object)
			for _, uploaded := range assets {
				s.deleteObjectBestEffort(ctx, uploaded.ObjectKey)
			}
			return nil, fmt.Errorf("计算页块 %d-%d 摘要失败: %w", f.PageStart, f.PageEnd, err)
		}
		assets = append(assets, &model.BidAnalysisV3DocumentAsset{ProjectID: project.ID, RunID: run.ID, AssetType: "chunk_pdf", Bucket: s.oss.GetDefaultBucketName(), ObjectKey: object, FileName: f.Name, MimeType: "application/pdf", SizeBytes: f.Size, Sha256: chunkSHA})
		chunks = append(chunks, &model.BidAnalysisV3DocumentChunk{ProjectID: project.ID, RunID: run.ID, ChunkNo: int32(i + 1), PageStart: int32(f.PageStart), PageEnd: int32(f.PageEnd), Status: "pending", Checksum: chunkSHA})
	}
	if err := s.repo.SaveChunkDefinitions(ctx, assets, chunks); err != nil {
		for _, uploaded := range assets {
			s.deleteObjectBestEffort(ctx, uploaded.ObjectKey)
		}
		return nil, err
	}
	if err := s.repo.SetRunDocumentTotals(ctx, run.ID, pageCount, len(chunks)); err != nil {
		return nil, err
	}
	if err := s.repo.SetStage(ctx, project.ID, run.ID, "document_preprocessing", repov3.StageSucceeded, 1, 1, 0, ""); err != nil {
		return nil, err
	}
	return chunks, nil
}

func copyFile(source, target string) error {
	return copyFileContext(context.Background(), source, target)
}

func copyFileContext(ctx context.Context, source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			if closeErr := out.Close(); closeErr != nil {
				return errors.Join(err, closeErr)
			}
			return err
		}
		n, readErr := in.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				if closeErr := out.Close(); closeErr != nil {
					return errors.Join(writeErr, closeErr)
				}
				return writeErr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if closeErr := out.Close(); closeErr != nil {
				return errors.Join(readErr, closeErr)
			}
			return readErr
		}
	}
	return out.Close()
}

func (s *Service) parseChunk(ctx context.Context, projectID, runID int64, chunk *model.BidAnalysisV3DocumentChunk, workDir string) error {
	localPath := filepath.Join(workDir, fmt.Sprintf("chunk-%04d.pdf", chunk.ChunkNo))
	var asset model.BidAnalysisV3DocumentAsset
	if err := s.repo.DB().WithContext(ctx).First(&asset, chunk.PdfAssetID).Error; err != nil {
		return err
	}
	if err := s.oss.Get(ctx, asset.ObjectKey, localPath); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	started := time.Now()
	result, doc, ocrUsed, actualAttempts, err := parseChunkWithRetry(ctx, s.docling, localPath, int(chunk.PageEnd-chunk.PageStart+1))
	if err != nil {
		return err
	}
	rawObject := fmt.Sprintf("bid-analysis-v3/%d/run-%d/docling/chunk-%04d.json", projectID, runID, chunk.ChunkNo)
	if deleted, _ := s.redis.Client().Exists(ctx, fmt.Sprintf("cancel:tender_parse_v3:%d", projectID)).Result(); deleted > 0 {
		return fmt.Errorf("项目已删除，停止写入解析结果")
	}
	rawPath := filepath.Join(workDir, fmt.Sprintf("docling-%04d.json", chunk.ChunkNo))
	if err := os.WriteFile(rawPath, result.JSON, 0o600); err != nil {
		return err
	}
	if err := s.oss.Put(ctx, rawObject, rawPath); err != nil {
		return err
	}
	rawSHA, err := fileSHA256(rawPath)
	if err != nil {
		s.deleteObjectBestEffort(ctx, rawObject)
		return fmt.Errorf("计算 Docling JSON 摘要失败: %w", err)
	}
	rawAsset := &model.BidAnalysisV3DocumentAsset{ProjectID: projectID, RunID: runID, AssetType: "docling_json", Bucket: s.oss.GetDefaultBucketName(), ObjectKey: rawObject, FileName: filepath.Base(rawPath), MimeType: "application/json", SizeBytes: int64(len(result.JSON)), Sha256: rawSHA}
	if err := s.repo.CreateAsset(ctx, rawAsset); err != nil {
		s.deleteObjectBestEffort(ctx, rawObject)
		return err
	}
	index := buildChunkIndex(projectID, runID, chunk, doc, ocrUsed)
	index.Chunk.DoclingAssetID = rawAsset.ID
	index.Chunk.Attempts = int32(actualAttempts)
	index.Chunk.ProcessingMs = time.Since(started).Milliseconds()
	index.Chunk.Checksum = rawSHA
	if err := s.repo.SaveChunkIndex(ctx, index); err != nil {
		return err
	}
	return s.repo.MarkParsedProgress(ctx, runID, chunk.PageEnd-chunk.PageStart+1)
}

// doclingRetryBackoff Docling 瞬时故障（超时/连接/5xx）重试前的等待时长，
// 避免短时抖动立即重打服务；内容/格式错误保持立即重试。
var doclingRetryBackoff = []time.Duration{3 * time.Second, 15 * time.Second}

// doclingTransientError 判断 Docling 失败是否属于瞬时故障：超时、连接/网络错误、
// 限流与服务端 5xx 值得退避重试；JSON 内容损坏、页数不完整等确定性错误立即重试。
func doclingTransientError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection") ||
		strings.Contains(msg, "status=429") ||
		strings.Contains(msg, "status=500") ||
		strings.Contains(msg, "status=502") ||
		strings.Contains(msg, "status=503") ||
		strings.Contains(msg, "status=504") ||
		strings.Contains(msg, "api call failed")
}

// parseChunkWithRetry 对单个页块执行最多 3 次完整解析尝试。每次尝试覆盖
// Docling 解析、JSON 反序列化、页数校验与稀疏页 OCR 分支；失败统一重试，
// 避免 JSON 反序列化失败、页数不完整等可重试失败直接报废整个页块。
func parseChunkWithRetry(ctx context.Context, svc docling.Service, localPath string, expectedPages int) (result *docling.ParseResult, doc *docling.DoclingDocument, ocrUsed bool, attempts int, err error) {
	for attempt := 1; attempt <= 3; attempt++ {
		if err = ctx.Err(); err != nil {
			return nil, nil, false, attempt, err
		}
		attempts = attempt
		result, doc, ocrUsed, err = parseChunkOnce(ctx, svc, localPath, expectedPages)
		if err == nil {
			return result, doc, ocrUsed, attempts, nil
		}
		if attempt < 3 && doclingTransientError(err) {
			select {
			case <-ctx.Done():
				return nil, nil, false, attempt, ctx.Err()
			case <-time.After(doclingRetryBackoff[attempt-1]):
			}
		}
	}
	return nil, nil, false, attempts, err
}

// parseChunkOnce 执行一次完整的页块解析尝试：
// Docling 解析 → ParseDocument → 页数校验 →（文本稀疏时）OCR 重解析与校验。
func parseChunkOnce(ctx context.Context, svc docling.Service, localPath string, expectedPages int) (*docling.ParseResult, *docling.DoclingDocument, bool, error) {
	result, err := svc.Parse(ctx, localPath, &docling.ParseOptions{ToFormats: []string{"json"}, TableMode: "accurate", DocumentTimeout: 600})
	if err != nil {
		return nil, nil, false, err
	}
	if result == nil {
		return nil, nil, false, fmt.Errorf("Docling 返回空结果")
	}
	if result.Status != "success" {
		return nil, nil, false, fmt.Errorf("Docling 返回 %s: %+v", result.Status, result.Errors)
	}
	doc, err := result.ParseDocument()
	if err != nil {
		return nil, nil, false, fmt.Errorf("解析 Docling JSON 失败: %w", err)
	}
	if err := validateDoclingCoverage(doc, expectedPages); err != nil {
		return nil, nil, false, err
	}
	ocrUsed := false
	if documentTextChars(doc) < expectedPages*40 {
		ocr := true
		ocrResult, ocrErr := svc.Parse(ctx, localPath, &docling.ParseOptions{ToFormats: []string{"json"}, TableMode: "accurate", DoOCR: &ocr, OCRLang: []string{"zh", "en"}, DocumentTimeout: 600})
		if ocrErr != nil {
			return nil, nil, false, fmt.Errorf("稀疏页块 OCR 解析失败: %w", ocrErr)
		}
		if ocrResult == nil || ocrResult.Status != "success" {
			return nil, nil, false, fmt.Errorf("稀疏页块 OCR 未成功")
		}
		ocrDoc, err := ocrResult.ParseDocument()
		if err != nil {
			return nil, nil, false, fmt.Errorf("解析 OCR Docling JSON 失败: %w", err)
		}
		if err := validateDoclingCoverage(ocrDoc, expectedPages); err != nil {
			return nil, nil, false, err
		}
		result, doc = ocrResult, ocrDoc
		ocrUsed = true
	}
	return result, doc, ocrUsed, nil
}

func documentTextChars(doc *docling.DoclingDocument) int {
	total := 0
	for _, t := range doc.Texts {
		total += len([]rune(strings.TrimSpace(t.Text)))
	}
	return total
}

func validateDoclingCoverage(doc *docling.DoclingDocument, expectedPages int) error {
	if doc == nil || len(doc.Pages) != expectedPages {
		actual := 0
		if doc != nil {
			actual = len(doc.Pages)
		}
		return fmt.Errorf("Docling 页数不完整：期望 %d 页，实际 %d 页", expectedPages, actual)
	}
	for page := 1; page <= expectedPages; page++ {
		if _, ok := doc.Pages[strconv.Itoa(page)]; !ok {
			return fmt.Errorf("Docling 缺少页 %d", page)
		}
	}
	return nil
}

func buildChunkIndex(projectID, runID int64, chunk *model.BidAnalysisV3DocumentChunk, doc *docling.DoclingDocument, ocrUsed bool) repov3.ChunkIndex {
	index := repov3.ChunkIndex{Chunk: chunk}
	pageKeys := make([]int, 0, len(doc.Pages))
	for key := range doc.Pages {
		if n, err := strconv.Atoi(key); err == nil {
			pageKeys = append(pageKeys, n)
		}
	}
	sort.Ints(pageKeys)
	charsByPage := map[int]int{}
	for _, text := range doc.Texts {
		for _, p := range text.Prov {
			charsByPage[p.PageNo] += len([]rune(text.Text))
		}
	}
	for _, local := range pageKeys {
		p := doc.Pages[strconv.Itoa(local)]
		index.Pages = append(index.Pages, &model.BidAnalysisV3DocumentPage{ProjectID: projectID, RunID: runID, ChunkID: chunk.ID, PageNo: chunk.PageStart + int32(local) - 1, Width: p.Size.Width, Height: p.Size.Height, TextChars: int32(charsByPage[local]), OcrUsed: ocrUsed})
	}
	order := int32(0)
	for i, item := range doc.Texts {
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		provs := item.Prov
		if len(provs) == 0 {
			continue
		}
		for pi, prov := range provs {
			globalPage := chunk.PageStart + int32(prov.PageNo) - 1
			page := doc.Pages[strconv.Itoa(prov.PageNo)]
			l, t, w, h := normalizeBBox(prov.BBox, page.Size)
			parent := ""
			if item.Parent != nil {
				parent = globalRef(runID, chunk.ChunkNo, item.Parent.Ref)
			}
			raw, _ := json.Marshal(item)
			text := item.Text
			if prov.CharSpan[0] >= 0 && prov.CharSpan[1] > prov.CharSpan[0] {
				runes := []rune(item.Text)
				if prov.CharSpan[0] < len(runes) && prov.CharSpan[1] <= len(runes) {
					text = string(runes[prov.CharSpan[0]:prov.CharSpan[1]])
				}
			}
			selfRef := item.SelfRef
			if selfRef == "" {
				selfRef = fmt.Sprintf("texts/%d", i)
			}
			order++
			index.Blocks = append(index.Blocks, &model.BidAnalysisV3DocumentBlock{ProjectID: projectID, RunID: runID, ChunkID: chunk.ID, BlockRef: fmt.Sprintf("%s:prov:%02d", globalRef(runID, chunk.ChunkNo, selfRef), pi), ParentRef: parent, PageNo: globalPage, BlockType: "text", Label: item.Label, Text: text, OriginalText: item.Orig, BboxLeft: l, BboxTop: t, BboxWidth: w, BboxHeight: h, SortOrder: order, ContentHash: hashText(text), RawJSON: string(raw)})
		}
	}
	for i, item := range doc.Tables {
		if len(item.Prov) == 0 {
			continue
		}
		pageStart, pageEnd := int32(math.MaxInt32), int32(0)
		regions := make([]sourceRegion, 0, len(item.Prov))
		for _, prov := range item.Prov {
			pageNo := chunk.PageStart + int32(prov.PageNo) - 1
			if pageNo < pageStart {
				pageStart = pageNo
			}
			if pageNo > pageEnd {
				pageEnd = pageNo
			}
			page := doc.Pages[strconv.Itoa(prov.PageNo)]
			left, top, width, height := normalizeBBox(prov.BBox, page.Size)
			regions = append(regions, sourceRegion{PageNo: pageNo, Left: left, Top: top, Width: width, Height: height})
		}
		first := regions[0]
		data, _ := json.Marshal(item.Data)
		regionsJSON, _ := json.Marshal(regions)
		captions := make([]string, 0, len(item.Captions))
		for _, c := range item.Captions {
			captions = append(captions, c.Text)
		}
		selfRef := item.SelfRef
		if selfRef == "" {
			selfRef = fmt.Sprintf("tables/%d", i)
		}
		order++
		index.Tables = append(index.Tables, &model.BidAnalysisV3SourceTable{ProjectID: projectID, RunID: runID, ChunkID: chunk.ID, TableRef: globalRef(runID, chunk.ChunkNo, selfRef), Label: item.Label, Caption: strings.Join(captions, " "), PageStart: pageStart, PageEnd: pageEnd, RowCount: int32(item.Data.NumRows), ColumnCount: int32(item.Data.NumCols), DataJSON: string(data), RegionsJSON: string(regionsJSON), BboxLeft: first.Left, BboxTop: first.Top, BboxWidth: first.Width, BboxHeight: first.Height, SortOrder: order, ContentHash: hashText(string(data))})
	}
	index.Chunk.TextChars = int32(documentTextChars(doc))
	return index
}

func normalizeBBox(box docling.BBox, page docling.PageSize) (left, top, width, height float64) {
	if page.Width <= 0 || page.Height <= 0 {
		return 0, 0, 1, 1
	}
	minX, maxX := math.Min(box.L, box.R), math.Max(box.L, box.R)
	minY, maxY := math.Min(box.T, box.B), math.Max(box.T, box.B)
	left, width, height = minX/page.Width, (maxX-minX)/page.Width, (maxY-minY)/page.Height
	if strings.EqualFold(box.CoordOrigin, "BOTTOMLEFT") {
		top = (page.Height - maxY) / page.Height
	} else {
		top = minY / page.Height
	}
	return clamp01(left), clamp01(top), clamp01(width), clamp01(height)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
func hashText(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }

func (s *Service) deleteObjectBestEffort(ctx context.Context, objectKey string) {
	if err := s.oss.Delete(ctx, objectKey); err != nil {
		s.logger.Warnw("清理未落库的 V3 对象失败", "object_key", objectKey, "err", err)
	}
}

func sourceTableRegions(table *model.BidAnalysisV3SourceTable) []sourceRegion {
	var regions []sourceRegion
	if table != nil && json.Unmarshal([]byte(table.RegionsJSON), &regions) == nil && len(regions) > 0 {
		return regions
	}
	if table == nil {
		return nil
	}
	return []sourceRegion{{PageNo: table.PageStart, Left: table.BboxLeft, Top: table.BboxTop, Width: table.BboxWidth, Height: table.BboxHeight}}
}

func (s *Service) projectCancelled(ctx context.Context, projectID int64) bool {
	count, err := s.redis.Client().Exists(ctx, fmt.Sprintf("cancel:tender_parse_v3:%d", projectID)).Result()
	return err == nil && count > 0
}
func globalRef(runID int64, chunkNo int32, ref string) string {
	return fmt.Sprintf("run:%d:chunk:%04d:%s", runID, chunkNo, strings.TrimPrefix(ref, "#/"))
}
