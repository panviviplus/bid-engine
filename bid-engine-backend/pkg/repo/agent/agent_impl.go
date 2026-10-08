package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// svcImpl agent服务实现
type svcImpl struct {
	logger       *zap.SugaredLogger
	agentConfigs map[string]*AgentConfig
	httpClient   *http.Client
}

// CallAgentStream 调用指定的agent服务（流式返回）
func (s *svcImpl) CallAgentStream(ctx context.Context, agentID string, req *AgentRequest) (<-chan *AgentResponse, <-chan error) {
	respChan := make(chan *AgentResponse, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		// 获取agent配置
		config, exists := s.agentConfigs[agentID]
		if !exists {
			errChan <- fmt.Errorf("未找到agent配置: %s", agentID)
			return
		}

		// 序列化请求数据
		reqData, err := json.Marshal(req)
		if err != nil {
			errChan <- fmt.Errorf("序列化请求数据失败: %v", err)
			return
		}

		// 创建HTTP请求
		httpReq, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(reqData))
		if err != nil {
			errChan <- fmt.Errorf("创建HTTP请求失败: %v", err)
			return
		}

		// 设置请求头
		httpReq.Header.Set("Authorization", config.Token)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Cache-Control", "no-cache")

		// 创建HTTP客户端（如果还没有）
		if s.httpClient == nil {
			tr := &http.Transport{
				DisableKeepAlives: true,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			}
			s.httpClient = &http.Client{
				Timeout:   time.Duration(config.Timeout) * time.Second,
				Transport: tr,
			}
		}

		// 发送请求
		s.logger.Infow("调用agent流式服务", "agentID", agentID, "url", config.URL, "query", req.Query)
		resp, err := s.httpClient.Do(httpReq)
		if err != nil {
			errChan <- fmt.Errorf("调用agent服务失败: %v", err)
			return
		}
		defer resp.Body.Close()

		// 检查HTTP状态码
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("agent服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
			return
		}

		// 处理流式响应
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// 解析SSE数据
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var agentResp AgentResponse
				if err := json.Unmarshal([]byte(data), &agentResp); err != nil {
					s.logger.Warnw("解析流式响应失败", "data", data, "err", err)
					continue
				}

				// 只处理message事件类型的响应
				if agentResp.Event == "message" && agentResp.Answer != "" {
					select {
					case respChan <- &agentResp:
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					}
				} else if agentResp.Event == "message_end" {
					// 消息结束处理逻辑
					break
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("读取流式响应失败: %v", err)
			return
		}

		s.logger.Infow("agent流式服务调用完成", "agentID", agentID)
	}()

	return respChan, errChan
}

// CallAgent 调用指定的agent服务，一次性返回
func (s *svcImpl) CallAgent(ctx context.Context, agentID string, req *AgentRequest) (*AgentResponse, error) {
	// 获取agent配置
	config, exists := s.agentConfigs[agentID]
	if !exists {
		return nil, fmt.Errorf("未找到agent配置: %s", agentID)
	}

	s.logger.Infow("CallAgent --> 调用agent服务，匹配到目标agentConfig：", "agentID", agentID, "config", config)

	// 序列化请求数据
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", config.URL, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Authorization", config.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	// 基于 ctx 与 config 计算“本次请求的超时”
	tr := &http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
	}
	var timeout time.Duration
	if dl, ok := ctx.Deadline(); ok {
		// ctx 设置了deadline，则以 ctx 为准
		timeout = time.Until(dl)
	} else if config.Timeout > 0 {
		// 未设置 ctx deadline，则采用配置的超时（秒）
		timeout = time.Duration(config.Timeout) * time.Second
	} else {
		// config<=0 表示不超时
		timeout = 0
	}
	s.httpClient = &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	// 发送请求
	s.logger.Infow("CallAgent --> 调用agent服务，", "agentID", agentID, "url", config.URL, "query", req.Query)
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用agent服务失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent服务返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var agentResp AgentResponse
	if err := json.Unmarshal(body, &agentResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	s.logger.Infow("CallAgent --> agent服务调用成功", "agentID", agentID, "answer", agentResp.Answer)
	return &agentResp, nil
}

// GetAgentConfig 获取agent配置信息
func (s *svcImpl) GetAgentConfig(agentID string) (*AgentConfig, error) {
	config, exists := s.agentConfigs[agentID]
	if !exists {
		return nil, fmt.Errorf("未找到agent配置: %s", agentID)
	}
	return config, nil
}

// IsValidAgent 验证agent ID是否有效
func (s *svcImpl) IsValidAgent(agentID string) bool {
	_, exists := s.agentConfigs[agentID]
	return exists
}
