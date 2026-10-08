package user

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gen"
	"gorm.io/gorm"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

func (s *svcImpl) AddUser(c *gin.Context, user *model.User) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("直接添加用户", "user", user)
	dbDo := query.Use(s.db).User.WithContext(c)
	err := dbDo.Create(user)
	if err != nil {
		logger.Warnw("数据库-直接添加用户错误", "err", err.Error())
		return err
	}
	return nil
}

func (s *svcImpl) AddUser2Company(c *gin.Context, user *model.User) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("添加用户", "user", user)
	dbDo := query.Use(s.db).User.WithContext(c)
	err := dbDo.Create(user)
	if err != nil {
		logger.Warnw("数据库-添加用户错误", "err", err.Error())
		return err
	}
	return nil
}

func (s *svcImpl) SearchUsers(c *gin.Context, param SearchParam) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户列表", "param", param)
	var (
		mc     = query.Use(s.db).User
		dbDo   = mc.WithContext(c)
		offset = (param.PageNum - 1) * param.PageSize
		limit  = param.PageSize
		q      = param.Query
	)
	// 检索条件
	conditions := param.toConditions(s.db)
	if q != "" {
		dbDo = mc.WithContext(c).Where(conditions...).
			Where(
				mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")),
			)
		queryInt, _ := strconv.ParseInt(q, 10, 64)
		if queryInt > 0 {
			dbDo = mc.WithContext(c).Where(conditions...).
				Where(
					mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).Or(mc.UserID.Eq(queryInt)),
				)
			if len(q) >= 4 {
				dbDo = mc.WithContext(c).Where(conditions...).
					Where(
						mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).
							Or(mc.UserID.Eq(queryInt)).
							Or(mc.Mobile.Like("%" + q + "%")),
					)
			}
		}
	} else {
		dbDo = mc.WithContext(c).Where(conditions...)
	}
	users, err := dbDo.Offset(offset).Limit(limit).Find()
	if err != nil {
		logger.Warnw("数据库-获取用户列表错误", "err", err.Error())
		return nil, err
	}
	return users, nil
}

func (s *svcImpl) GetUserNum(c *gin.Context, param SearchParam) (int, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户数量", "param", param)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
		q    = param.Query
	)
	// 检索条件
	conditions := param.toConditions(s.db)
	if q != "" {
		dbDo = mc.WithContext(c).Where(conditions...).
			Where(
				mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")),
			)
		queryInt, _ := strconv.ParseInt(q, 10, 64)
		if queryInt > 0 {
			dbDo = mc.WithContext(c).Where(conditions...).
				Where(
					mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).Or(mc.UserID.Eq(queryInt)),
				)
			if len(q) >= 4 {
				dbDo = mc.WithContext(c).Where(conditions...).
					Where(
						mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).
							Or(mc.UserID.Eq(queryInt)).
							Or(mc.Mobile.Like("%" + q + "%")),
					)
			}
		}
	} else {
		dbDo = mc.WithContext(c).Where(conditions...)
	}
	count, err := dbDo.Count()
	if err != nil {
		logger.Warnw("数据库-获取用户数量错误", "err", err.Error())
		return int(count), err
	}
	return int(count), nil
}

func (s *svcImpl) SelectUsers(c *gin.Context, param entity.SearchUserParam) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户列表，筛选参数：", "param", param)
	var (
		mc = query.Use(s.db).User
	)

	// 分页参数兜底
	pageNum := param.PageNum
	if pageNum <= 0 {
		pageNum = 1
	}
	pageSize := param.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (pageNum - 1) * pageSize
	limit := pageSize

	// 构造条件（状态、时间范围）
	var conditions []gen.Condition
	if len(param.Statuses) > 0 {
		conditions = append(conditions, mc.Status.In(param.Statuses...))
	}
	if param.StartTime > 0 {
		conditions = append(conditions, mc.CreateTime.Gte(param.StartTime))
	}
	if param.EndTime > 0 {
		conditions = append(conditions, mc.CreateTime.Lte(param.EndTime))
	}

	q := param.Query
	var dbDo = mc.WithContext(c)
	if q != "" {
		dbDo = mc.WithContext(c).Where(conditions...).
			Where(mc.Nickname.Like("%" + q + "%"))
		if queryInt, _ := strconv.ParseInt(q, 10, 64); queryInt > 0 {
			dbDo = mc.WithContext(c).Where(conditions...).
				Where(
					mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).Or(mc.UserID.Eq(queryInt)),
				)
			if len(q) >= 4 {
				dbDo = mc.WithContext(c).Where(conditions...).
					Where(
						mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).
							Or(mc.UserID.Eq(queryInt)).
							Or(mc.Mobile.Like("%" + q + "%")),
					)
			}
		}
	} else {
		dbDo = mc.WithContext(c).Where(conditions...)
	}

	users, err := dbDo.Offset(offset).Limit(limit).Find()
	if err != nil {
		logger.Warnw("数据库-获取用户列表错误", "err", err.Error())
		return nil, err
	}
	return users, nil
}

func (s *svcImpl) GetUserCount(c *gin.Context, param entity.SearchUserParam) (int, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库统计用户数量", "param", param)
	var (
		mc = query.Use(s.db).User
	)

	// 构造条件（状态、时间范围）
	var conditions []gen.Condition
	if len(param.Statuses) > 0 {
		conditions = append(conditions, mc.Status.In(param.Statuses...))
	}
	if param.StartTime > 0 {
		conditions = append(conditions, mc.CreateTime.Gte(param.StartTime))
	}
	if param.EndTime > 0 {
		conditions = append(conditions, mc.CreateTime.Lte(param.EndTime))
	}

	q := param.Query
	var dbDo = mc.WithContext(c)
	if q != "" {
		dbDo = mc.WithContext(c).Where(conditions...).
			Where(mc.Nickname.Like("%" + q + "%"))
		if queryInt, _ := strconv.ParseInt(q, 10, 64); queryInt > 0 {
			dbDo = mc.WithContext(c).Where(conditions...).
				Where(
					mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).Or(mc.UserID.Eq(queryInt)),
				)
			if len(q) >= 4 {
				dbDo = mc.WithContext(c).Where(conditions...).
					Where(
						mc.WithContext(c).Where(mc.Nickname.Like("%" + q + "%")).
							Or(mc.UserID.Eq(queryInt)).
							Or(mc.Mobile.Like("%" + q + "%")),
					)
			}
		}
	} else {
		dbDo = mc.WithContext(c).Where(conditions...)
	}

	count, err := dbDo.Count()
	if err != nil {
		logger.Warnw("数据库-获取用户数量错误", "err", err.Error())
		return int(count), err
	}
	return int(count), nil
}

func (s *svcImpl) GetFirstAdminCompanyUser(c *gin.Context, companyID int32) (*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息", "companyID", companyID)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.
		Where(mc.CompanyID.Eq(companyID)).
		Where(mc.Role.Eq(1)).
		Where(mc.Status.Eq(1)).
		Order(mc.UserID).First()
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误", "err", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *svcImpl) GetCompanyUser(c *gin.Context, companyID int32) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息", "companyID", companyID)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	users, err := dbDo.Where(mc.CompanyID.Eq(companyID)).Order(mc.UserID).Find()
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误", "err", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return users, nil
}

func (s *svcImpl) GetUser(c *gin.Context, userID int64) (*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息，", "userID", userID)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.Where(mc.UserID.Eq(userID)).First()
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误：", "err", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *svcImpl) GetUserByUsername(c *gin.Context, username string) (*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息", "username", username)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.Where(mc.Username.Eq(username)).First()
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误", "err", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *svcImpl) GetUserByMobile(c *gin.Context, mobile string) (*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息，按手机号", "mobile", mobile)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.Where(mc.Mobile.Eq(mobile)).First()
	if err != nil {
		logger.Warnw("数据库-按手机号获取用户信息错误", "err", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *svcImpl) GetUsers(c *gin.Context, userIDs []int64) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取用户信息", "userIDs", userIDs)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.Where(mc.UserID.In(userIDs...)).Find()
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误", "err", err.Error())
		return nil, err
	}
	return user, nil
}

func (s *svcImpl) UpdateUser(c *gin.Context, user *model.User) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新用户信息", "user", user)
	dbDo := query.Use(s.db).User.WithContext(c)
	err := dbDo.Save(user)
	if err != nil {
		logger.Warnw("数据库-更新用户错误", "err", err.Error())
		return err
	}
	return nil
}

// UpdateUserFields 精确更新用户资料字段，只 update 指定列，避免全表覆写
func (s *svcImpl) UpdateUserFields(c *gin.Context, userID int64, nickname, mobile, email, avatarFile string, updateTime int64) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("精确更新用户字段", "userID", userID)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	updates := map[string]interface{}{
		"nickname":    nickname,
		"mobile":      mobile,
		"email":       email,
		"avatar_file": avatarFile,
		"update_time": updateTime,
	}
	if _, err := dbDo.Where(mc.UserID.Eq(userID)).Updates(updates); err != nil {
		logger.Warnw("数据库-精确更新用户字段错误", "err", err.Error())
		return err
	}
	return nil
}

// UpdateUserStatus 更新用户状态
func (s *svcImpl) UpdateUserStatus(c *gin.Context, userID int64, status int32) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新用户状态", "userID", userID, "status", status)
	mc := query.Use(s.db).User
	_, err := mc.WithContext(c).Where(mc.UserID.Eq(userID)).Update(mc.Status, status)
	if err != nil {
		logger.Warnw("数据库-更新用户状态错误", "err", err.Error())
		return err
	}
	return nil
}

// nolint
func (s *svcImpl) DeleteUser(c *gin.Context, userID int64, pubType int32) (err error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("删除用户", "userID", userID)
	var (
		q      = query.Use(s.db)
		tx     = q.Begin()
		result gen.ResultInfo
	)
	defer func() {
		if recover() != nil || err != nil {
			logger.Warnw("事务回滚")
			_ = tx.Rollback()
		}
	}()
	logger.Infow("事务开启")

	// 删除用户
	result, err = tx.User.WithContext(c).Where(q.User.UserID.Eq(userID)).Delete()
	if err != nil {
		logger.Warnw("数据库-删除用户错误", "err", err.Error())
		return err
	}
	logger.Infow("删除用户结果", "result", result)

	err = tx.Commit()
	if err != nil {
		logger.Infow("事务提交错误", "err", err)
		return
	}
	logger.Infow("事务结束")
	return nil
}

func (s *svcImpl) DisableUser(c *gin.Context, userID int64) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("禁用用户", "userID", userID)
	mc := query.Use(s.db).User
	_, err := mc.WithContext(c).Where(mc.UserID.Eq(userID)).Update(mc.Status, 0)
	if err != nil {
		logger.Warnw("数据库-禁用用户错误", "err", err.Error())
		return err
	}
	return nil
}

func (s *svcImpl) EnableUser(c *gin.Context, userID int64) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("启用用户", "userID", userID)
	mc := query.Use(s.db).User
	_, err := mc.WithContext(c).Where(mc.UserID.Eq(userID)).Update(mc.Status, entity.CommonStatusAvailable)
	if err != nil {
		logger.Warnw("数据库-启用用户错误", "err", err.Error())
		return err
	}
	return nil
}

func (s *svcImpl) CheckAndAddUser(c *gin.Context, profile *entity.AuthUser) (*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("添加用户：", "profile", profile)
	mUser := query.Use(s.db).User
	if profile == nil || profile.ID == "" {
		logger.Warnw("用户信息为空", "profile", profile)
		return nil, fmt.Errorf("prfile或者用户ID不能为空")
	}
	now := time.Now()
	// 获取用户信息
	userInfo, err := s.GetUserByUsername(c, profile.Username)
	if err != nil {
		logger.Warnw("通过用户名获取用户信息失败", "err", err.Error())
		return nil, err
	}
	if userInfo != nil {
		return userInfo, nil
	}
	userInfo = &model.User{
		CompanyID:  1,
		Username:   profile.Username,
		Mobile:     profile.PhoneNum,
		Nickname:   profile.NickName,
		CreateTime: now.Unix(),
		UpdateTime: now.Unix(),
		Status:     entity.CommonStatusAvailable,
	}
	if err = mUser.WithContext(c).Create(userInfo); err != nil {
		logger.Warnw("数据库添加用户错误", "err", err.Error())
		return nil, err
	}
	return userInfo, nil
}

func (s *svcImpl) GetAllUser(c *gin.Context) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取所有用户（status=1）")
	mc := query.Use(s.db).User
	users, err := mc.WithContext(c).Find()
	if err != nil {
		logger.Warnw("数据库-获取所有用户错误", "err", err.Error())
		return nil, err
	}
	return users, nil
}

// GetActiveUsersByKeyword 获取所有状态为1且按关键字匹配的用户（昵称或手机号模糊匹配）
func (s *svcImpl) GetActiveUsersByKeyword(c *gin.Context, keyword string) ([]*model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	kw := strings.TrimSpace(keyword)
	logger.Infow("从数据库获取所有用户（status=1，按关键字模糊匹配）", "keyword", kw)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	if kw != "" {
		like := "%" + kw + "%"
		dbDo = dbDo.Where(mc.Nickname.Like(like), mc.Mobile.Like(like))
	}
	users, err := dbDo.Find()
	if err != nil {
		logger.Warnw("数据库-按关键字获取用户错误", "err", err.Error())
		return nil, err
	}
	return users, nil
}
