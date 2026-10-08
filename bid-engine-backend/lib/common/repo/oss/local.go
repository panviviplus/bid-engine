package oss

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
)

// Local 本地存储
type Local struct {
	logger *zap.SugaredLogger
}

// NewLocal 实例化
func NewLocal() Service {
	return &Local{
		logger: logtool.GetLogger().Sugar(),
	}
}

// Upload 上传文件
func (l *Local) Upload(ctx context.Context, localFile io.Reader, remotePath string) error {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-上传,开始...", "remotePath", remotePath)
	// 创建目录
	dir := filepath.Dir(remotePath)
	_ = os.MkdirAll(dir, 0777)
	// 创建文件
	out, err := os.Create(remotePath)
	if err != nil {
		logger.Infow("local-上传,创建目录失败", "err", err.Error())
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, localFile)
	if err != nil {
		logger.Warnw("local-上传,Copy到目标目录失败", "err", err)
		return err
	}
	logger.Infow("local-上传,成功")
	return nil
}

// Download 下载文件
func (l *Local) Download(ctx context.Context, remotePath string) ([]byte, error) {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-下载,开始...", "remotePath", remotePath)
	content, err := ioutil.ReadFile(remotePath)
	if err != nil {
		logger.Warnw("local-下载,失败", "err", err)
		return nil, err
	}
	logger.Infow("local-下载,成功")
	return content, nil
}

const (
	HeaderRange = "Range"
)

func getRange(bytesRange string, fileSize int64) (start, end int64, err error) {
	ranges := strings.Split(strings.Trim(bytesRange, "bytes="), "-")
	if len(ranges) != 2 {
		err = fmt.Errorf("wrong %s:%s", HeaderRange, bytesRange)
		return
	}
	start, err = strconv.ParseInt(ranges[0], 10, 64)
	if err != nil {
		return
	}
	if ranges[1] == "" {
		ranges[1] = strconv.FormatInt(fileSize-1, 10)
	}
	end, err = strconv.ParseInt(ranges[1], 10, 64)
	if err != nil {
		return
	}
	return
}

func (l *Local) DownloadByRange(ctx context.Context, remotePath string, contentRange string) ([]byte, error) {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-分片下载,开始...", "remotePath", remotePath, "contentRange", contentRange)
	fi, err := os.Stat(remotePath)
	if err != nil {
		logger.Warnw("local-分片下载,失败", "err", err, "remotePath", remotePath, "contentRange", contentRange)
		return nil, err
	}
	start, end, err := getRange(contentRange, fi.Size())
	if err != nil {
		logger.Warnw("local-分片下载,失败", "err", err, "remotePath", remotePath, "contentRange", contentRange)
		return nil, err
	}
	sourceFile, err := os.Open(remotePath)
	defer sourceFile.Close()
	if err != nil {
		logger.Warnw("local-分片下载,失败", "err", err, "remotePath", remotePath, "contentRange", contentRange)
		return nil, err
	}
	content, err := io.ReadAll(sourceFile)
	if err != nil {
		logger.Warnw("local-分片下载,失败", "err", err, "remotePath", remotePath, "contentRange", contentRange)
		return nil, err
	}
	logger.Infow("local-分片下载,成功")
	return content[start : end+1], nil
}

// Delete 删除文件
func (l *Local) Delete(ctx context.Context, remotePath string) error {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-删除,开始...", "remotePath", remotePath)
	err := os.Remove(remotePath)
	if err != nil {
		logger.Warnw("local-删除,失败", "err", err)
		return err
	}
	logger.Infow("local-删除,成功")
	return nil
}

func (l *Local) Copy(ctx context.Context, targetPath, srcPath string) error {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-复制,开始...", "targetPath", targetPath, "srcPath", srcPath)
	// 打开源文件
	src, err := os.Open(srcPath)
	if err != nil {
		logger.Warnw("local-复制,读取原始文件错误", "err", err)
		return err
	}
	defer src.Close()
	// 创建目标文件
	dir := filepath.Dir(targetPath)
	_ = os.MkdirAll(dir, 0777)
	// 创建文件
	target, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		logger.Infow("local-复制,创建目标文件错误", "err", err.Error())
		return err
	}
	//使用结束关闭文件
	defer target.Close()
	size, err := io.Copy(target, src)
	if err != nil {
		logger.Warnw("local-复制,错误", "err", err)
		return err
	}
	logger.Infow("local-复制,成功", "size", size)
	return nil
}

func (l *Local) Exist(ctx context.Context, remotePath string) (bool, error) {
	logger := l.logger.With(logtool.Ctx(ctx)...)
	logger.Infow("local-检查是否存在，开始", "remotePath", remotePath)
	exist := fileExists(remotePath)
	logger.Infow("local-检查是否存在，结束", "remotePath", remotePath, "exist", exist)
	return exist, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	// 如果错误是因为文件不存在，则返回 false
	if os.IsNotExist(err) {
		return false
	}
	// 否则返回 true，表示文件存在
	return err == nil
}
