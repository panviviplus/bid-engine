// Package request 请求参数和返回结果相关定义
package request

// CommonRet 通用返回结构
type CommonRet struct {
	Code      int32  `json:"code"`
	Message   string `json:"message" default:"success"`
	RequestID string `json:"requestId"`
}

// CommonStatus 通过返回状态
type CommonStatus struct {
	BackendLatencyMs int32 `json:"backend_latency_ms"`
	NumHits          int   `json:"num_hits"`
}
