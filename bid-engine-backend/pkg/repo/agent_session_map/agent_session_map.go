package agentsessionmap

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"bid-engine/lib/common/logtool"
	"bid-engine/lib/common/storage"
)

// AgentSessionMap agent会话映射表
type AgentSessionMap struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id" comment:"自增主键"`                                                         // 自增主键
	OurConversationID   string    `gorm:"size:64;not null;index:idx_agent_our,unique" json:"our_conversation_id" comment:"本服务会话ID"`                  // 本服务会话ID
	AgentConversationID string    `gorm:"size:64;not null;index:idx_agent_conv,unique" json:"agent_conversation_id" comment:"Agent会话ID"`             // Agent会话ID
	AgentID             string    `gorm:"size:64;not null;index:idx_agent_our,unique;index:idx_agent_conv,unique" json:"agent_id" comment:"Agent标识"` // Agent标识
	CreatedAt           time.Time `json:"created_at" comment:"创建时间"`                                                                                 // 创建时间
	UpdatedAt           time.Time `json:"updated_at" comment:"更新时间"`                                                                                 // 更新时间
}

// TableName 指定表名
func (AgentSessionMap) TableName() string {
	return "agent_session_map"
}

// Service agent会话映射仓库接口
type Service interface {
	// GetByOurConv 根据本服务会话ID与agentID查询映射
	GetByOurConv(ctx context.Context, ourConvID, agentID string) (*AgentSessionMap, error)
	// GetByAgentConv 根据Agent会话ID与agentID查询映射
	GetByAgentConv(ctx context.Context, agentConvID, agentID string) (*AgentSessionMap, error)
	// Upsert 新增或更新映射（以 ourConvID+agentID 唯一）
	Upsert(ctx context.Context, ourConvID, agentConvID, agentID string) error
}

var (
	once     sync.Once
	instance Service
)

type svcImpl struct {
	logger *zap.SugaredLogger // 日志
	db     *gorm.DB           // 数据库
}

// GetInstance 单例获取
func GetInstance() Service {
	once.Do(func() {
		logger := logtool.GetLogger().Sugar()
		db := storage.GetDB()
		// 自动建表
		_ = db.AutoMigrate(&AgentSessionMap{})
		instance = &svcImpl{
			logger: logger,
			db:     db,
		}
	})
	return instance
}
