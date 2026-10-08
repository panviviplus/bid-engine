package cos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"

	"bid-engine/lib/common/logtool"
)

// DeleteCdn 删除cdn文件
func (c *svcImpl) DeleteCdn(ctx context.Context, name string) error {
	log := c.logger.With(logtool.WithCtx(ctx))
	_, err := c.clientCdn.Object.Delete(ctx, name)
	if err != nil {
		log.Warnw("call Delete", "err", err, "name", name)
		return err
	}
	return nil
}

// Delete 删除标准桶中的对象
func (c *svcImpl) Delete(ctx context.Context, name string) error {
	log := c.logger.With(logtool.WithCtx(ctx))
	_, err := c.client.Object.Delete(ctx, name)
	if err != nil {
		log.Warnw("call Delete", "err", err, "name", name)
		return err
	}
	return nil
}

// PutCdn 上传
func (c *svcImpl) PutCdn(ctx context.Context, name string, fileName string) error {
	log := c.logger.With(logtool.WithCtx(ctx))
	_, _, err := c.clientCdn.Object.Upload(ctx, name, fileName, &cos.MultiUploadOptions{
		PartSize:       32,
		ThreadPoolSize: 32,
	})
	if err != nil {
		log.Warnw("call Upload",
			"err", err, "name", name, "fileName", fileName)
		return err
	}
	return nil
}

// Put 上传
func (c *svcImpl) Put(ctx context.Context, name string, fileName string) error {
	log := c.logger.With(logtool.WithCtx(ctx))
	_, _, err := c.client.Object.Upload(ctx, name, fileName, &cos.MultiUploadOptions{
		PartSize:       32,
		ThreadPoolSize: 32,
	})
	if err != nil {
		log.Warnw("call Upload",
			"err", err, "name", name)
		return err
	}
	return nil
}

// Get 下载文件到本地
func (c *svcImpl) Get(ctx context.Context, name string, localPath string) error {
	log := c.logger.With(logtool.WithCtx(ctx))

	// 下载文件内容
	resp, err := c.Download(ctx, name)
	if err != nil {
		log.Warnw("call Download", "err", err, "name", name)
		return err
	}
	defer resp.Body.Close()

	// 创建本地文件
	localFile, err := os.Create(localPath)
	if err != nil {
		log.Warnw("create local file failed", "err", err, "localPath", localPath)
		return err
	}
	defer localFile.Close()

	// 将内容写入本地文件
	_, err = io.Copy(localFile, resp.Body)
	if err != nil {
		log.Warnw("write to local file failed", "err", err, "localPath", localPath)
		return err
	}

	return nil
}

func (c *svcImpl) Exist(ctx context.Context, fileName string) (bool, error) {
	log := c.logger.With(logtool.WithCtx(ctx))
	flag, err := c.client.Object.IsExist(ctx, fileName)
	if err != nil {
		log.Warnw("call Upload",
			"err", err, "name", fileName)
		return false, err
	}
	return flag, nil
}

// Download 下载
func (c *svcImpl) Download(ctx context.Context, name string) (*cos.Response, error) {
	log := c.logger.With(logtool.WithCtx(ctx))
	opt := &cos.ObjectGetOptions{
		XCosTrafficLimit: 838860800,
	}
	resp, err := c.client.Object.Get(ctx, name, opt)
	if err != nil {
		log.Warnw("call Get",
			"err", err, "name", name)
		return nil, err
	}
	return resp, nil
}

// DownloadByRange 下载分片
func (c *svcImpl) DownloadByRange(ctx context.Context, name string, contentRange string) (*cos.Response, error) {
	log := c.logger.With(logtool.WithCtx(ctx))
	opt := &cos.ObjectGetOptions{
		XCosTrafficLimit: 838860800,
		Range:            contentRange,
	}
	resp, err := c.client.Object.Get(ctx, name, opt)
	if err != nil {
		log.Warnw("call Get",
			"err", err, "name", name)
		return nil, err
	}
	return resp, nil
}

// GetPresignedURL 生成默认桶文件的GET预签名URL，用户WPS回调时文件加载
func (s *svcImpl) GetPresignedURL(ctx context.Context, name string, expire time.Duration) (*url.URL, error) {
	// 容错，客户端未初始化直接返回错误
	if s.client == nil {
		return nil, errors.New("cos default client not initialized")
	}
	//获取客户端凭证
	cre := s.client.GetCredential()
	// 兜底校验：若凭证未正确加载，直接报错，避免生成缺少 q-ak 的预签名
	if cre == nil || cre.GetSecretId() == "" || cre.GetSecretKey() == "" {
		return nil, errors.New("cos default credential not loaded (empty secret_id/secret_key)")
	}
	// 生成GET方式的预签名URL
	return s.client.Object.GetPresignedURL(ctx, http.MethodGet, name, cre.GetSecretId(), cre.GetSecretKey(), expire, nil)
}

// PutPresignedURL 生成PUT方式的预签名URL（用于三阶段保存上传）
func (s *svcImpl) PutPresignedURL(ctx context.Context, name string, expire time.Duration) (*url.URL, error) {
	// 容错，客户端未初始化直接返回错误
	if s.client == nil {
		return nil, errors.New("cos default client not initialized")
	}
	// 获取凭证
	cre := s.client.GetCredential()
	if cre == nil || cre.GetSecretId() == "" || cre.GetSecretKey() == "" {
		return nil, errors.New("cos default credential not loaded (empty secret_id/secret_key)")
	}
	// 生成PUT方式的预签名URL
	return s.client.Object.GetPresignedURL(ctx, http.MethodPut, name, cre.GetSecretId(), cre.GetSecretKey(), expire, nil)
}
