package sms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	redisRepo "bid-engine/pkg/repo/redis"
)

const (
	otpKeyPrefix    = "sms:otp:"
	otpRLKeyPrefix  = "sms:rl:"
	otpTTL          = 5 * time.Minute
	otpRateLimitTTL = 1 * time.Minute
	otpUsedMarker   = "USED"
)

// OTPStore 基于 Redis 的验证码存储，替换原 sync.Map 实现。
type OTPStore struct {
	redis redisRepo.Service
}

// NewOTPStore 创建 Redis 验证码存储。
func NewOTPStore() *OTPStore {
	return &OTPStore{redis: redisRepo.GetInstance()}
}

func otpKey(scene, mobile string) string {
	return fmt.Sprintf("%s%s:%s", otpKeyPrefix, scene, mobile)
}

func otpRLKey(scene, mobile string) string {
	return fmt.Sprintf("%s%s:%s", otpRLKeyPrefix, scene, mobile)
}

// Set 存储验证码，自动设 5 分钟 TTL。
func (s *OTPStore) Set(ctx context.Context, scene, mobile, code string) error {
	return s.redis.Set(ctx, otpKey(scene, mobile), code, otpTTL)
}

// Verify 校验验证码。验证成功后将标记为 USED 并保留至 TTL 到期（防重放）。
func (s *OTPStore) Verify(ctx context.Context, scene, mobile, code string) (bool, error) {
	key := otpKey(scene, mobile)
	val, err := s.redis.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil // key 不存在（已过期或从未发送）
		}
		return false, err
	}
	if val == "" || val == otpUsedMarker {
		return false, nil
	}
	if val != code {
		return false, nil
	}
	// 标记为已使用，防重放
	existingTTL, err := s.redis.Exists(ctx, key)
	if err != nil || existingTTL == 0 {
		return false, nil
	}
	_ = s.redis.Set(ctx, key, otpUsedMarker, otpTTL)
	return true, nil
}

// CheckRateLimit 检查频率限制。返回 true 表示在冷却期内。
func (s *OTPStore) CheckRateLimit(ctx context.Context, scene, mobile string) (bool, error) {
	key := otpRLKey(scene, mobile)
	n, err := s.redis.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetRateLimit 设置频率限制标记（1 分钟冷却）。
func (s *OTPStore) SetRateLimit(ctx context.Context, scene, mobile string) error {
	return s.redis.Set(ctx, otpRLKey(scene, mobile), "1", otpRateLimitTTL)
}
