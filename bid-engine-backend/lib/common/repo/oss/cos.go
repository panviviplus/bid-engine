package oss

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tencentyun/cos-go-sdk-v5"
	"go.uber.org/zap"

	libcos "bid-engine/lib/common/cos"
	"bid-engine/lib/common/logtool"
)

const (
	AuditInputTypePDF           = "pdf"
	AuditDetectTypePornPolitics = "Porn,Politics"
)

// Cos 腾讯云cos对象存储
type Cos struct {
	logger    *zap.SugaredLogger
	cosClient *cos.Client
}

// NewCos 实例化
func NewCos(name string) Service {
	client, _ := libcos.NewCosClient(name)
	// 连接s3
	return &Cos{
		logger:    logtool.MustGetLogger().Sugar(),
		cosClient: client,
	}
}

// Upload 上传文件
func (c *Cos) Upload(ctx context.Context, localFile io.Reader, remotePath string) error {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-上传,开始...", "remotePath", remotePath)
	_, err := c.cosClient.Object.Put(ctx, remotePath, localFile, nil)
	if err != nil {
		logger.Warnw("cos-上传,失败", "err", err)
		return err
	}
	logger.Infow("cos-上传,成功")
	return nil
}

// Download 下载文件
func (c *Cos) Download(ctx context.Context, remotePath string) ([]byte, error) {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-下载,开始...", "remotePath", remotePath)
	opt := &cos.ObjectGetOptions{
		XCosTrafficLimit: 838860800,
	}
	resp, err := c.cosClient.Object.Get(ctx, remotePath, opt)
	if err != nil {
		logger.Warnw("cos-下载,失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	if resp.Response.StatusCode != http.StatusOK {
		logger.Warnw("cos-下载,读取文件内容失败", "err", err,
			"remotePath", remotePath, "code", resp.Response.StatusCode, "body", fmt.Errorf("%v", resp.Response.Body))
		return nil, err
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnw("cos-下载,读取文件内容失败", "err", err, "remotePath", remotePath,
			"body", fmt.Errorf("%v", resp.Response.Body))
		return nil, err
	}
	logger.Info("cos-下载,成功")
	return content, nil
}

// DownloadByRange 分片下载文件
func (c *Cos) DownloadByRange(ctx context.Context, remotePath string, contentRange string) ([]byte, error) {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-分片下载,开始...", "remotePath", remotePath, "contentRange", contentRange)
	opt := &cos.ObjectGetOptions{
		XCosTrafficLimit: 838860800,
		Range:            contentRange,
	}
	resp, err := c.cosClient.Object.Get(ctx, remotePath, opt)
	if err != nil {
		logger.Warnw("cos-分片下载,失败", "err", err, "remotePath", remotePath)
		return nil, err
	}
	if resp.Response.StatusCode != http.StatusOK && resp.Response.StatusCode != http.StatusPartialContent {
		logger.Warnw("cos-下载,读取文件内容失败", "err", err,
			"remotePath", remotePath, "code", resp.Response.StatusCode,
			"body", fmt.Errorf("%v", resp.Response.Body))
		return nil, err
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnw("cos-分片下载,读取文件内容失败", "err", err, "remotePath", remotePath,
			"body", fmt.Errorf("%v", resp.Response.Body))
		return nil, err
	}
	logger.Info("cos-分片下载,成功")
	return content, nil
}

// Exist 查看文件是否存在
func (c *Cos) Exist(ctx context.Context, remotePath string) (bool, error) {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-检查是否存在，开始", "remotePath", remotePath)

	flag, err := c.cosClient.Object.IsExist(ctx, remotePath)
	if err != nil {
		logger.Warnw("cos-删除,失败", "err", err)
		return false, err
	}
	logger.Infow("cos-检查是否存在，结束", "remotePath", remotePath, "exist", flag)
	return flag, nil
}

// Delete 删除文件
func (c *Cos) Delete(ctx context.Context, remotePath string) error {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-删除,开始...", "remotePath", remotePath)
	_, err := c.cosClient.Object.Delete(ctx, remotePath)
	if err != nil {
		logger.Warnw("cos-删除,失败", "err", err)
		return err
	}
	logger.Infow("cos-删除,成功")
	return nil
}

// AuditDocument 审核文档
func (c *Cos) AuditDocument(ctx context.Context, remotePath, inputType, detectType string) (string, error) {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("param", "remotePath", remotePath, "inputType", inputType,
		"detectType", detectType)
	opt := &cos.PutDocumentAuditingJobOptions{
		InputObject: remotePath,
		InputType:   inputType,
		Conf: &cos.DocumentAuditingJobConf{
			DetectType: detectType,
		},
	}
	res, _, err := c.cosClient.CI.PutDocumentAuditingJob(ctx, opt)
	if err != nil {
		logger.Warnw(
			"call oss func [CI.PutDocumentAuditingJob]",
			"err", err, "remotePath", remotePath)
		return "", err
	}
	logger.Infow(
		"call oss func [CI.PutDocumentAuditingJob]",
		"req", opt, "res", res)
	return res.JobsDetail.JobId, nil
}

func (c *Cos) Copy(ctx context.Context, targetPath, srcPath string) error {
	logger := c.logger.With(logtool.WithCtx(ctx))
	logger.Infow("cos-复制,开始...", "targetPath", targetPath, "srcPath", srcPath)
	_, _, err := c.cosClient.Object.Copy(ctx, targetPath, srcPath, nil)
	if err != nil {
		logger.Warnw("cos-复制,失败", "err", err)
		return err
	}
	logger.Info("cos-复制,成功")
	return nil
}
