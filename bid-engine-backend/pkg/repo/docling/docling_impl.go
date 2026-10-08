package docling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// API 常量
// =============================================================================

const (
	convertFileAPI = "/v1/convert/file"
	healthAPI      = "/health"
)

// =============================================================================
// 公开方法
// =============================================================================

// Parse 解析文档文件，返回完整结构化结果。
//
// 内部使用 POST /v1/convert/file（同步 multipart），
// 请求 json + md + html + text 四种格式，使用 accurate 表格模式。
func (s *svcImpl) Parse(ctx context.Context, filePath string, opts *ParseOptions) (*ParseResult, error) {
	log := s.logger.With("file", filePath)

	// === 构建 multipart 请求 ===
	payload, contentType, err := s.buildMultipart(filePath, opts)
	if err != nil {
		log.Warnw("构建multipart请求失败", "err", err)
		return nil, err
	}

	url := strings.TrimRight(_doclingBaseURL, "/") + convertFileAPI
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
	if err != nil {
		log.Warnw("创建HTTP请求失败", "err", err)
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	// === 发送请求 ===
	resp, err := s.client.Do(req)
	if err != nil {
		log.Warnw("调用Docling API失败", "err", err)
		return nil, fmt.Errorf("docling api call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnw("读取响应失败", "err", err)
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// === 错误处理 ===
	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
		log.Warnw("Docling处理超时", "status", resp.StatusCode)
		return nil, fmt.Errorf("docling processing timed out (status=%d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		log.Warnw("Docling API返回非200", "status", resp.StatusCode, "body_snippet", snippet)
		return nil, fmt.Errorf("docling api status=%d: %s", resp.StatusCode, snippet)
	}

	// === 解析响应 ===
	var cr convertResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		log.Warnw("解析响应JSON失败", "err", err)
		return nil, fmt.Errorf("unmarshal docling response failed: %w", err)
	}

	if cr.Status == "failure" {
		log.Warnw("Docling处理失败", "errors", cr.Errors)
		return nil, fmt.Errorf("docling status=failure: %+v", cr.Errors)
	}

	result := &ParseResult{
		Filename:       cr.Document.Filename,
		Markdown:       cr.Document.MarkdownContent,
		HTML:           cr.Document.HTMLContent,
		Text:           cr.Document.TextContent,
		JSON:           cr.Document.JSONContent,
		Doctags:        cr.Document.DoctagsContent,
		Status:         cr.Status,
		ProcessingTime: cr.ProcessingTime,
		Errors:         cr.Errors,
	}

	log.Infow("Docling解析完成",
		"status", result.Status,
		"timeSec", result.ProcessingTime,
		"markdownLen", len(result.Markdown),
		"htmlLen", len(result.HTML),
	)

	return result, nil
}

// ParseToMarkdown 解析并只返回 Markdown
func (s *svcImpl) ParseToMarkdown(ctx context.Context, filePath string) (string, error) {
	opts := &ParseOptions{
		ToFormats: []string{"md"},
	}
	result, err := s.Parse(ctx, filePath, opts)
	if err != nil {
		return "", err
	}
	return result.Markdown, nil
}

// ParseToHTML 解析并只返回 HTML
func (s *svcImpl) ParseToHTML(ctx context.Context, filePath string) (string, error) {
	opts := &ParseOptions{
		ToFormats: []string{"html"},
	}
	result, err := s.Parse(ctx, filePath, opts)
	if err != nil {
		return "", err
	}
	return result.HTML, nil
}

// ParseToText 解析并只返回纯文本
func (s *svcImpl) ParseToText(ctx context.Context, filePath string) (string, error) {
	opts := &ParseOptions{
		ToFormats: []string{"text"},
	}
	result, err := s.Parse(ctx, filePath, opts)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// HealthCheck 检查 Docling 服务是否可达
func (s *svcImpl) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(_doclingBaseURL, "/") + healthAPI
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("docling health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docling health check status=%d", resp.StatusCode)
	}
	return nil
}

// =============================================================================
// 内部
// =============================================================================

// buildMultipart 构建 multipart/form-data 请求体。
//
// 默认选项（可被 opts 覆盖）：
//   - to_formats: ["json", "md", "html", "text"]
//   - table_mode: "accurate"
func (s *svcImpl) buildMultipart(filePath string, opts *ParseOptions) (*bytes.Buffer, string, error) {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	// --- 上传文件 ---
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("files", filepath.Base(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("copy file content: %w", err)
	}

	// --- 合并选项默认值 ---
	if opts == nil {
		opts = &ParseOptions{}
	}

	// to_formats 默认值
	toFormats := opts.ToFormats
	if len(toFormats) == 0 {
		toFormats = []string{"json", "md", "html", "text"}
	}
	for _, f := range toFormats {
		_ = writer.WriteField("to_formats", f)
	}

	// table_mode 默认值
	tableMode := opts.TableMode
	if tableMode == "" {
		tableMode = "accurate"
	}
	_ = writer.WriteField("table_mode", tableMode)

	// --- 可选参数 ---
	if opts.DoOCR != nil {
		_ = writer.WriteField("do_ocr", fmt.Sprintf("%v", *opts.DoOCR))
	}
	if opts.ForceOCR != nil {
		_ = writer.WriteField("force_ocr", fmt.Sprintf("%v", *opts.ForceOCR))
	}
	if len(opts.OCRLang) > 0 {
		for _, lang := range opts.OCRLang {
			_ = writer.WriteField("ocr_lang", lang)
		}
	}
	if opts.OCREngine != "" {
		_ = writer.WriteField("ocr_engine", opts.OCREngine)
	}
	if opts.PageRange != nil {
		_ = writer.WriteField("page_range", fmt.Sprintf("[%d,%d]", opts.PageRange[0], opts.PageRange[1]))
	}
	if opts.IncludeImages != nil {
		_ = writer.WriteField("include_images", fmt.Sprintf("%v", *opts.IncludeImages))
	}
	if opts.AbortOnError != nil {
		_ = writer.WriteField("abort_on_error", fmt.Sprintf("%v", *opts.AbortOnError))
	}
	if opts.DocumentTimeout > 0 {
		_ = writer.WriteField("document_timeout", fmt.Sprintf("%.1f", opts.DocumentTimeout))
	}
	if opts.Pipeline != "" {
		_ = writer.WriteField("pipeline", opts.Pipeline)
	}
	if len(opts.FromFormats) > 0 {
		for _, f := range opts.FromFormats {
			_ = writer.WriteField("from_formats", f)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	return payload, writer.FormDataContentType(), nil
}
