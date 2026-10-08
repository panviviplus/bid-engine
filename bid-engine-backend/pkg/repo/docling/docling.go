// Package docling 提供 Docling 文档解析服务的 HTTP 客户端封装。
//
// Docling 是一个开源的文档解析引擎，支持 PDF/DOCX/PPTX/XLSX/HTML/图片 等格式，
// 输出 Markdown / HTML / 纯文本 / 结构化 JSON (DoclingDocument)。
//
// API 文档: http://localhost:5001/docs
// 源码仓库: https://github.com/docling-project/docling-serve
package docling

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	skbcfg "bid-engine/pkg/config"
)

// =============================================================================
// Service 接口
// =============================================================================

// Service Docling 文档解析服务接口
type Service interface {
	// Parse 解析文档文件，返回完整结构化结果（json + md + html + text）
	Parse(ctx context.Context, filePath string, opts *ParseOptions) (*ParseResult, error)

	// ParseToMarkdown 解析并只返回 Markdown
	ParseToMarkdown(ctx context.Context, filePath string) (string, error)

	// ParseToHTML 解析并只返回 HTML
	ParseToHTML(ctx context.Context, filePath string) (string, error)

	// ParseToText 解析并只返回纯文本
	ParseToText(ctx context.Context, filePath string) (string, error)

	// HealthCheck 检查 Docling 服务是否可达
	HealthCheck(ctx context.Context) error
}

// =============================================================================
// 单例
// =============================================================================

var (
	instance *svcImpl
	once     sync.Once
)

// GetInstance 获取 Docling 服务单例
func GetInstance() Service {
	once.Do(func() {
		_doclingBaseURL = skbcfg.Get("properties.docling_base_url")
		if _doclingBaseURL == "" {
			_doclingBaseURL = "http://localhost:5001"
		}
		timeout := time.Duration(doclingTimeoutSec("properties.docling_timeout_sec", 660)) * time.Second
		logtool.GetLogger().Sugar().Infow("docling服务初始化",
			"baseURL", _doclingBaseURL,
			"timeout", timeout,
		)
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			client: http.Client{
				Timeout: timeout,
			},
		}
	})
	return instance
}

// doclingTimeoutSec 读取 docling 同步解析的客户端超时（秒）。
// 默认 660s：需覆盖服务端 DOCLING_SERVE_MAX_SYNC_WAIT（600s）+ 响应余量，
// 否则客户端会在服务端处理完成前提前掐断连接。
func doclingTimeoutSec(key string, fallback int) int {
	if raw := strings.TrimSpace(skbcfg.Get(key)); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 60 && n <= 3600 {
			return n
		}
	}
	return fallback
}

// svcImpl Docling 服务实现
type svcImpl struct {
	logger *zap.SugaredLogger
	client http.Client
}

// 包级配置（GetInstance 时从 conf-local.yml 读取）
var _doclingBaseURL string

// =============================================================================
// 请求选项
// =============================================================================

// ParseOptions Docling 解析请求选项
type ParseOptions struct {
	// FromFormats 输入格式提示，可选，Docling 会自动检测
	// 合法值: "pdf", "docx", "pptx", "xlsx", "html", "md", "image"
	FromFormats []string `json:"from_formats,omitempty"`

	// ToFormats 期望的输出格式，默认 ["json", "md", "html", "text"]
	// 合法值: "md", "json", "html", "text", "doctags", "yaml", "html_split_page", "vtt"
	ToFormats []string `json:"to_formats,omitempty"`

	// DoOCR 是否启用 OCR（对扫描件/图片必须开启）
	DoOCR *bool `json:"do_ocr,omitempty"`

	// OCRLang OCR 语言列表，如 ["en", "zh"]
	OCRLang []string `json:"ocr_lang,omitempty"`

	// OCRengine OCR 引擎: "easyocr"（默认）, "tesseract", "rapidocr", "ocrmac", "auto"
	OCREngine string `json:"ocr_engine,omitempty"`

	// TableMode 表格提取模式: "fast"（默认）, "accurate"
	TableMode string `json:"table_mode,omitempty"`

	// PageRange 只解析指定页面范围 [start, end]
	PageRange *[2]int `json:"page_range,omitempty"`

	// IncludeImages 是否在结果中包含页面图片（base64）
	IncludeImages *bool `json:"include_images,omitempty"`

	// ForceOCR 强制对全文做 OCR
	ForceOCR *bool `json:"force_ocr,omitempty"`

	// AbortOnError 遇到第一个错误即停止
	AbortOnError *bool `json:"abort_on_error,omitempty"`

	// DocumentTimeout 单个文档超时秒数（API 侧），上限由服务端 max_sync_wait 决定（默认 120s）
	DocumentTimeout float64 `json:"document_timeout,omitempty"`

	// Pipeline 布局管线: "default", 或自定义
	Pipeline string `json:"pipeline,omitempty"`
}

// =============================================================================
// 响应结构（精确映射 Docling API 返回体）
// =============================================================================

// convertResponse Docling /v1/convert/file 的顶层响应
type convertResponse struct {
	Document       convertDocument `json:"document"`
	Status         string          `json:"status"`
	ProcessingTime float64         `json:"processing_time"`
	Timings        json.RawMessage `json:"timings"`
	Errors         []ErrorItem     `json:"errors"`
}

// convertDocument 响应内层 document 对象
type convertDocument struct {
	Filename        string          `json:"filename"`
	MarkdownContent string          `json:"md_content"`
	HTMLContent     string          `json:"html_content"`
	TextContent     string          `json:"text_content"`
	JSONContent     json.RawMessage `json:"json_content"`
	DoctagsContent  string          `json:"doctags_content"`
}

// ParseResult Parse 方法返回的解析结果
type ParseResult struct {
	// Filename 文件名
	Filename string `json:"filename"`

	// Markdown 文档 Markdown 表示
	Markdown string `json:"markdown"`

	// HTML 文档 HTML 表示（含 CSS 样式）
	HTML string `json:"html"`

	// Text 文档纯文本
	Text string `json:"text"`

	// JSON DoclingDocument 结构化 JSON（后续可反序列化为 DoclingDocument）
	JSON json.RawMessage `json:"json"`

	// Doctags DocTags 格式输出
	Doctags string `json:"doctags"`

	// Status 处理状态: "success", "partial_success", "skipped", "failure"
	Status string `json:"status"`

	// ProcessingTime 处理耗时（秒）
	ProcessingTime float64 `json:"processing_time"`

	// Errors 错误列表（partial_success 时非空）
	Errors []ErrorItem `json:"errors"`
}

// ErrorItem 解析错误项
type ErrorItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// =============================================================================
// DoclingDocument — json_content 的结构化表示（核心数据模型）
// =============================================================================

// DoclingDocument 是 Docling 结构化 JSON 输出的顶层结构。
// 由 ParseResult.JSON 反序列化得到，包含文档的完整语义解析结果。
type DoclingDocument struct {
	Name          string              `json:"name"`
	SchemaName    string              `json:"schema_name"`
	Version       string              `json:"version"`
	Origin        json.RawMessage     `json:"origin,omitempty"`
	Pages         map[string]PageInfo `json:"pages"` // key: 页码字符串 "1", "2", ...
	Texts         []TextItem          `json:"texts"`
	Tables        []TableItem         `json:"tables"`
	Pictures      []PictureItem       `json:"pictures"`
	Groups        []GroupItem         `json:"groups"`
	Body          GroupItem           `json:"body"`
	KeyValueItems []KeyValueItem      `json:"key_value_items,omitempty"`
	FormItems     []FormItem          `json:"form_items,omitempty"`
	Furniture     *GroupItem          `json:"furniture,omitempty"` // 页眉/页脚等装饰元素（GroupItem 树）
}

// PageInfo 页面信息
type PageInfo struct {
	Size  PageSize   `json:"size"`
	Image *PageImage `json:"image,omitempty"`
}

// PageSize 页面尺寸
type PageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PageImage 页面图像（base64 PNG）
type PageImage struct {
	MimeType string   `json:"mimetype"`
	DPI      int      `json:"dpi"`
	Size     PageSize `json:"size"`
	URI      string   `json:"uri"` // data:image/png;base64,...
}

// TextItem 文本项（段落/标题/页眉等）
type TextItem struct {
	SelfRef      string       `json:"self_ref"`
	Parent       *Ref         `json:"parent,omitempty"`
	Children     []Ref        `json:"children,omitempty"`
	Text         string       `json:"text"`
	Orig         string       `json:"orig,omitempty"`
	Label        string       `json:"label"`         // text, title, section_header, page_header, page_footer, ...
	ContentLayer string       `json:"content_layer"` // "body" | "furniture"
	Prov         []Provenance `json:"prov"`
	Formatting   *Formatting  `json:"formatting,omitempty"`
	Hyperlink    *Hyperlink   `json:"hyperlink,omitempty"`
	Meta         any          `json:"meta,omitempty"`
	Enumerated   bool         `json:"enumerated,omitempty"`
	Marker       string       `json:"marker,omitempty"`
}

// Ref 引用（$ref 格式）
type Ref struct {
	Ref string `json:"$ref"`
}

// Provenance 溯源信息 — 标注文本块在原始文档中的位置
type Provenance struct {
	PageNo   int    `json:"page_no"`
	BBox     BBox   `json:"bbox"`
	CharSpan [2]int `json:"charspan"` // [start, end] 相对于 Text 字段的字符区间
}

// BBox 边界框
type BBox struct {
	L           float64 `json:"l"`            // left
	T           float64 `json:"t"`            // top
	R           float64 `json:"r"`            // right
	B           float64 `json:"b"`            // bottom
	CoordOrigin string  `json:"coord_origin"` // "BOTTOMLEFT" | "TOPLEFT"
}

// Formatting 文本格式
type Formatting struct {
	Bold   *bool `json:"bold,omitempty"`
	Italic *bool `json:"italic,omitempty"`
}

// Hyperlink 超链接。Docling 官方数据模型（docling-serve v1.16.1，
// docling_core 的 TextItem.hyperlink 为 Optional[Union[AnyUrl, Path]]）中
// hyperlink 是普通字符串（如 "mailto:..."）；个别版本/场景也可能序列化为
// {"url": "..."} 对象。自定义 UnmarshalJSON 同时兼容这两种形态。
type Hyperlink struct {
	URL string `json:"url,omitempty"`
}

// UnmarshalJSON 兼容字符串与对象两种 Docling 输出形态。
func (h *Hyperlink) UnmarshalJSON(data []byte) error {
	if h == nil {
		return errors.New("docling: Hyperlink.UnmarshalJSON on nil pointer")
	}
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		h.URL = plain
		return nil
	}
	type hyperlinkAlias Hyperlink
	var obj hyperlinkAlias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*h = Hyperlink(obj)
	return nil
}

// TableItem 表格项
type TableItem struct {
	SelfRef      string       `json:"self_ref"`
	Parent       *Ref         `json:"parent,omitempty"`
	Children     []Ref        `json:"children,omitempty"`
	Label        string       `json:"label"`
	ContentLayer string       `json:"content_layer,omitempty"`
	Prov         []Provenance `json:"prov"`
	Data         TableData    `json:"data"`
	Captions     []TextItem   `json:"captions,omitempty"`
	Annotations  []any        `json:"annotations,omitempty"`
	References   []Ref        `json:"references,omitempty"`
	Footnotes    []any        `json:"footnotes,omitempty"`
	Image        *PageImage   `json:"image,omitempty"`
	Meta         any          `json:"meta,omitempty"`
}

// TableData 表格数据
type TableData struct {
	NumRows    int           `json:"num_rows"`
	NumCols    int           `json:"num_cols"`
	Grid       [][]TableCell `json:"grid"`
	TableCells []TableCell   `json:"table_cells"`
}

// TableCell 表格单元格
type TableCell struct {
	Text              string `json:"text"`
	BBox              BBox   `json:"bbox"`
	RowSpan           int    `json:"row_span"`
	ColSpan           int    `json:"col_span"`
	StartRowOffsetIdx int    `json:"start_row_offset_idx"`
	EndRowOffsetIdx   int    `json:"end_row_offset_idx"`
	StartColOffsetIdx int    `json:"start_col_offset_idx"`
	EndColOffsetIdx   int    `json:"end_col_offset_idx"`
	ColumnHeader      bool   `json:"column_header"`
	RowHeader         bool   `json:"row_header"`
	RowSection        bool   `json:"row_section"`
	Fillable          bool   `json:"fillable"`
}

// PictureItem 图片项
type PictureItem struct {
	SelfRef      string       `json:"self_ref"`
	Parent       *Ref         `json:"parent,omitempty"`
	Children     []Ref        `json:"children,omitempty"`
	Label        string       `json:"label"`
	ContentLayer string       `json:"content_layer,omitempty"`
	Prov         []Provenance `json:"prov"`
	Caption      []TextItem   `json:"caption,omitempty"`
	Image        PageImage    `json:"image"`
	Annotations  []any        `json:"annotations,omitempty"`
	References   []Ref        `json:"references,omitempty"`
	Footnotes    []any        `json:"footnotes,omitempty"`
	Meta         any          `json:"meta,omitempty"`
}

// GroupItem 分组（body 也是 GroupItem 类型）
type GroupItem struct {
	SelfRef      string       `json:"self_ref,omitempty"`
	Name         string       `json:"name"`
	Children     []GroupChild `json:"children,omitempty"`
	Prov         []Provenance `json:"prov,omitempty"`
	ContentLayer string       `json:"content_layer,omitempty"`
	Meta         any          `json:"meta,omitempty"`
}

// GroupChild 分组子项（$ref 引用）
type GroupChild struct {
	Ref string `json:"$ref"`
}

// KeyValueItem 键值对
type KeyValueItem struct {
	SelfRef string       `json:"self_ref"`
	Key     TextItem     `json:"key"`
	Value   TextItem     `json:"value"`
	Prov    []Provenance `json:"prov"`
	Label   string       `json:"label"`
}

// FormItem 表单项
type FormItem struct {
	SelfRef string       `json:"self_ref"`
	Prov    []Provenance `json:"prov"`
	Label   string       `json:"label"`
}

// =============================================================================
// ParseResult 便捷方法
// =============================================================================

// ParseDocument 将 ParseResult.JSON 反序列化为 DoclingDocument
func (r *ParseResult) ParseDocument() (*DoclingDocument, error) {
	var doc DoclingDocument
	if err := json.Unmarshal(r.JSON, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
