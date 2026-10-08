package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Ping 测试 Redis 连接
func (s *svcImpl) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Get 获取 key 对应的值
func (s *svcImpl) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

// Set 设置 key-value，expiration 为 0 表示永不过期
func (s *svcImpl) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

// Del 删除一个或多个 key
func (s *svcImpl) Del(ctx context.Context, keys ...string) error {
	return s.client.Del(ctx, keys...).Err()
}

// Exists 检查 key 是否存在，返回存在的数量
func (s *svcImpl) Exists(ctx context.Context, keys ...string) (int64, error) {
	return s.client.Exists(ctx, keys...).Result()
}

// Expire 设置 key 的过期时间
func (s *svcImpl) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return s.client.Expire(ctx, key, expiration).Err()
}

// Client 暴露底层 *redis.Client
func (s *svcImpl) Client() *redis.Client {
	return s.client
}
