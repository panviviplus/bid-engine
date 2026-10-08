// Package lang 提供语言的定义
package lang

import "net/http"

// CtxKey ctx设置key的类型
type CtxKey string

const (
	// CtxKeyLang ctx设置语言选项的key
	CtxKeyLang CtxKey = "language"
)

// Language 多语言定义
type Language string

const (
	// ZH 中文简体
	ZH Language = "zh"
	// EN 英语
	EN Language = "en"
)

// GetLanguageFromRequest 通过http request获取用户设置的语言
func GetLanguageFromRequest(request *http.Request) Language {
	userLanguage := request.Header.Get("X-SmartBid-Language")
	if userLanguage == string(EN) {
		return EN
	}
	return ZH
}
