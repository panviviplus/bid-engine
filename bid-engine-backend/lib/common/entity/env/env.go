// Package env 环境相关变量/常量配置
package env

import (
	"os"
	"strings"
)

const (
	// Local 本地环境
	Local = "local"
	// Dev 开发环境
	Dev = "dev"
	// Test 测试环境
	Test = "test"
	// Staging 预发布环境
	Staging = "staging"
	// Prod 正式环境
	Prod = "prod"
)

var envToDomain = map[string]string{
	Local:   "localhost",
	Dev:     "localhost",
	Test:    "localhost",
	Staging: "localhost",
	Prod:    "localhost",
}

// GetDomainByEnv 根据环境获取对应域名
func GetDomainByEnv(env string) string {
	return envToDomain[env]
}

const (
	canaryFlag = "-canary"
	canaryEnv  = "CANARY"
)

// IsCanaryEnv 判断是否是canary环境
func IsCanaryEnv() bool {
	// 通过CANARY环境变量获取
	if IsCanaryByEnv() {
		return true
	}
	// 通过HOSTNAME https://kubernetes.io/docs/concepts/containers/container-environment/
	hostname := os.Getenv("HOSTNAME")
	if strings.Contains(hostname, canaryFlag) {
		return true
	}
	return false
}

// IsCanaryByEnv 通过环境变量判断是否是canary环境，环境变量可以通过docker引入，或者服务层自己设置（SetCanaryEnv）
func IsCanaryByEnv() bool {
	envVal := os.Getenv(canaryEnv)
	return envVal == "1"
}

// SetCanaryEnv 设置当前环境为canary
func SetCanaryEnv() {
	_ = os.Setenv(canaryEnv, "1")
}
