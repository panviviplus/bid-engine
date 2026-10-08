package oss

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"

	bidrepo "bid-engine/pkg/repo/bidhub"
)

// IsZwyEnv MinIO 本地环境时返回 true，使 bidhub 中已有的本地路径逻辑生效
func (s *svcImpl) IsZwyEnv() bool {
	return s.useMinio
}

// GetDefaultBucketName 根据后端返回对应桶名
func (s *svcImpl) GetDefaultBucketName() string {
	if s.useMinio {
		return bidrepo.ProjectFileSaveMinioBucket
	}
	return bidrepo.ProjectFileSaveCosBucket
}

// Put 上传文件（MinIO 或 COS）
func (s *svcImpl) Put(ctx context.Context, key, localPath string) error {
	if s.useMinio {
		_, err := s.minioCli.FPutObject(ctx, s.bucket, key, localPath, minio.PutObjectOptions{})
		return err
	}
	return s.cos.Put(ctx, key, localPath)
}

// Get 下载文件到本地（MinIO 或 COS）
func (s *svcImpl) Get(ctx context.Context, key, localPath string) error {
	if s.useMinio {
		return s.minioCli.FGetObject(ctx, s.bucket, key, localPath, minio.GetObjectOptions{})
	}
	return s.cos.Get(ctx, key, localPath)
}

// Open 打开文件流（MinIO 或 COS），调用方负责 Close
func (s *svcImpl) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.useMinio {
		return s.minioCli.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	}
	resp, err := s.cos.Download(ctx, key)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Delete 删除对象（MinIO 或 COS）
func (s *svcImpl) Delete(ctx context.Context, key string) error {
	if s.useMinio {
		return s.minioCli.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	}
	return s.cos.Delete(ctx, key)
}

// GetPresignedURL 生成GET预签名URL（MinIO 或 COS）
func (s *svcImpl) GetPresignedURL(ctx context.Context, key string, expire time.Duration) (*url.URL, error) {
	if s.useMinio {
		return s.minioCli.PresignedGetObject(ctx, s.bucket, key, expire, nil)
	}
	return s.cos.GetPresignedURL(ctx, key, expire)
}
