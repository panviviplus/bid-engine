package redis

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	commoncfg "bid-engine/lib/common/config"
	"bid-engine/lib/common/entity/env"
	"bid-engine/lib/common/logtool"
)

// Service Redis 通用操作接口（handler 层直接调用）
type Service interface {
	Ping(ctx context.Context) error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error

	// Client 暴露底层 *redis.Client 供高级操作（Pipeline、Pub/Sub 等）
	Client() *redis.Client
}

type svcImpl struct {
	logger *zap.SugaredLogger
	client *redis.Client
}

var (
	svcInstance Service
	once        sync.Once
)

// GetInstance 获取 Redis 服务单例（根据 env 自动选择本地/远程）
// env=local → conf-local.yml 无配置时 fallback 到 127.0.0.1:6379，否则使用 conf-local.yml 中的值
// 其他环境 → 读取 redis.host / redis.auth
func GetInstance() Service {
	once.Do(func() {
		cfg := commoncfg.GetConfig()
		svc := &svcImpl{logger: logtool.GetLogger().Sugar()}

		var addr, auth string
		if cfg.Env == env.Local {
			addr = cfg.Redis.Host
			if addr == "" {
				addr = "127.0.0.1:6379"
			}
		} else {
			addr = cfg.Redis.Host
			auth = cfg.Redis.Auth
		}

		svc.client = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: auth,
			DB:       0,
		})
		svcInstance = svc
	})
	return svcInstance
}
