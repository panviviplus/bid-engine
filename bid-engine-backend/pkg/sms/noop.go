package sms

import (
	"context"

	"go.uber.org/zap"
	"bid-engine/lib/common/logtool"
)

// NoopProvider 开发/测试环境短信 Provider —— 仅输出日志，不真实发送短信。
type NoopProvider struct {
	logger *zap.SugaredLogger
}

// NewNoopProvider 创建一个 noop 短信 Provider。
func NewNoopProvider() *NoopProvider {
	return &NoopProvider{
		logger: logtool.GetLogger().Sugar(),
	}
}

// Send 仅记录日志，不做真实短信发送。
func (p *NoopProvider) Send(_ context.Context, mobile, code, scene string) error {
	p.logger.Infow("[SMS] noop send",
		"scene", scene,
		"mobile", maskMobile(mobile),
		"code", code,
	)
	return nil
}

// maskMobile 对手机号脱敏显示（保留前3后4）。
func maskMobile(mobile string) string {
	if len(mobile) < 7 {
		return mobile
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
