package feedback

import (
	"time"

	"gorm.io/gen"
	"gorm.io/gorm"

	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

// SearchParam 反馈记录查询参数（保持与 role/doc 风格一致）
type SearchParam struct {
	entity.SearchParam
	UserID    int64  // 用户ID（非管理员只能看自己）
	CompanyID int32  // 公司ID（过滤）
	Type      string // 反馈类型
	StartTime int64  // 创建开始时间（秒）
	EndTime   int64  // 创建结束时间（秒）
}

// toConditions 将参数转换为 gorm/gen 条件
func (p *SearchParam) toConditions(db *gorm.DB) []gen.Condition {
	var (
		mc         = query.Use(db).FeedbackRecord
		conditions []gen.Condition
	)

	// 状态筛选
	if len(p.Statuses) > 0 {
		conditions = append(conditions, mc.Status.In(p.Statuses...))
	}
	// 用户名筛选（昵称模糊匹配，保持与其他列表接口一致的子串匹配体验）
	// 用户ID筛选（用于权限控制）
	if p.UserID > 0 {
		conditions = append(conditions, mc.UserID.Eq(p.UserID))
	}
	// 公司ID筛选
	if p.CompanyID > 0 {
		conditions = append(conditions, mc.CompanyID.Eq(p.CompanyID))
	}
	// 类型筛选（字符串）
	if p.Type != "" {
		conditions = append(conditions, mc.Type.Eq(p.Type))
	}
	// 时间范围筛选（create_time 为 time.Time，使用 time.Unix）
	if p.StartTime > 0 {
		conditions = append(conditions, mc.CreateTime.Gte(time.Unix(p.StartTime, 0)))
	}
	if p.EndTime > 0 {
		conditions = append(conditions, mc.CreateTime.Lte(time.Unix(p.EndTime, 0)))
	}
	return conditions
}
