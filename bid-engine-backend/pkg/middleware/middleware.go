// Package middleware 定义了常用的中间件，包括cors控制、jwt鉴权、请求日志等等
package middleware

import (
	"sync"

	"bid-engine/pkg/middleware/schedule"

	skbcfg "bid-engine/pkg/config"
)

var (
	cookieDomain     string
	jwtIdentifyKey   string
	onceCookieDomain sync.Once
	openAPIkey       string
)

// Init 初始化
func Init() {
	cookieDomain = ""
	jwtIdentifyKey = jwtDefaultKey
	onceCookieDomain.Do(func() {
		cookieDomain = skbcfg.Get("properties.cookie_domain")
		jwtIdentifyKey = skbcfg.Get("properties.cookie_name")
	})
	schedule.Init()
}

func getOpenAPIKey() string {
	if openAPIkey != "" {
		return openAPIkey
	}
	openAPIkey = skbcfg.Get("properties.openapi_token")
	// 未配置时不使用内置默认值，由调用方按拒绝访问处理
	return openAPIkey
}
