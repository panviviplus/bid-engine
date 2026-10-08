// Package sms 短信验证码服务抽象层
// 当前仅提供 NoopProvider 用于开发调试，后续可接入阿里云/腾讯云等短信服务商。
package sms

import "context"

// Provider 短信发送接口，各短信服务商实现此接口。
type Provider interface {
	// Send 发送短信验证码到指定手机号。
	// scene 为业务场景标识，如 "login"、"register"，方便后续做模板区分。
	Send(ctx context.Context, mobile, code, scene string) error
}
