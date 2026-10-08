package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrMissingAPIKey     = errors.New("llm api key is empty")
	ErrMissingBaseURL    = errors.New("llm base_url is empty")
	ErrMissingEndpoint   = errors.New("llm endpoint_path is empty")
	ErrMissingModel      = errors.New("llm model is empty")
	ErrInvalidRequest    = errors.New("invalid llm request")
	ErrEmptyLLMResponse  = errors.New("empty llm response")
	ErrNonJSONContent    = errors.New("llm response is not valid json")
	ErrJSONNotFound      = errors.New("json object not found in text")
	ErrProviderNotConfig = errors.New("llm provider is not configured")

	// ── 用户 LLM 配置相关（业务层友好文案）───────────────────────
	// ErrLLMNotConfigured 用户未配置 LLM（或配置不完整），仅认用户配置，不做系统兜底
	ErrLLMNotConfigured = errors.New("LLM未配置，请先在“系统管理-模型配置”中配置")
	// ErrLLMKeyUnauthorized LLM API Key 无权限（HTTP 401/403）
	ErrLLMKeyUnauthorized = errors.New("LLM Key无权限，请检查API Key是否正确")
	// ErrLLMTimeout LLM 调用超时
	ErrLLMTimeout = errors.New("LLM调用超时，请稍后重试")
	// ErrLLMRateLimit LLM 请求过于频繁或已达限流（HTTP 429）
	ErrLLMRateLimit = errors.New("LLM请求过于频繁或已达限流，请稍后重试")
	// ErrLLMUnavailable LLM 不可用/调用失败（网络、服务端错误等）
	ErrLLMUnavailable = errors.New("LLM不可用，请检查服务状态或稍后重试")
	// ErrLLMModelNotFound 模型不存在或不可用（HTTP 400(含 model)/404）
	ErrLLMModelNotFound = errors.New("模型不存在或不可用，请检查模型 ID 与响应类型")
	// ErrLLMRequestRejected 请求被提供方拒绝（HTTP 4xx，参数/能力不匹配，重试无意义）
	ErrLLMRequestRejected = errors.New("模型拒绝了本次请求（参数或能力不匹配），请检查模型配置")
	// ErrLLMURLUnreachable LLM 服务不可达（网络/DNS/连接失败）
	ErrLLMURLUnreachable = errors.New("LLM服务不可达，请检查 Base URL 或网络")
)

func ErrProviderNotSupported(p Provider) error {
	return fmt.Errorf("provider not supported: %s", p)
}

// ── 用户可见错误码（供前端差异化提示）────────────────────────────

// LLMErrCode LLM 配置/调用错误码（用户可见，不含原始错误细节）
type LLMErrCode = int32

const (
	LLMErrNotConfigured   LLMErrCode = 1001 // 未配置/配置不完整
	LLMErrKeyUnauthorized LLMErrCode = 1002 // API Key 无权限
	LLMErrModelNotFound   LLMErrCode = 1003 // 模型 ID 错误或不可用
	LLMErrTimeout         LLMErrCode = 1004 // 调用超时
	LLMErrRateLimit       LLMErrCode = 1005 // 限流
	LLMErrURLUnreachable  LLMErrCode = 1006 // 服务不可达
	LLMErrUnavailable     LLMErrCode = 1007 // 通用不可用
	LLMErrRequestRejected LLMErrCode = 1008 // 请求被拒绝（参数/能力不匹配）
)

type llmErrMeta struct {
	Code LLMErrCode
	Msg  string
}

var llmErrMetaMap = map[error]llmErrMeta{
	ErrLLMNotConfigured:   {LLMErrNotConfigured, "LLM未配置，请先在“系统管理-模型配置”中配置"},
	ErrLLMKeyUnauthorized: {LLMErrKeyUnauthorized, "LLM Key无权限，请检查API Key是否正确"},
	ErrLLMModelNotFound:   {LLMErrModelNotFound, "模型不存在或不可用，请检查模型 ID 与响应类型"},
	ErrLLMTimeout:         {LLMErrTimeout, "LLM调用超时，请稍后重试"},
	ErrLLMRateLimit:       {LLMErrRateLimit, "LLM请求过于频繁或已达限流，请稍后重试"},
	ErrLLMURLUnreachable:  {LLMErrURLUnreachable, "LLM服务不可达，请检查 Base URL 或网络"},
	ErrLLMUnavailable:     {LLMErrUnavailable, "LLM不可用，请检查服务状态或稍后重试"},
	ErrLLMRequestRejected: {LLMErrRequestRejected, "模型拒绝了本次请求（参数或能力不匹配），请检查模型配置"},
}

// LLMErrorMeta 提取用户可见的 LLM 错误码与友好文案（剥离原始错误细节，避免透传到前端）。
// 未直接命中哨兵时按错误特征归类（兼容裸 httpStatusError / 网络错误）；未知错误统一归为通用不可用。
// 原始错误详情仅供服务端日志使用。
func LLMErrorMeta(err error) (LLMErrCode, string) {
	if err != nil {
		for sentinel, meta := range llmErrMetaMap {
			if errors.Is(err, sentinel) {
				return meta.Code, meta.Msg
			}
		}
		sentinel := classifySentinel(err)
		if meta, ok := llmErrMetaMap[sentinel]; ok {
			return meta.Code, meta.Msg
		}
	}
	meta := llmErrMetaMap[ErrLLMUnavailable]
	return meta.Code, meta.Msg
}

// IsLLMError 判断错误是否属于已分类的 LLM 配置/调用错误
func IsLLMError(err error) bool {
	if err == nil {
		return false
	}
	for sentinel := range llmErrMetaMap {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// IsProviderRejection 判断 LLM 调用错误是否为“请求本身被提供方拒绝”的确定性错误
// （HTTP 4xx，429 限流除外）。这类错误重试或换网络没有意义，应视为格式/配置问题：
// 由调用方走格式降级或提示用户修正，而不是按基础设施故障处理。
// 典型场景：部分网关不支持 strict json_schema，会返回 400/403 等拒绝请求。
func IsProviderRejection(err error) bool {
	var he *httpStatusError
	if errors.As(err, &he) {
		switch he.Status {
		case http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
			http.StatusUnsupportedMediaType:
			return true
		}
	}
	return false
}

type ResponseFormatRejection uint8

const (
	ResponseFormatRejectionNone ResponseFormatRejection = iota
	ResponseFormatStrictUnsupported
	ResponseFormatSchemaUnsupported
)

// ClassifyResponseFormatRejection 只识别明确由 response_format 造成的确定性请求拒绝。
// 鉴权、模型、限流和服务端错误绝不能触发格式降级。
func ClassifyResponseFormatRejection(err error) ResponseFormatRejection {
	var he *httpStatusError
	if !errors.As(err, &he) {
		return ResponseFormatRejectionNone
	}
	switch he.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusUnsupportedMediaType:
	default:
		return ResponseFormatRejectionNone
	}
	body := strings.ToLower(he.Body)
	if strings.Contains(body, "strict") {
		return ResponseFormatStrictUnsupported
	}
	if strings.Contains(body, "json_schema") || strings.Contains(body, "response_format") {
		return ResponseFormatSchemaUnsupported
	}
	return ResponseFormatRejectionNone
}

// FriendlyMessage 返回用户可见错误文案：已分类的 LLM 错误返回友好文案（不含原始细节），
// 其它错误原样返回 err.Error()。用于不确定错误来源、需兼容非 LLM 错误的场景（如流水线 last_error）。
func FriendlyMessage(err error) string {
	if err == nil {
		return ""
	}
	if IsLLMError(err) {
		_, msg := LLMErrorMeta(err)
		return msg
	}
	return err.Error()
}

// ── 错误分类 ────────────────────────────────────────────────────

// classifySentinel 将底层 LLM 调用错误归类为对应的业务错误哨兵（不含原始细节）：
//   - context deadline → 超时；
//   - HTTP 401/403 → Key 无权限；404 / 400(含 model) → 模型不存在；
//   - HTTP 429 → 限流；
//   - 网络类（DNS/连接/URL）→ 服务不可达；
//   - 其它 → 不可用。
func classifySentinel(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrLLMTimeout
	}

	var he *httpStatusError
	if errors.As(err, &he) {
		if he.Status == http.StatusForbidden && strings.Contains(strings.ToLower(he.Body), "ip access denied") {
			return ErrLLMUnavailable
		}
		switch he.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrLLMKeyUnauthorized
		case http.StatusNotFound:
			return ErrLLMModelNotFound
		case http.StatusBadRequest:
			if strings.Contains(strings.ToLower(he.Body), "model") {
				return ErrLLMModelNotFound
			}
			return ErrLLMUnavailable
		case http.StatusTooManyRequests:
			return ErrLLMRateLimit
		default:
			return ErrLLMUnavailable
		}
	}

	var dnsErr *net.DNSError
	var opErr *net.OpError
	var urlErr *url.Error
	var netErr net.Error
	if errors.As(err, &dnsErr) || errors.As(err, &opErr) || errors.As(err, &urlErr) || errors.As(err, &netErr) {
		return ErrLLMURLUnreachable
	}

	return ErrLLMUnavailable
}

// classifyLLMError 将底层 LLM 调用错误归类为业务友好错误（保留原始错误作为 detail，便于日志排查）。
// 前端展示请使用 LLMErrorMeta 提取友好文案与错误码，勿直接透传 err.Error()。
func classifyLLMError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", classifySentinel(err), err)
}

// httpStatusError 携带 HTTP 状态码的 LLM 调用错误，供错误分类使用
type httpStatusError struct {
	Status int
	Body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("llm http status %d: %s", e.Status, e.Body)
}
