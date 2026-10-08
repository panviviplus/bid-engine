package pdf

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/lib/common/logtool"
)

var (
	instance *svcImpl
	once     sync.Once
)

// Service PDF服务接口定义
type Service interface {
	// Parse 解析pdf文件
	Parse(c *gin.Context, fileName string) (*Data, error)
	// GetHtml 获取解析结果的html
	GetHtml(c *gin.Context, id string) (string, error)
	// Convert2Pdf 其他类型文档转换成pdf格式
	Convert2Pdf(c *gin.Context, srcFilename, targetFilename string) (err error)
	// Convert2PdfCompat 兼容性更强的docx转pdf
	Convert2PdfCompat(c *gin.Context, srcFilename, targetFilename string) error
	// Convert2PdfBySoffice 仅使用本地soffice/unoconv转换
	Convert2PdfBySoffice(c *gin.Context, srcFilename, targetFilename string) error
	ConvertDocToDocx(c *gin.Context, srcFilename, targetFilename string) error

	// CalculateCharIndex 根据PDF解析结果，计算每个字符相对全文的索引
	CalculateCharIndex(c *gin.Context, data *Data) ([]CharIndex, error)
}

// pdf 服务
type svcImpl struct {
	logger *zap.SugaredLogger
	client http.Client
}

// GetInstance 创建Term的实例
func GetInstance() Service {
	once.Do(func() {
		_pdfBaseURL = skbcfg.Get("properties.pdf_base_url")
		_pdfAppID = skbcfg.Get("properties.pdf_app_id")
		_pdfAppSecret = skbcfg.Get("properties.pdf_app_secret")
		// 超时时间
		timeout := time.Second * 7200
		timeoutSec, _ := strconv.Atoi(os.Getenv("PDF_TIMEOUT"))
		if timeoutSec > 0 {
			timeout = time.Second * time.Duration(timeoutSec)
		}
		logtool.GetLogger().Sugar().Infow("pdf服务超时时间", "timeout", timeout)
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			client: http.Client{
				Timeout: timeout,
			},
		}
	})
	return instance
}
