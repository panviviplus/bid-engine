package feedback

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

// buildContentTypeByExt 根据文件扩展名推断 Content-Type
func buildContentTypeByExt(ext string) string {
	e := strings.ToLower(strings.TrimSpace(ext))
	switch e {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// sendOSSAsInlineImage 从 OSS 下载并以内联图片返回
func (s *svcImpl) sendOSSAsInlineImage(c *gin.Context, key string) error {
	// 确保临时目录存在
	if err := os.MkdirAll("./tmp", 0777); err != nil {
		return fmt.Errorf("make tmp dir failed: %w", err)
	}

	// 下载到临时文件
	tempPath := filepath.Join("./tmp", fmt.Sprintf("download-%d%s", time.Now().UnixNano(), filepath.Ext(key)))
	if err := s.oss.Get(entity.ConvertContext(c), key, tempPath); err != nil {
		return fmt.Errorf("download from OSS failed: %w", err)
	}
	defer os.Remove(tempPath)

	// 读取文件内容
	content, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("read file failed: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(key))
	ct := buildContentTypeByExt(ext)
	c.Header("Content-Type", ct)
	c.Data(200, ct, content)
	return nil
}

// GetImageByKey 根据OSS对象key获取图片源文件（以内联图片返回）
func (s *svcImpl) GetImageByKey(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	objectKey := c.Query("objectKey")
	if objectKey == "" {
		logger.Warnw("GetImageByKey --> objectKey为空")
		handler.SendNormalResp(c, entity.ErrCodeParam, "objectKey不能为空", nil)
		return
	}

	// URL解码（兼容带特殊字符的key）
	decodedKey, err := url.QueryUnescape(objectKey)
	if err != nil {
		logger.Warnw("GetImageByKey --> URL解码失败", "objectKey", objectKey, "err", err)
		decodedKey = objectKey
	}

	logger.Infow("GetImageByKey --> 获取图片", "objectKey", decodedKey)
	ownerID := entity.GetUserIDFromCtx(c)
	if entity.IsSuperAdmin(c) {
		ownerID = 0
	}
	allowed, err := s.repo.HasPhotoKeyForUser(c.Request.Context(), ownerID, decodedKey)
	if err != nil {
		logger.Warnw("GetImageByKey --> 校验图片归属失败", "objectKey", decodedKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "获取图片失败", nil)
		return
	}
	if !allowed {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
		return
	}

	if err := s.sendOSSAsInlineImage(c, decodedKey); err != nil {
		logger.Errorw("GetImageByKey --> 获取图片失败", "objectKey", decodedKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeS3Read, "获取图片失败", nil)
		return
	}
}
