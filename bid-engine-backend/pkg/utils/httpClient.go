package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bid-engine/lib/common/logtool"
)

// HTTPClient HTTP客户端配置
type HTTPClient struct {
	client     *http.Client
	timeout    time.Duration
	logger     *SyncLogger
	ownsLogger bool
}

// HTTPRequest HTTP请求参数
type HTTPRequest struct {
	URL     string            // 请求URL
	Method  string            // 请求方法 GET/POST
	Headers map[string]string // 请求头
	Body    interface{}       // 请求体（POST请求）
	Timeout time.Duration     // 超时时间
}

// HTTPResponse HTTP响应结果
type HTTPResponse struct {
	StatusCode   int                 // 状态码
	Headers      map[string][]string // 响应头
	Body         []byte              // 原始响应体
	ResponseTime time.Duration       // 响应时间
}

// NewHTTPClient 创建HTTP客户端
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// NewHTTPClientWithLogger 创建带日志的HTTP客户端
func NewHTTPClientWithLogger(timeout time.Duration, loggerType string) (*HTTPClient, error) {
	client := NewHTTPClient(timeout)

	if loggerType != "" {
		logger, err := NewSyncLogger(loggerType)
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
		client.logger = logger
		client.ownsLogger = true
	}

	return client, nil
}

// Close 关闭客户端（主要是关闭日志）
func (c *HTTPClient) Close() error {
	if c.logger != nil && c.ownsLogger {
		return c.logger.Close()
	}
	return nil
}

// SetLogger 注入外部的同步日志记录器，使HTTP客户端与任务使用同一日志文件
func (c *HTTPClient) SetLogger(logger *SyncLogger) {
	c.logger = logger
	c.ownsLogger = false
}

// GET 发送GET请求（不带重试）
func (c *HTTPClient) GET(url string, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	req := &HTTPRequest{
		URL:     url,
		Method:  "GET",
		Headers: headers,
		Timeout: c.timeout,
	}
	return c.doRequest(req, result)
}

// POST 发送POST请求（不带重试）
func (c *HTTPClient) POST(url string, body interface{}, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	req := &HTTPRequest{
		URL:     url,
		Method:  "POST",
		Headers: headers,
		Body:    body,
		Timeout: c.timeout,
	}
	return c.doRequest(req, result)
}

// GETWithRetry 发送GET请求（带重试，最多重试3次）
func (c *HTTPClient) GETWithRetry(url string, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	req := &HTTPRequest{
		URL:     url,
		Method:  "GET",
		Headers: headers,
		Timeout: c.timeout,
	}
	return c.doRequestWithRetry(req, result, 3)
}

// POSTWithRetry 发送POST请求（带重试，最多重试3次）
func (c *HTTPClient) POSTWithRetry(url string, body interface{}, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	req := &HTTPRequest{
		URL:     url,
		Method:  "POST",
		Headers: headers,
		Body:    body,
		Timeout: c.timeout,
	}
	return c.doRequestWithRetry(req, result, 3)
}

// doRequest 执行HTTP请求
func (c *HTTPClient) doRequest(req *HTTPRequest, result interface{}) (*HTTPResponse, error) {
	startTime := time.Now()

	// 记录请求日志
	c.logRequest(req)

	// 构建HTTP请求
	httpReq, err := c.buildHTTPRequest(req)
	if err != nil {
		c.logError(req.URL, err, "构建HTTP请求失败")
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// 发送请求
	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logError(req.URL, err, "发送HTTP请求失败")
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logError(req.URL, err, "读取响应体失败")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	responseTime := time.Since(startTime)

	// 构建响应对象
	httpResp := &HTTPResponse{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header,
		Body:         bodyBytes,
		ResponseTime: responseTime,
	}

	// 记录响应日志
	c.logResponse(req.URL, httpResp)

	// 状态码与类型校验
	is2xx := resp.StatusCode >= 200 && resp.StatusCode < 300
	ct := resp.Header.Get("Content-Type")
	isJSON := strings.Contains(strings.ToLower(ct), "application/json")

	if !is2xx {
		c.logError(req.URL, fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode), "非2xx响应")
		return httpResp, fmt.Errorf("http status %d", resp.StatusCode)
	}

	// 仅在2xx+JSON时进行反序列化
	if result != nil {
		if isJSON && len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, result); err != nil {
				c.logParseError(string(bodyBytes), err, "解析响应JSON失败")
				return httpResp, fmt.Errorf("failed to unmarshal response: %w", err)
			}
		} else if len(bodyBytes) > 0 && !isJSON {
			c.logError(req.URL, fmt.Errorf("unexpected content-type: %s", ct), "非JSON响应")
			return httpResp, fmt.Errorf("unexpected content-type: %s", ct)
		}
	}

	return httpResp, nil
}

// doRequestWithRetry 执行带重试的HTTP请求
func (c *HTTPClient) doRequestWithRetry(req *HTTPRequest, result interface{}, maxRetries int) (*HTTPResponse, error) {
	var lastErr error
	var resp *HTTPResponse

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待
			waitTime := time.Duration(attempt) * time.Second
			c.logInfo(fmt.Sprintf("第%d次重试，等待%v", attempt, waitTime), "url", req.URL)
			time.Sleep(waitTime)
		}

		resp, lastErr = c.doRequest(req, result)
		if lastErr == nil {
			if attempt > 0 {
				c.logInfo(fmt.Sprintf("第%d次重试成功", attempt), "url", req.URL)
			}
			return resp, nil
		}

		c.logError(req.URL, lastErr, fmt.Sprintf("第%d次请求失败", attempt+1))
	}

	return resp, fmt.Errorf("请求失败，已重试%d次: %w", maxRetries, lastErr)
}

// buildHTTPRequest 构建HTTP请求
func (c *HTTPClient) buildHTTPRequest(req *HTTPRequest) (*http.Request, error) {
	var body io.Reader

	// 处理请求体
	if req.Body != nil && req.Method == "POST" {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequest(req.Method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认Content-Type（POST请求）
	if req.Method == "POST" && req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义请求头
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// 注意：不在这里设置context超时，使用http.Client的Timeout设置

	return httpReq, nil
}

// 日志记录方法
func (c *HTTPClient) logRequest(req *HTTPRequest) {
	if c.logger != nil {
		c.logger.LogAPIRequest(req.URL, req.Method, req.Body)
	} else {
		logger := logtool.GetLogger().Sugar()
		logger.Infow("HTTP Request",
			"url", req.URL,
			"method", req.Method,
			"body", req.Body,
		)
	}
}

func (c *HTTPClient) logResponse(url string, resp *HTTPResponse) {
	if c.logger != nil {
		c.logger.LogAPIResponse(url, resp.StatusCode, string(resp.Body), resp.ResponseTime)
	} else {
		logger := logtool.GetLogger().Sugar()
		logger.Infow("HTTP Response",
			"url", url,
			"status_code", resp.StatusCode,
			"response_body", string(resp.Body),
			"response_time_ms", resp.ResponseTime.Milliseconds(),
		)
	}
}

func (c *HTTPClient) logError(url string, err error, context string) {
	if c.logger != nil {
		c.logger.LogAPIError(url, err, context)
	} else {
		logger := logtool.GetLogger().Sugar()
		logger.Errorw("HTTP Error",
			"url", url,
			"error", err.Error(),
			"context", context,
		)
	}
}

func (c *HTTPClient) logParseError(responseBody string, err error, context string) {
	if c.logger != nil {
		c.logger.LogParseError(responseBody, err, context)
	} else {
		logger := logtool.GetLogger().Sugar()
		logger.Errorw("HTTP Parse Error",
			"response_body", responseBody,
			"error", err.Error(),
			"context", context,
		)
	}
}

func (c *HTTPClient) logInfo(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Info(msg, keysAndValues...)
	} else {
		logger := logtool.GetLogger().Sugar()
		logger.Infow(msg, keysAndValues...)
	}
}

// 便捷函数：快速创建HTTP客户端并发送请求

// QuickGET 快速发送GET请求
func QuickGET(url string, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	client := NewHTTPClient(30 * time.Second)
	defer client.Close()
	return client.GET(url, headers, result)
}

// QuickPOST 快速发送POST请求
func QuickPOST(url string, body interface{}, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	client := NewHTTPClient(30 * time.Second)
	defer client.Close()
	return client.POST(url, body, headers, result)
}

// QuickGETWithRetry 快速发送带重试的GET请求
func QuickGETWithRetry(url string, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	client := NewHTTPClient(30 * time.Second)
	defer client.Close()
	return client.GETWithRetry(url, headers, result)
}

// QuickPOSTWithRetry 快速发送带重试的POST请求
func QuickPOSTWithRetry(url string, body interface{}, headers map[string]string, result interface{}) (*HTTPResponse, error) {
	client := NewHTTPClient(30 * time.Second)
	defer client.Close()
	return client.POSTWithRetry(url, body, headers, result)
}
