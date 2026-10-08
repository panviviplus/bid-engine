package bid

import (
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
)

func (s *svcImpl) AddBidProject(ctx *gin.Context, p *model.BidProject) error {
	log := s.logger.With("traceId", ctx.GetString("traceId"))
	dbDo := query.Use(s.db).BidProject.WithContext(ctx)
	log.Infow("数据库-新增bid_project记录", "project", p)
	return dbDo.Save(p)
}

// CountBidProjects 按用户与创建时间范围统计 bid_project 数量
func (s *svcImpl) CountBidProjects(ctx *gin.Context, userID int64, startTime, endTime int64) (int64, error) {
	bp := query.Use(s.db).BidProject
	q := bp.WithContext(ctx)
	if userID > 0 {
		q = q.Where(bp.UserID.Eq(userID))
	}
	if startTime > 0 {
		q = q.Where(bp.CreatedAt.Gte(time.Unix(startTime, 0)))
	}
	if endTime > 0 {
		q = q.Where(bp.CreatedAt.Lte(time.Unix(endTime, 0)))
	}
	return q.Count()
}
