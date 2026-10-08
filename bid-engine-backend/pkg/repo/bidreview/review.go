package bidreview

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
)

// TermHit 术语召回命中的分块
type TermHit struct {
	ChunkID   int64
	FileID    int64
	PageStart int32
	PageEnd   int32
	Content   string
	Score     float64
}

// TermSeed 术语索引种子（ChunkNo 用于落库后回填 chunk_id）
type TermSeed struct {
	ChunkNo int32
	Term    string
	PageNo  int32
	Weight  int32
}

// DocumentPayload 单文件解析结果载荷
type DocumentPayload struct {
	Pages  []*model.BidReviewV2DocumentPage
	Blocks []*model.BidReviewV2DocumentBlock
	Chunks []*model.BidReviewV2DocumentChunk
	Terms  []*TermSeed
}

// Service 投标书审核模块（V2）数据访问接口
type Service interface {
	// ===== 项目 =====
	GetProjectByID(ctx context.Context, id int64) (*model.BidReviewV2Project, error)
	GetProjectForUser(ctx context.Context, userID, id int64) (*model.BidReviewV2Project, error)
	AddProject(ctx context.Context, p *model.BidReviewV2Project) error
	UpdateProjectFields(ctx context.Context, id int64, fields map[string]interface{}) error
	UpdateProjectStageStatus(ctx context.Context, id int64, stageStatus map[string]string, stage string, progress int32) error
	GetProjectsForUser(ctx context.Context, userID int64, pageNum, pageSize int, status, keyword string) ([]*model.BidReviewV2Project, int64, error)
	DeleteProjectCascadeForUser(ctx context.Context, userID, id int64) (*model.BidReviewV2Project, error)

	// ===== 文件 =====
	BatchCreateFiles(ctx context.Context, files []*model.BidReviewV2File) error
	GetFilesByProjectAndType(ctx context.Context, projectID int64, fileType string) ([]*model.BidReviewV2File, error)
	GetFileByID(ctx context.Context, id int64) (*model.BidReviewV2File, error)
	UpdateFile(ctx context.Context, id int64, fields map[string]interface{}) error

	// ===== 文档（页 / 块 / 分块 / 术语索引）=====
	ReplaceDocumentContent(ctx context.Context, projectID, fileID int64, payload *DocumentPayload) error
	GetBlocksByFilePage(ctx context.Context, fileID int64, pageNo int32) ([]*model.BidReviewV2DocumentBlock, error)
	GetBlocksByProjectType(ctx context.Context, projectID int64, blockType string, limit int) ([]*model.BidReviewV2DocumentBlock, error)
	GetPagesByFileRange(ctx context.Context, fileID int64, startPage, endPage int32) ([]*model.BidReviewV2DocumentPage, error)
	SearchTermIndex(ctx context.Context, projectID int64, terms []string, limit int) ([]*TermHit, error)
	DeleteDocumentContentByProject(ctx context.Context, projectID int64) error
	DeleteDocumentContentByType(ctx context.Context, projectID int64, fileType string) error

	// ===== 解析快照 =====
	SaveSnapshot(ctx context.Context, s *model.BidReviewV2AnalysisSnapshot) error
	GetSnapshot(ctx context.Context, projectID int64) (*model.BidReviewV2AnalysisSnapshot, error)
	DeleteSnapshot(ctx context.Context, projectID int64) error

	// ===== 清单项 =====
	BatchCreateChecklistItems(ctx context.Context, items []*model.BidReviewV2ChecklistItem) error
	GetChecklistItems(ctx context.Context, projectID int64) ([]*model.BidReviewV2ChecklistItem, error)
	GetChecklistItemByID(ctx context.Context, id int64) (*model.BidReviewV2ChecklistItem, error)
	UpdateChecklistItem(ctx context.Context, id int64, fields map[string]interface{}) error
	DeleteChecklistItem(ctx context.Context, id int64) error
	DeleteChecklistItemsBySource(ctx context.Context, projectID int64, sources []string) error
	CountChecklistItems(ctx context.Context, projectID int64) (total, passed, warning, errCount int64, err error)

	// ===== 判定与证据 =====
	// SaveVerdict 单事务写入判定 + 证据 +（可选）整改项
	SaveVerdict(ctx context.Context, finding *model.BidReviewV2Finding, evidences []*model.BidReviewV2Evidence, remediation *model.BidReviewV2Remediation) error
	GetLatestFindings(ctx context.Context, projectID int64) ([]*model.BidReviewV2Finding, error)
	GetLatestFindingByItem(ctx context.Context, itemID int64) (*model.BidReviewV2Finding, error)
	GetEvidencesByProject(ctx context.Context, projectID int64) ([]*model.BidReviewV2Evidence, error)
	DeleteFindingsByProject(ctx context.Context, projectID int64) error
	DeleteFindingsByDimension(ctx context.Context, projectID int64, dimension string) error

	// ===== 整改 =====
	UpsertRemediation(ctx context.Context, r *model.BidReviewV2Remediation) error
	GetRemediations(ctx context.Context, projectID int64) ([]*model.BidReviewV2Remediation, error)
	GetRemediationByID(ctx context.Context, id int64) (*model.BidReviewV2Remediation, error)
	UpdateRemediation(ctx context.Context, id int64, fields map[string]interface{}) error
	CountTodoRemediations(ctx context.Context, projectID int64) (int64, error)

	// ===== 规则库 =====
	ListRules(ctx context.Context, userID int64, dimension string, enabledOnly bool, keyword string) ([]*model.BidReviewV2Rule, error)
	GetRuleByID(ctx context.Context, id int64) (*model.BidReviewV2Rule, error)
	AddRule(ctx context.Context, r *model.BidReviewV2Rule) error
	UpdateRule(ctx context.Context, userID, id int64, fields map[string]interface{}) error
	DeleteRule(ctx context.Context, userID, id int64) error
	IncrRuleHit(ctx context.Context, ids []int64) error

	// ===== 阶段运行 =====
	UpsertStageRun(ctx context.Context, r *model.BidReviewV2StageRun) error
	UpdateStageRun(ctx context.Context, projectID int64, stage string, fields map[string]interface{}) error
	GetStageRuns(ctx context.Context, projectID int64) ([]*model.BidReviewV2StageRun, error)

	// ===== 操作日志 / 导出 =====
	AddOpLog(ctx context.Context, log *model.BidReviewV2OpLog) error
	GetOpLogs(ctx context.Context, projectID int64, limit int) ([]*model.BidReviewV2OpLog, error)
	AddExportRecord(ctx context.Context, r *model.BidReviewV2ExportRecord) error

	// ===== 主体信息（暗标身份信息比对用）=====
	GetCompanyName(ctx context.Context, companyID int32) (string, error)
}

var (
	instance Service
	once     sync.Once
)

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

// GetInstance 获取数据访问单例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return instance
}
