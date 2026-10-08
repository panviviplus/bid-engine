package user

import (
	"gorm.io/gen"
	"gorm.io/gorm"

	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

// SearchParam 搜索参数
type SearchParam struct {
	entity.SearchParam
	CompanyID int32
}

func (s *SearchParam) toConditions(db *gorm.DB) []gen.Condition {
	var (
		mc         = query.Use(db).User
		conditions []gen.Condition
	)
	// 查询条件
	if s.CompanyID > 0 {
		conditions = append(conditions, mc.CompanyID.Eq(s.CompanyID))
	}
	if len(s.Statuses) > 0 {
		conditions = append(conditions, mc.Status.In(s.Statuses...))
	}
	return conditions
}
