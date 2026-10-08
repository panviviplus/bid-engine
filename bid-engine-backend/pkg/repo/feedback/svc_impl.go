package feedback

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gen/field"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

// AddRecord 新增反馈记录
func (s *svcImpl) AddRecord(c *gin.Context, r *model.FeedbackRecord) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("新增反馈记录", "record", r)
	requestedStatus := r.Status
	return s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		record := query.Use(tx).FeedbackRecord
		if err := record.WithContext(c).Create(r); err != nil {
			return err
		}
		if requestedStatus != entity.CommonStatusUnavailable {
			return nil
		}
		if _, err := record.WithContext(c).
			Where(record.ID.Eq(r.ID)).
			Update(record.Status, int32(requestedStatus)); err != nil {
			return err
		}
		r.Status = int32(requestedStatus)
		return nil
	})
}

// UpdateRecord 更新反馈记录
func (s *svcImpl) UpdateRecord(c *gin.Context, r *model.FeedbackRecord) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新反馈记录", "record", r)
	return query.Use(s.db).FeedbackRecord.WithContext(c).Save(r)
}

// UpdateDescription 仅更新反馈描述与更新时间
func (s *svcImpl) UpdateDescription(c *gin.Context, id int64, description string) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新反馈记录描述", "id", id)
	mc := query.Use(s.db).FeedbackRecord
	_, err := mc.WithContext(c).Where(mc.ID.Eq(id)).Updates(map[string]interface{}{
		"description": description,
		"update_time": time.Now(),
	})
	return err
}

func (s *svcImpl) UpdateDescriptionForUser(ctx context.Context, userID, id int64, description string) error {
	if userID <= 0 || id <= 0 {
		return gorm.ErrRecordNotFound
	}
	record := query.Use(s.db).FeedbackRecord
	result, err := record.WithContext(ctx).Where(record.ID.Eq(id), record.UserID.Eq(userID)).Updates(map[string]interface{}{
		"description": description,
		"update_time": time.Now(),
	})
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus 仅更新反馈状态与更新时间
func (s *svcImpl) UpdateStatus(c *gin.Context, id int64, status int32) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新反馈记录状态", "id", id, "status", status)
	mc := query.Use(s.db).FeedbackRecord
	_, err := mc.WithContext(c).Where(mc.ID.Eq(id)).Updates(map[string]interface{}{
		"status":      status,
		"update_time": time.Now(),
	})
	return err
}

// DeleteRecord 删除反馈记录（物理删除）
func (s *svcImpl) DeleteRecord(c *gin.Context, id int64) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("删除反馈记录", "id", id)
	mc := query.Use(s.db).FeedbackRecord
	_, err := mc.WithContext(c).Where(mc.ID.Eq(id)).Delete()
	return err
}

func (s *svcImpl) DeleteRecordForUser(ctx context.Context, userID, id int64) error {
	if userID <= 0 || id <= 0 {
		return gorm.ErrRecordNotFound
	}
	record := query.Use(s.db).FeedbackRecord
	result, err := record.WithContext(ctx).Where(record.ID.Eq(id), record.UserID.Eq(userID)).Delete()
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetRecord 获取单条反馈记录
func (s *svcImpl) GetRecord(c *gin.Context, id int64) (*model.FeedbackRecord, error) {
	mc := query.Use(s.db).FeedbackRecord
	return mc.WithContext(c).Where(mc.ID.Eq(id)).First()
}

func (s *svcImpl) GetRecordForUser(ctx context.Context, userID, id int64) (*model.FeedbackRecord, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	record := query.Use(s.db).FeedbackRecord
	return record.WithContext(ctx).Where(record.ID.Eq(id), record.UserID.Eq(userID)).First()
}

func (s *svcImpl) HasPhotoKeyForUser(ctx context.Context, userID int64, objectKey string) (bool, error) {
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return false, nil
	}
	queryDB := s.db.WithContext(ctx).Model(&model.FeedbackRecord{}).Select("photo_keys")
	if userID > 0 {
		queryDB = queryDB.Where("user_id = ?", userID)
	}
	var rows []struct {
		PhotoKeys string
	}
	if err := queryDB.Scan(&rows).Error; err != nil {
		return false, err
	}
	for _, row := range rows {
		for _, candidate := range strings.Split(row.PhotoKeys, entity.FeedbackPhotoKeySep) {
			if strings.TrimSpace(candidate) == key {
				return true, nil
			}
		}
	}
	return false, nil
}

// SearchRecords 条件查询反馈记录（分页）
func (s *svcImpl) SearchRecords(c *gin.Context, param SearchParam) ([]*model.FeedbackRecord, int64, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("查询反馈记录", "param", param)

	// 默认分页
	if param.PageNum <= 0 {
		param.PageNum = 1
	}
	if param.PageSize <= 0 || param.PageSize > 50 {
		param.PageSize = 10
	}
	offset := (param.PageNum - 1) * param.PageSize

	mc := query.Use(s.db).FeedbackRecord
	dbDo := mc.WithContext(c)

	// 关键词模糊匹配：昵称或描述（OR）
	if strings.TrimSpace(param.Query) != "" {
		like := "%" + strings.TrimSpace(param.Query) + "%"
		dbDo = dbDo.Where(field.Or(
			mc.Nickname.Like(like),
			mc.Description.Like(like),
		))
	}
	conds := param.toConditions(s.db)
	if len(conds) > 0 {
		dbDo = dbDo.Where(conds...)
	}

	list, total, err := dbDo.Order(mc.CreateTime.Desc()).FindByPage(offset, param.PageSize)
	return list, total, err
}

// GetRecordNum 统计数量（配合分页或导出）
func (s *svcImpl) GetRecordNum(c *gin.Context, param SearchParam) (int64, error) {
	mc := query.Use(s.db).FeedbackRecord
	dbDo := mc.WithContext(c)
	conds := param.toConditions(s.db)
	if len(conds) > 0 {
		dbDo = dbDo.Where(conds...)
	}
	cnt, err := dbDo.Count()
	return cnt, err
}

// GetTypes 获取反馈类型选项（去重）
func (s *svcImpl) GetTypes(c *gin.Context) ([]string, error) {
	mc := query.Use(s.db).FeedbackRecord
	rows, err := mc.WithContext(c).Select(mc.Type).Group(mc.Type).Find()
	if err != nil {
		return nil, err
	}
	types := make([]string, 0, len(rows))
	for _, r := range rows {
		if r != nil && r.Type != "" {
			types = append(types, r.Type)
		}
	}
	return types, nil
}

func (s *svcImpl) GetTypesForUser(ctx context.Context, userID int64) ([]string, error) {
	if userID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	record := query.Use(s.db).FeedbackRecord
	rows, err := record.WithContext(ctx).Select(record.Type).Where(record.UserID.Eq(userID)).Group(record.Type).Find()
	if err != nil {
		return nil, err
	}
	types := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Type != "" {
			types = append(types, row.Type)
		}
	}
	return types, nil
}
