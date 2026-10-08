package agentsessionmap

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetByOurConv 根据本服务会话ID与agentID查询映射
func (s *svcImpl) GetByOurConv(ctx context.Context, ourConvID, agentID string) (*AgentSessionMap, error) {
	var m AgentSessionMap
	err := s.db.WithContext(ctx).
		Where("our_conversation_id = ? AND agent_id = ?", ourConvID, agentID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &m, err
}

// GetByAgentConv 根据Agent会话ID与agentID查询映射
func (s *svcImpl) GetByAgentConv(ctx context.Context, agentConvID, agentID string) (*AgentSessionMap, error) {
	var m AgentSessionMap
	err := s.db.WithContext(ctx).
		Where("agent_conversation_id = ? AND agent_id = ?", agentConvID, agentID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &m, err
}

// Upsert 新增或更新映射（以 ourConvID + agentID 为唯一约束侧）
func (s *svcImpl) Upsert(ctx context.Context, ourConvID, agentConvID, agentID string) error {
	now := time.Now()
	row := &AgentSessionMap{
		OurConversationID:   ourConvID,
		AgentConversationID: agentConvID,
		AgentID:             agentID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "our_conversation_id"}, {Name: "agent_id"}},
		DoUpdates: clause.Assignments(map[string]any{"agent_conversation_id": agentConvID, "updated_at": now}),
	}).Create(row).Error
}
