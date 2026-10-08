package cos

import (
	"net/url"
	"sync"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	libcos "bid-engine/lib/common/cos"
	"bid-engine/lib/common/logtool"
)

// cos 相关操作
type svcImpl struct {
	logger    *zap.SugaredLogger
	client    *cos.Client
	clientCdn *cos.Client
}

var (
	svcInstance Service
	once        sync.Once
)

type Service interface {
	DeleteCdn(ctx context.Context, name string) error
	PutCdn(ctx context.Context, name string, fileName string) error
	Get(ctx context.Context, name string, localPath string) error
	Put(ctx context.Context, name string, fileName string) error
	Delete(ctx context.Context, name string) error
	Exist(ctx context.Context, fileName string) (bool, error)
	Download(ctx context.Context, name string) (*cos.Response, error)
	DownloadByRange(ctx context.Context, name string, contentRange string) (*cos.Response, error)

	// GetPresignedURL 生成默认桶文件的GET预签名URL，用户WPS回调时文件加载
	GetPresignedURL(ctx context.Context, name string, expire time.Duration) (*url.URL, error)
	// PutPresignedURL 生成PUT方式的预签名URL（用于三阶段保存上传）
	PutPresignedURL(ctx context.Context, name string, expire time.Duration) (*url.URL, error)
}

// GetInstance 获取示例
func GetInstance() Service {
	once.Do(func() {
		client, _ := libcos.NewCosClient("default")
		clientCdn, _ := libcos.NewCosClient("cdn")

		svcInstance = &svcImpl{
			logger:    logtool.GetLogger().Sugar(),
			client:    client,
			clientCdn: clientCdn,
		}
	})
	return svcInstance
}
