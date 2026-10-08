// Package oss 包含了对象存储相关的操作，实现了aws s3 对文件的上传、下载等操作
package oss

import (
	"context"
	"io"
	"sync"

	"bid-engine/lib/common/config"
)

var (
	instance Service
	once     sync.Once
)

// Service 对象存储接口定义
type Service interface {
	// Upload 上传文件
	Upload(ctx context.Context, localFile io.Reader, remotePath string) error
	// Download 下载文件
	Download(ctx context.Context, remotePath string) ([]byte, error)
	// DownloadByRange 分片下载文件
	DownloadByRange(ctx context.Context, remotePath string, contentRange string) ([]byte, error)
	// Delete 删除文件
	Delete(ctx context.Context, remotePath string) error
	// Copy 复制文件
	Copy(ctx context.Context, targetPath, srcPath string) error
	// Exist 查看文件是否存在
	Exist(ctx context.Context, remotePath string) (bool, error)
}

// GetCosClient 获取Cos对象存储client
func GetCosClient(name string) Service {
	return NewCos(name)
}

// GetLocalClient 获取本地对象存储client
func GetLocalClient() Service {
	return NewLocal()
}

// GetInstance 获取OSS实例
func GetInstance() Service {
	conf := config.GetConfig()
	once.Do(func() {
		var (
			// 默认使用cos
			ossType = "cos"
			ossName = "default"
		)
		if conf.GetProperty("oss_type") != "" {
			ossType = conf.GetProperty("oss_type")
		}
		if conf.GetProperty("oss_name") != "" {
			ossName = conf.GetProperty("oss_name")
		}
		instance = GetOSSClient(ossType, ossName)
	})
	return instance
}

// GetOSSClient 获取 oss client
func GetOSSClient(ossType, ossName string) Service {
	if ossType == "cos" {
		return NewCos(ossName)
	}
	if ossType == "minio" {
		return NewMinio()
	}
	// 默认是local
	return NewLocal()
}
