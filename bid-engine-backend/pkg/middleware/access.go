package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/config"
)

var (
	onceAllowedDomains sync.Once
	allowedDomains     []string
)

// 从配置读取允许的域名后缀，逗号分隔；例如：cors_allowed_domains: "example.com,example.org"
func getAllowedDomains() []string {
	onceAllowedDomains.Do(func() {
		val := config.GetConfig().GetProperty("cors_allowed_domains")
		if val != "" {
			for _, p := range strings.Split(val, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					// 统一去掉前导点，方便后续 apex 与 subdomain 判断
					allowedDomains = append(allowedDomains, strings.TrimPrefix(p, "."))
				}
			}
		}
		// 配置缺省时回落到内置默认后缀（修正为不带前导点）
		if len(allowedDomains) == 0 {
			// 默认不放开任何外部域名：本地联调已在 isValidOrigin 中单独放开，
			// 部署方通过 properties.cors_allowed_domains 配置自己的域名后缀。
			allowedDomains = []string{}
		}
	})
	return allowedDomains
}

func isValidOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// 解析 Origin，提取主机名，避免端口号影响后缀匹配
	if u, err := url.Parse(origin); err == nil && u.Hostname() != "" {
		origin = u.Hostname()
	}

	// 放开本地联调（如不需要可去掉）
	if origin == "localhost" || origin == "127.0.0.1" {
		return true
	}

	// 使用配置化后缀，兼容 apex 与子域
	for _, d := range getAllowedDomains() {
		if origin == d || strings.HasSuffix(origin, "."+d) {
			return true
		}
	}

	return false
}

// AccessControl 访问控制
func AccessControl() gin.HandlerFunc {

	return func(ctx *gin.Context) {
		// 默认放行的请求头，补充常见头：Authorization / X-Requested-With / Accept / Request-Id
		allowHeaders := "Origin, Content-Type, Content-Scene, Authorization, X-Requested-With, Accept, Request-Id"
		origin := ctx.GetHeader("Origin")
		if isValidOrigin(origin) {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", allowHeaders)
			ctx.Header("Access-Control-Allow-Credentials", "true")
		}

		if ctx.Request.Method == "OPTIONS" {
			// 注意标准首字母大小写：Access-Control-Request-Headers
			extHeader := ctx.GetHeader("Access-Control-Request-Headers")
			if extHeader != "" {
				allowHeaders += "," + extHeader
			}
			// 预检响应也需要完整回包
			if isValidOrigin(origin) {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				ctx.Header("Access-Control-Allow-Credentials", "true")
			}

			ctx.Header("Access-Control-Allow-Headers", allowHeaders)
			ctx.AbortWithStatus(http.StatusOK)
		}
		ctx.Next()
	}
}
