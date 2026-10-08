package oss

import (
	"io"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	commoncfg "bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/repo/cos"
)

// Service 提供统一的对象存储操作接口（支持 COS / MinIO 切换，由 conf-local.yml 的 oss_type 决定）
type Service interface {
	// Put 上传本地文件到对象存储
	Put(ctx context.Context, key, localPath string) error
	// Get 下载对象存储文件到本地路径
	Get(ctx context.Context, key, localPath string) error
	// Open 打开对象存储中的对象并返回可读流（调用方负责 Close）
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete 删除对象存储中的对象
	Delete(ctx context.Context, key string) error

	// IsZwyEnv MinIO 本地环境时返回 true；COS 时返回 false（历史遗留接口，名称保留兼容）
	IsZwyEnv() bool
	// GetDefaultBucketName 获取默认的文件桶名称
	GetDefaultBucketName() string

	// GetPresignedURL 生成对象的GET预签名URL
	GetPresignedURL(ctx context.Context, key string, expire time.Duration) (*url.URL, error)
}

type svcImpl struct {
	logger   *zap.SugaredLogger
	cos      cos.Service   // oss_type=cos 时使用（保留不变）
	minioCli *minio.Client // oss_type=minio 时使用（新增）
	bucket   string        // 当前使用的桶名
	useMinio bool          // 是否使用 MinIO 后端
}

var (
	svcInstance Service
	once        sync.Once
)

// GetInstance 获取统一对象存储服务实例（根据 oss_type 自动选择 COS 或 MinIO）
func GetInstance() Service {
	once.Do(func() {
		ossType := skbcfg.Get("properties.oss_type")
		svc := &svcImpl{
			logger: logtool.GetLogger().Sugar(),
		}

		if ossType == "minio" {
			cfg := commoncfg.GetConfig().Minio
			client, err := minio.New(cfg.Endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
				Secure: cfg.UseSSL,
			})
			if err != nil {
				svc.logger.Fatalw("MinIO 客户端初始化失败", "err", err)
			}
			svc.minioCli = client
			svc.bucket = cfg.DefaultBucket
			svc.useMinio = true
		} else {
			// 默认走 COS（oss_type=cos 或未设置时）
			svc.cos = cos.GetInstance()
			svc.useMinio = false
		}

		svcInstance = svc
	})
	return svcInstance
}
