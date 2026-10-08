package bidgen

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
)

var (
	ErrGenerationActive = errors.New("标书已有生成任务")
	ErrTaskNotRunnable  = errors.New("生成任务当前不可执行")
)

// Service 标书生成数据仓库（7 张表 CRUD）
type Service interface {
	DB() *gorm.DB
	// ===== bid_gen_project =====
	AddProject(ctx context.Context, p *model.BidGenProject) error
	GetProjectByID(ctx context.Context, id int64) (*model.BidGenProject, error)
	GetProjectForUser(ctx context.Context, userID, id int64) (*model.BidGenProject, error)
	UpdateProjectFields(ctx context.Context, id int64, fields map[string]interface{}) error
	UpdateProjectForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error
	GetProjectsForUser(ctx context.Context, userID int64, pageNum, pageSize int, status, name string) ([]*model.BidGenProject, int64, error)
	CountProjectsByTime(ctx context.Context, userID int64, startTime, endTime int64) (int64, error)
	DeleteProjectByID(ctx context.Context, id int64) error
	DeleteProjectForUser(ctx context.Context, userID, id int64) error
	DeleteProjectCascadeForUser(ctx context.Context, userID, id int64) (*model.BidGenProject, error)
	GetSourceSnapshot(ctx context.Context, bidProjectID int64) (*model.BidGenSourceSnapshot, error)

	// ===== bid_gen_outline =====
	AddOutlineNode(ctx context.Context, n *model.BidGenOutline) error
	BatchCreateOutlineNodes(ctx context.Context, nodes []*model.BidGenOutline) error
	GetOutlineByProjectID(ctx context.Context, projectID int64) ([]*model.BidGenOutline, error)
	// CountOutlineByProjectIDs 批量统计多个项目的章节总数与已生成数（避免 N+1）
	CountOutlineByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64][2]int64, error)
	GetOutlineByID(ctx context.Context, id int64) (*model.BidGenOutline, error)
	UpdateOutlineNode(ctx context.Context, id int64, fields map[string]interface{}) error
	// SetOutlineSubtreeCompleted 单事务：批量置位章节（含子章节）写作状态 + 重算项目完成度
	SetOutlineSubtreeCompleted(ctx context.Context, projectID int64, outlineIDs []int64, genStatus string, projectFields map[string]interface{}) error
	DeleteOutlineByID(ctx context.Context, id int64) error
	DeleteOutlineByProjectID(ctx context.Context, projectID int64) error
	DeleteOutlineSubtree(ctx context.Context, projectID, rootID int64) ([]int64, error)
	// ReconcileOutline 按文档标题结构对账大纲表（单事务），返回“请求数组下标 → 新 id”映射
	ReconcileOutline(ctx context.Context, projectID int64, items []entity.BidGenOutlineSyncItem) (map[int64]int64, error)
	// ApplyOutlineStructure 单事务批量更新大纲结构（parent_id/level/sort_order），仅接受本项目内节点
	ApplyOutlineStructure(ctx context.Context, projectID int64, nodes []entity.BidGenApplyOutlineNode) error

	// ===== bid_gen_chapter_content =====
	UpsertChapterContent(ctx context.Context, cc *model.BidGenChapterContent) error
	GetChapterContent(ctx context.Context, projectID, outlineID int64) (*model.BidGenChapterContent, error)
	GetChapterContentsByProjectID(ctx context.Context, projectID int64) ([]*model.BidGenChapterContent, error)
	SaveGeneratedChapter(ctx context.Context, cc *model.BidGenChapterContent, doc *model.BidGenDocContent, taskID int64, completedCount, progress int32) error
	DeleteChapterContentByProjectID(ctx context.Context, projectID int64) error
	DeleteChapterContentsByOutlineIDs(ctx context.Context, outlineIDs []int64) error

	// ===== bid_gen_doc_content =====
	UpsertDocContent(ctx context.Context, dc *model.BidGenDocContent) error
	GetDocContent(ctx context.Context, projectID int64) (*model.BidGenDocContent, error)
	DeleteDocContentByProjectID(ctx context.Context, projectID int64) error

	// ===== bid_gen_task =====
	AddTask(ctx context.Context, t *model.BidGenTask) error
	GetTaskByID(ctx context.Context, id int64) (*model.BidGenTask, error)
	GetTaskForUser(ctx context.Context, userID, id int64) (*model.BidGenTask, error)
	GetRunningTask(ctx context.Context, projectID int64) (*model.BidGenTask, error)
	CreateGenerationTask(ctx context.Context, userID int64, task *model.BidGenTask) error
	ClaimGenerationTask(ctx context.Context, taskID int64) (*model.BidGenTask, bool, error)
	ListActiveGenerationTasks(ctx context.Context) ([]*model.BidGenTask, error)
	UpdateTaskFields(ctx context.Context, id int64, fields map[string]interface{}) error
	UpdateTaskFieldsIfStatus(ctx context.Context, id int64, statuses []string, fields map[string]interface{}) (bool, error)
	FinishGeneration(ctx context.Context, projectID int64, projectFields map[string]interface{}, taskID int64, taskFields map[string]interface{}) error
	DeleteTasksByProjectID(ctx context.Context, projectID int64) error

	// ===== bid_gen_material_ref =====
	BatchCreateMaterialRefs(ctx context.Context, refs []*model.BidGenMaterialRef) error
	GetMaterialRefsByProject(ctx context.Context, projectID int64) ([]*model.BidGenMaterialRef, error)
	DeleteMaterialRefsByProject(ctx context.Context, projectID int64) error

	// ===== bid_gen_export_record =====
	AddExportRecord(ctx context.Context, r *model.BidGenExportRecord) error
	GetExportRecordsByProject(ctx context.Context, projectID int64) ([]*model.BidGenExportRecord, error)
	DeleteExportRecordsByProject(ctx context.Context, projectID int64) error
}

var (
	instance Service
	once     sync.Once
)

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

// GetInstance 获取仓库单例
func GetInstance() Service {
	once.Do(func() {
		instance = &svcImpl{
			logger: logtool.GetLogger().Sugar(),
			db:     storage.GetDB(),
		}
	})
	return instance
}

func (s *svcImpl) DB() *gorm.DB { return s.db }
