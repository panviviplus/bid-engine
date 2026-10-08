package material

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/pkg/repo/agent"
	repoLLM "bid-engine/pkg/repo/llm"
	"bid-engine/pkg/repo/docling"
	"bid-engine/pkg/repo/material"
	"bid-engine/pkg/repo/oss"
	repoPDF "bid-engine/pkg/repo/pdf"
	"bid-engine/lib/common/logtool"
)

var (
	instance Service
	once     sync.Once
)

type Service interface {
	Types(c *gin.Context)
	Companies(c *gin.Context)
	Users(c *gin.Context)

	List(c *gin.Context)
	// 类型专属列表接口
	ListQualifications(c *gin.Context)
	ListPerformances(c *gin.Context)
	ListTemplates(c *gin.Context)
	// 类型专属 CRUD
	AddQualification(c *gin.Context)
	DetailQualification(c *gin.Context)
	UpdateQualification(c *gin.Context)
	DeleteQualification(c *gin.Context)
	AddPerformance(c *gin.Context)
	DetailPerformance(c *gin.Context)
	UpdatePerformance(c *gin.Context)
	DeletePerformance(c *gin.Context)
	AddTemplate(c *gin.Context)
	DetailTemplate(c *gin.Context)
	UpdateTemplate(c *gin.Context)
	DeleteTemplate(c *gin.Context)

	Gallery(c *gin.Context)
	GalleryDelete(c *gin.Context)
	GalleryMove(c *gin.Context)

	GalleryUpload(c *gin.Context)
	AddFile(c *gin.Context)
	DeleteFile(c *gin.Context)
	DownloadFile(c *gin.Context)
	PreviewFile(c *gin.Context)

	EditImage(c *gin.Context)
	DeleteImage(c *gin.Context)

	SortImageFiles(c *gin.Context)
	SortDocFiles(c *gin.Context)

	// OCR 解析相关
	GetOcrResults(c *gin.Context)
	RetryOcr(c *gin.Context)
}

type svcImpl struct {
	logger     *zap.SugaredLogger
	repo       material.Service
	oss        oss.Service
	agentModel agent.Service
	pdf        repoPDF.Service
	llm        repoLLM.Service
	docling    docling.Service
}

func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger:     logtool.GetLogger().Sugar(),
			repo:       material.GetInstance(),
			oss:        oss.GetInstance(),
			agentModel: agent.GetInstance(),
			pdf:        repoPDF.GetInstance(),
			llm:        repoLLM.GetInstance(),
		docling:    docling.GetInstance(),
		}
	})
	return instance
}
