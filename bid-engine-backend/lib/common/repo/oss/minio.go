package oss

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"bid-engine/lib/common/config"
	"bid-engine/lib/common/logtool"
)

// Minio MinIO 对象存储（S3 兼容）
type Minio struct {
	logger *zap.SugaredLogger
	client *minio.Client
	bucket string
}

// NewMinio 创建 MinIO 客户端实例
func NewMinio() Service {
	cfg := config.GetConfig().Minio
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		logtool.MustGetLogger().Sugar().Fatalw("MinIO 客户端初始化失败", "err", err)
	}
	return &Minio{
		logger: logtool.MustGetLogger().Sugar(),
		client: client,
		bucket: cfg.DefaultBucket,
	}
}

// Upload 上传文件到 MinIO
func (m *Minio) Upload(ctx context.Context, localFile io.Reader, remotePath string) error {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-上传,开始...", "remotePath", remotePath, "bucket", m.bucket)
	_, err := m.client.PutObject(ctx, m.bucket, remotePath, localFile, -1, minio.PutObjectOptions{})
	if err != nil {
		logger.Warnw("minio-上传,失败", "err", err)
		return err
	}
	logger.Infow("minio-上传,成功")
	return nil
}

// Download 下载文件内容
func (m *Minio) Download(ctx context.Context, remotePath string) ([]byte, error) {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-下载,开始...", "remotePath", remotePath, "bucket", m.bucket)
	obj, err := m.client.GetObject(ctx, m.bucket, remotePath, minio.GetObjectOptions{})
	if err != nil {
		logger.Warnw("minio-下载,失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	defer obj.Close()
	content, err := io.ReadAll(obj)
	if err != nil {
		logger.Warnw("minio-下载,读取文件内容失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	logger.Info("minio-下载,成功")
	return content, nil
}

// DownloadByRange 分片下载文件
func (m *Minio) DownloadByRange(ctx context.Context, remotePath string, contentRange string) ([]byte, error) {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-分片下载,开始...", "remotePath", remotePath, "contentRange", contentRange)
	opts := minio.GetObjectOptions{}
	// MinIO 通过 Header 设置 Range
	opts.Set("Range", contentRange)
	obj, err := m.client.GetObject(ctx, m.bucket, remotePath, opts)
	if err != nil {
		logger.Warnw("minio-分片下载,失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	defer obj.Close()
	content, err := io.ReadAll(obj)
	if err != nil {
		logger.Warnw("minio-分片下载,读取文件内容失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	logger.Info("minio-分片下载,成功")
	return content, nil
}

// Delete 删除对象
func (m *Minio) Delete(ctx context.Context, remotePath string) error {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-删除,开始...", "remotePath", remotePath, "bucket", m.bucket)
	err := m.client.RemoveObject(ctx, m.bucket, remotePath, minio.RemoveObjectOptions{})
	if err != nil {
		logger.Warnw("minio-删除,失败", "err", err)
		return err
	}
	logger.Infow("minio-删除,成功")
	return nil
}

// Copy 复制对象
func (m *Minio) Copy(ctx context.Context, targetPath, srcPath string) error {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-复制,开始...", "targetPath", targetPath, "srcPath", srcPath, "bucket", m.bucket)
	src := minio.CopySrcOptions{
		Bucket: m.bucket,
		Object: srcPath,
	}
	dst := minio.CopyDestOptions{
		Bucket: m.bucket,
		Object: targetPath,
	}
	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		logger.Warnw("minio-复制,失败", "err", err)
		return err
	}
	logger.Info("minio-复制,成功")
	return nil
}

// Exist 检查对象是否存在
func (m *Minio) Exist(ctx context.Context, remotePath string) (bool, error) {
	logger := m.logger.With(logtool.WithCtx(ctx))
	logger.Infow("minio-检查是否存在，开始", "remotePath", remotePath, "bucket", m.bucket)
	_, err := m.client.StatObject(ctx, m.bucket, remotePath, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			logger.Infow("minio-检查是否存在，结束", "remotePath", remotePath, "exist", false)
			return false, nil
		}
		logger.Warnw("minio-检查是否存在，失败", "err", err)
		return false, err
	}
	logger.Infow("minio-检查是否存在，结束", "remotePath", remotePath, "exist", true)
	return true, nil
}
