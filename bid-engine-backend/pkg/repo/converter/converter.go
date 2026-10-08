// Package converter 提供“Office 文档 → PDF”独立转换服务的 HTTP 客户端封装。
//
// 转换能力由单独的 doc-converter 依赖服务提供（见 docker-compose.yml），
// 后端镜像因此不需要安装 LibreOffice；handler / logic 层统一通过本包调用。
package converter

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"bid-engine/lib/common/logtool"
	skbcfg "bid-engine/pkg/config"
)

// Service 文档转换服务接口
type Service interface {
	// ConvertToPDF 把本地 Office 文档（.doc/.docx/.ppt/.pptx 等）转成 PDF，
	// 返回本地 PDF 临时文件路径；调用方负责删除该文件。
	ConvertToPDF(ctx context.Context, srcPath string) (string, error)

	// HealthCheck 检查转换服务是否可达
	HealthCheck(ctx context.Context) error
}

var (
	instance *svcImpl
	once     sync.Once
)

// GetInstance 获取文档转换服务单例
func GetInstance() Service {
	once.Do(func() {
		baseURL := strings.TrimSpace(skbcfg.Get("properties.converter_base_url"))
		if baseURL == "" {
			baseURL = "http://127.0.0.1:5010"
		}
		timeout := time.Duration(configSec("properties.converter_timeout_sec", 300)) * time.Second
		logtool.GetLogger().Sugar().Infow("文档转换服务初始化",
			"baseURL", baseURL,
			"timeout", timeout,
		)
		instance = &svcImpl{
			logger:  logtool.GetLogger().Sugar(),
			baseURL: strings.TrimSuffix(baseURL, "/"),
			client:  http.Client{Timeout: timeout},
		}
	})
	return instance
}

func configSec(key string, fallback int) int {
	if raw := strings.TrimSpace(skbcfg.Get(key)); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 10 && n <= 3600 {
			return n
		}
	}
	return fallback
}

type svcImpl struct {
	logger  *zap.SugaredLogger
	baseURL string
	client  http.Client
}

// ConvertToPDF 以流式 multipart 上传源文件，并把转换结果落盘为本地临时文件。
// 用流式上传是为了让 200MB 级的文档不至于整份读进内存。
func (s *svcImpl) ConvertToPDF(ctx context.Context, srcPath string) (string, error) {
	if strings.TrimSpace(srcPath) == "" {
		return "", fmt.Errorf("源文件路径为空")
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("打开源文件失败: %w", err)
	}
	defer func() { _ = src.Close() }()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var writeErr error
		defer func() {
			_ = mw.Close()
			_ = pw.CloseWithError(writeErr)
		}()
		part, err := mw.CreateFormFile("file", filepath.Base(srcPath))
		if err != nil {
			writeErr = err
			return
		}
		if _, err := io.Copy(part, src); err != nil {
			writeErr = err
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/convert", pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		return "", fmt.Errorf("构造转换请求失败: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	started := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用文档转换服务失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("文档转换失败（HTTP %d）: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out, err := os.CreateTemp("", "converted-*.pdf")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(out.Name())
		return "", fmt.Errorf("写入转换结果失败: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(out.Name())
		return "", fmt.Errorf("写入转换结果失败: %w", err)
	}
	fi, err := os.Stat(out.Name())
	if err != nil || fi.Size() == 0 {
		_ = os.Remove(out.Name())
		return "", fmt.Errorf("转换服务返回了空 PDF")
	}
	s.logger.Infow("文档转换成功",
		"src", filepath.Base(srcPath),
		"size", fi.Size(),
		"cost", time.Since(started).String(),
	)
	return out.Name(), nil
}

func (s *svcImpl) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("转换服务健康检查返回 HTTP %d", resp.StatusCode)
	}
	return nil
}
