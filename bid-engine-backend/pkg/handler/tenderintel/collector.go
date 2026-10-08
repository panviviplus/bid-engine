package tenderintel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	skbcfg "bid-engine/pkg/config"
)

// 采集服务（tender-collection）的 HTTP 契约。
//
// 采集服务独立部署、独立运维，只负责“站点 → 规范公告文档”：
// 不连数据库、不持存储凭据。落库、打标、匹配、提醒全部在本后端完成。

const (
	defaultCollectorBaseURL = "http://127.0.0.1:5011"
	defaultCollectTimeout   = 180 * time.Second
)

// collectorAttachment 附件链接（只记录链接，不下载文件）。
type collectorAttachment struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// collectorExtractMeta 抽取元信息。
type collectorExtractMeta struct {
	Strategy   string   `json:"strategy"`
	Confidence float64  `json:"confidence"`
	Warnings   []string `json:"warnings"`
}

// collectorDoc 规范化公告文档（跨服务契约，字段名与采集服务保持一致）。
type collectorDoc struct {
	SourceKey      string                `json:"sourceKey"`
	SourceName     string                `json:"sourceName"`
	URL            string                `json:"url"`
	CanonicalURL   string                `json:"canonicalUrl"`
	ExternalID     string                `json:"externalId"`
	Title          string                `json:"title"`
	PublisherName  string                `json:"publisherName"`
	AgencyName     string                `json:"agencyName"`
	ProjectCode    string                `json:"projectCode"`
	BudgetText     string                `json:"budgetText"`
	PublishDate    string                `json:"publishDate"`
	DeadlineText   string                `json:"deadlineText"`
	RegionText     string                `json:"regionText"`
	NoticeTypeText string                `json:"noticeTypeText"`
	BodyHTML       string                `json:"bodyHtml"`
	BodyMarkdown   string                `json:"bodyMarkdown"`
	BodyText       string                `json:"bodyText"`
	Attachments    []collectorAttachment `json:"attachments"`
	FetchedAt      string                `json:"fetchedAt"`
	Extract        collectorExtractMeta  `json:"extract"`
}

// collectorCollectRequest 一次采集请求。
type collectorCollectRequest struct {
	SourceKey     string `json:"sourceKey"`
	ListURL       string `json:"listUrl"`
	DiscoveryMode string `json:"discoveryMode"`
	NeedsBrowser  bool   `json:"needsBrowser"`
	Cursor        string `json:"cursor"`
	MaxPages      int    `json:"maxPages"`
	MaxItems      int    `json:"maxItems"`
	// omitempty：nil 切片会被序列化成 null，而采集服务侧 keywords 是列表类型，收到 null 会 422
	Keywords []string `json:"keywords,omitempty"`
	Params   string   `json:"params"`
}

// collectorExtractRequest 单条公告重抓请求。
type collectorExtractRequest struct {
	URL            string `json:"url"`
	SourceKey      string `json:"sourceKey"`
	NeedsBrowser   bool   `json:"needsBrowser"`
	IncludeRawHTML bool   `json:"includeRawHtml"`
}

type collectorItemError struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// collectorCollectResponse 一次采集响应。
type collectorCollectResponse struct {
	SourceKey string               `json:"sourceKey"`
	Cursor    string               `json:"cursor"`
	Items     []collectorDoc       `json:"items"`
	Errors    []collectorItemError `json:"errors"`
}

type collectorClient struct {
	baseURL string
	client  *http.Client
}

func newCollectorClient() *collectorClient {
	baseURL := strings.TrimSpace(skbcfg.Get("tender_intel.collector_base_url"))
	if baseURL == "" {
		baseURL = defaultCollectorBaseURL
	}
	return &collectorClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: defaultCollectTimeout},
	}
}

// Collect 调用采集服务完成“发现 + 抽取”。
func (c *collectorClient) Collect(ctx context.Context, req collectorCollectRequest) (*collectorCollectResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/collect", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("采集服务不可用: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读取采集结果失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("采集服务返回 %d: %s", resp.StatusCode, truncateRunes(string(raw), 300))
	}
	var out collectorCollectResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析采集结果失败: %w", err)
	}
	return &out, nil
}

// Extract 调用采集服务重抓单条公告。
func (c *collectorClient) Extract(ctx context.Context, req collectorExtractRequest) (*collectorDoc, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/extract", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("采集服务不可用: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读取单条抽取结果失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("采集服务返回 %d: %s", resp.StatusCode, truncateRunes(string(raw), 300))
	}
	var out collectorDoc
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析单条抽取结果失败: %w", err)
	}
	return &out, nil
}

// Health 探测采集服务是否可用（用于运行前自检）。
func (c *collectorClient) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("采集服务不可用: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("采集服务健康检查返回 %d", resp.StatusCode)
	}
	return nil
}

// collectorDiscoverItem 发现阶段返回的单条候选。
type collectorDiscoverItem struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	PublishDate string `json:"publishDate"`
}

// collectorDiscoverResponse 发现阶段响应。
type collectorDiscoverResponse struct {
	SourceKey string                  `json:"sourceKey"`
	Cursor    string                  `json:"cursor"`
	Items     []collectorDiscoverItem `json:"items"`
	Errors    []collectorItemError    `json:"errors"`
}

// collectorDiscoverRequest 发现请求（与采集服务的 /discover 契约一致）。
type collectorDiscoverRequest struct {
	SourceKey     string `json:"sourceKey"`
	ListURL       string `json:"listUrl"`
	DiscoveryMode string `json:"discoveryMode"`
	NeedsBrowser  bool   `json:"needsBrowser"`
	Cursor        string `json:"cursor"`
	MaxPages      int    `json:"maxPages"`
	// 同上：nil 切片必须省略而不是发 null
	Keywords []string `json:"keywords,omitempty"`
	Params   string   `json:"params"`
}

// Discover 只做发现，用于“采集源联通性探测”：确认列表页能否抓到候选公告。
func (c *collectorClient) Discover(ctx context.Context, req collectorDiscoverRequest) (*collectorDiscoverResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/discover", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("采集服务不可用: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("读取探测结果失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("采集服务返回 %d: %s", resp.StatusCode, truncateRunes(string(raw), 300))
	}
	var out collectorDiscoverResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析探测结果失败: %w", err)
	}
	return &out, nil
}
