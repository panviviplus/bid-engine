package material

import (
	"context"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
)

type ListParam struct {
	PageNum    int
	PageSize   int
	Keyword    string
	Type       string
	AuthUserID int64
}

type DeleteMaterialResult struct {
	ObjectKeys []string
}

type Service interface {
	ListMaterials(ctx context.Context, param ListParam) ([]*entity.MaterialListItem, int64, error)
	LookupMaterialForUser(ctx context.Context, userID, id int64) (*MaterialSummary, error)
	LookupMaterialsForUser(ctx context.Context, userID int64, ids []int64) ([]*MaterialSummary, error)
	DeleteMaterialGraphForUser(ctx context.Context, userID, id int64, materialType string) (*DeleteMaterialResult, error)

	ListCompaniesForUser(ctx context.Context, userID int64) ([]*entity.MaterialCompanyOption, error)
	ListUsersForUser(ctx context.Context, userID int64) ([]*entity.MaterialUserOption, error)

	ListMaterialFiles(c *gin.Context, materialID int64) ([]*model.MaterialFileInfo, error)
	GetMaterialFile(c *gin.Context, id int64) (*model.MaterialFileInfo, error)
	GetMaterialFileByObjectKey(c *gin.Context, objectKey string) (*model.MaterialFileInfo, error)
	CreateMaterialFile(c *gin.Context, f *model.MaterialFileInfo) error
	UpdateMaterialFileSortOrders(c *gin.Context, materialID int64, ids []int64) error
	UpdateMaterialFileInfo(c *gin.Context, id int64, fields map[string]interface{}) error
	DeleteMaterialFile(c *gin.Context, id int64) error
	DeleteMaterialFileGraph(c *gin.Context, id int64) (*model.MaterialFileInfo, []*model.MaterialImageInfo, error)
	DeleteMaterialFilesByMaterialID(c *gin.Context, materialID int64) error

	ListMaterialImages(c *gin.Context, materialID int64) ([]*model.MaterialImageInfo, error)
	ListMaterialImagesByUserID(c *gin.Context, userID int64) ([]*model.MaterialImageInfo, error)
	ListMaterialImagesByFileID(c *gin.Context, materialFileID int64) ([]*model.MaterialImageInfo, error)
	GetMaterialImage(c *gin.Context, id int64) (*model.MaterialImageInfo, error)
	GetMaterialImageByObjectKey(c *gin.Context, objectKey string) (*model.MaterialImageInfo, error)
	CreateMaterialImage(c *gin.Context, img *model.MaterialImageInfo) error
	GetMaxMaterialImageSortOrder(c *gin.Context, userID int64, materialID int64) (int32, error)
	UpdateMaterialImageSortOrders(c *gin.Context, materialID int64, materialFileIDs []int64) error
	UpdateMaterialImageInfo(c *gin.Context, id int64, fields map[string]interface{}) error
	DeleteMaterialImage(c *gin.Context, id int64) error
	DeleteMaterialImagesByMaterialID(c *gin.Context, materialID int64) error
	DeleteMaterialImagesByFileID(c *gin.Context, materialFileID int64) error

	GetCompanyName(c *gin.Context, companyID int32) (string, error)
	GetUserName(c *gin.Context, userID int64) (string, error)

	// MaterialOcrResult OCR 解析结果
	GetOcrResultByMaterialID(c *gin.Context, materialID int64) ([]*model.MaterialOcrResult, error)
	GetOcrResultByID(c *gin.Context, id int64) (*model.MaterialOcrResult, error)
	CreateOcrResult(c *gin.Context, r *model.MaterialOcrResult) error
	UpdateOcrResult(c *gin.Context, id int64, fields map[string]interface{}) error
	DeleteOcrResultsByMaterialID(c *gin.Context, materialID int64) error

	// 三张独立物料表的 CRUD
	CreateQualification(c *gin.Context, m *model.MaterialQualification) error
	GetQualificationForUser(ctx context.Context, userID, id int64) (*model.MaterialQualification, error)
	ListQualifications(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error)
	UpdateQualificationForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error
	DeleteQualificationForUser(ctx context.Context, userID, id int64) error

	CreatePerformance(c *gin.Context, m *model.MaterialPerformance) error
	GetPerformanceForUser(ctx context.Context, userID, id int64) (*model.MaterialPerformance, error)
	ListPerformances(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error)
	UpdatePerformanceForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error
	DeletePerformanceForUser(ctx context.Context, userID, id int64) error

	CreateTemplate(c *gin.Context, m *model.MaterialTemplate) error
	GetTemplateForUser(ctx context.Context, userID, id int64) (*model.MaterialTemplate, error)
	ListTemplates(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error)
	UpdateTemplateForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error
	DeleteTemplateForUser(ctx context.Context, userID, id int64) error
}

var (
	instance Service
	once     sync.Once
)

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return instance
}
