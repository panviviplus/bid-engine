package company

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"


	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

type svcImpl struct {
	logger *zap.SugaredLogger
	db     *gorm.DB
}

func (s *svcImpl) GetCompany(c *gin.Context, companyID int32) (*model.Company, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取公司信息", "companyID", companyID)
	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
	)
	company, err := dbDo.Where(mc.ID.Eq(companyID)).First()
	if err != nil {
		logger.Warnw("数据库-获取公司信息失败", "err", err.Error())
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return company, nil
}

func (s *svcImpl) GetCompanyByNameBINARY(c *gin.Context, name string) (company *model.Company, err error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取公司信息", "name", name)
	var (
		//mc   = query.Use(s.db).Company
		dbDo = s.db.WithContext(c)
	)
	err = dbDo.Where("BINARY name = ?", name).First(&company).Error
	if err != nil {
		logger.Warnw("数据库-获取公司信息失败", "err", err.Error())
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return company, nil
}

func (s *svcImpl) GetCompanyByName(c *gin.Context, name string) (*model.Company, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("从数据库获取公司信息", "name", name)
	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
	)
	company, err := dbDo.Where(mc.Name.Eq(name)).First()
	if err != nil {
		logger.Warnw("数据库-获取公司信息失败", "err", err.Error())
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return company, nil
}

func (s *svcImpl) AddCompanyRecord(c *gin.Context, company *model.Company) (err error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("添加公司记录", "company", company)

	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
	)

	err = dbDo.Create(company)
	if err != nil {
		logger.Warnw("数据库-添加公司记录失败", "err", err.Error())
		return err
	}

	return nil
}

// ListCompany 分页查询公司列表
func (s *svcImpl) ListCompany(c *gin.Context, pageSize, pageNum int, name string) ([]*model.Company, int64, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("分页查询公司列表", "pageSize", pageSize, "pageNum", pageNum, "name", name)

	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
	)

	// 添加名称筛选条件
	if name != "" {
		dbDo = dbDo.Where(mc.Name.Like("%" + name + "%"))
	}

	// 查询总数
	total, err := dbDo.Count()
	if err != nil {
		logger.Warnw("数据库-查询公司总数失败", "err", err.Error())
		return nil, 0, err
	}

	// 分页查询
	offset := (pageNum - 1) * pageSize
	companies, err := dbDo.Limit(pageSize).Offset(offset).Order(mc.ID.Desc()).Find()
	if err != nil {
		logger.Warnw("数据库-分页查询公司列表失败", "err", err.Error())
		return nil, 0, err
	}

	return companies, total, nil
}

// DeleteCompany 删除公司
func (s *svcImpl) DeleteCompany(c *gin.Context, companyID int32) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("删除公司", "companyID", companyID)

	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
	)

	_, err := dbDo.Where(mc.ID.Eq(companyID)).Delete()
	if err != nil {
		logger.Warnw("数据库-删除公司失败", "err", err.Error())
		return err
	}

	return nil
}

// UpdateCompany 更新公司信息
func (s *svcImpl) UpdateCompany(c *gin.Context, companyID int32, userId int64) error {
	logger := s.logger.With(entity.Ctx(c)...)

	var (
		mc   = query.Use(s.db).Company
		dbDo = mc.WithContext(c)
		now  = time.Now().Unix()
	)

	_, err := dbDo.Where(mc.ID.Eq(companyID)).Updates(map[string]interface{}{
		"owner_id":    userId,
		"update_time": now,
	})
	if err != nil {
		logger.Warnw("数据库-更新公司负责人失败", "err", err.Error())
		return err
	}

	return nil
}

// GetCompanyWithOwner 获取公司信息及负责人信息
func (s *svcImpl) GetCompanyWithOwner(c *gin.Context, companyID int32) (*model.Company, *model.User, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("获取公司信息及负责人信息", "companyID", companyID)

	var (
		q         = query.Use(s.db)
		mc        = q.Company
		mu        = q.User
		companyDo = mc.WithContext(c)
		userDo    = mu.WithContext(c)
	)

	// 查询公司信息
	company, err := companyDo.Where(mc.ID.Eq(companyID)).First()
	if err != nil {
		logger.Warnw("数据库-获取公司信息失败", "err", err.Error())
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	// 查询负责人信息
	var owner *model.User
	if company.OwnerID > 0 {
		owner, err = userDo.Where(mu.UserID.Eq(company.OwnerID)).First()
		if err != nil && err != gorm.ErrRecordNotFound {
			logger.Warnw("数据库-获取负责人信息失败", "err", err.Error())
			return nil, nil, err
		}
	}

	return company, owner, nil
}

// ResetUserCompanyID 重置用户的公司ID为0
func (s *svcImpl) ResetUserCompanyID(c *gin.Context, companyID int32) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("重置用户的公司ID", "companyID", companyID)

	var (
		mu   = query.Use(s.db).User
		dbDo = mu.WithContext(c)
		now  = time.Now().Unix()
	)

	_, err := dbDo.Where(mu.CompanyID.Eq(companyID)).Updates(map[string]interface{}{
		"company_id":  0,
		"update_time": now,
	})
	if err != nil {
		logger.Warnw("数据库-重置用户公司ID失败", "err", err.Error())
		return err
	}

	return nil
}


// ListCompanyWithoutPage 不分页查询公司列表（按名称模糊筛选）
func (s *svcImpl) ListCompanyWithoutPage(c *gin.Context, name string) ([]*model.Company, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("不分页查询公司列表", "name", name)

	mc := query.Use(s.db).Company
	dbDo := mc.WithContext(c)

	if name != "" {
		dbDo = dbDo.Where(mc.Name.Like("%" + name + "%"))
	}
	companies, err := dbDo.Order(mc.CreateTime.Desc()).Find()
	if err != nil {
		logger.Warnw("数据库-不分页查询公司列表失败", "err", err.Error())
		return nil, err
	}

	return companies, nil
}

// HasCompanyOwner 判断是否存在任意公司负责人为指定用户（已认证：status==1）
func (s *svcImpl) HasCompanyOwner(c *gin.Context, ownerID int64) (bool, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("判断是否存在公司负责人为指定用户（已认证）", "ownerID", ownerID)

	mc := query.Use(s.db).Company
	dbDo := mc.WithContext(c)

	cnt, err := dbDo.Where(mc.OwnerID.Eq(ownerID)).Where(mc.Status.Eq(1)).Count()
	if err != nil {
		logger.Warnw("数据库-判断公司负责人存在失败", "err", err.Error())
		return false, err
	}
	return cnt > 0, nil
}

// UpdateCompanyOwner 更新公司负责人
func (s *svcImpl) UpdateCompanyOwner(c *gin.Context, companyID int32, ownerID int64) error {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("更新公司负责人", "companyID", companyID, "ownerID", ownerID)

	mc := query.Use(s.db).Company
	dbDo := mc.WithContext(c)
	now := time.Now().Unix()

	_, err := dbDo.Where(mc.ID.Eq(companyID)).Updates(map[string]interface{}{
		"owner_id":    ownerID,
		"is_show":     true,
		"update_time": now,
	})
	if err != nil {
		logger.Warnw("数据库-更新公司负责人失败", "err", err.Error())
		return err
	}
	return nil
}

// ListCompanyWithCounts 分页查询公司列表，包含资质和图片数量
func (s *svcImpl) ListCompanyWithCounts(c *gin.Context, pageSize, pageNum int, companyID int32) ([]*entity.CompanyListItem, int64, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	logger.Infow("查询公司列表", "pageSize", pageSize, "pageNum", pageNum)

	q := query.Use(s.db)

	// 构建查询条件
	companyQuery := q.Company.WithContext(c)
	if companyID > 0 {
		companyQuery = companyQuery.Where(q.Company.ID.Eq(companyID))
	}
	// 分页查询并获取总数
	offset := (pageNum - 1) * pageSize

	companies, count, err := companyQuery.Order(q.Company.CreateTime.Desc()).FindByPage(offset, pageSize)
	if err != nil {
		logger.Errorw("分页查询公司列表失败", "err", err)
		return nil, count, err
	}

	// 构建返回结果
	var result []*entity.CompanyListItem
	for _, company := range companies {
		// 基本信息
		companyName := company.Name
		unifiedCreditCode := ""
		legalRepresentative := ""

		// 资质数量
		qualificationCount := int64(0)

		// 图片数量
		imageCount := int64(0)

		item := &entity.CompanyListItem{
			CompanyID:           company.ID,
			CompanyName:         companyName,
			UnifiedCreditCode:   unifiedCreditCode,
			LegalRepresentative: legalRepresentative,
			QualificationCount:  qualificationCount,
			ImageCount:          imageCount,
		}

		result = append(result, item)
	}

	logger.Infow("查询公司列表成功", "total", count, "count", len(result))
	return result, count, nil
}
