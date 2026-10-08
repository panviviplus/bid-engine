package material

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
)

func (s *svcImpl) ListMaterials(ctx context.Context, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	switch param.Type {
	case "qualification":
		return listMaterialsGeneric(s.db, ctx, "material_qualification", param)
	case "performance":
		return listMaterialsGeneric(s.db, ctx, "material_performance", param)
	case "template":
		return listMaterialsGeneric(s.db, ctx, "material_template", param)
	default:
		return listMaterialsUnion(s.db, ctx, param)
	}
}

func (s *svcImpl) DeleteMaterialGraphForUser(ctx context.Context, userID, id int64, materialType string) (*DeleteMaterialResult, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var parent any
	switch materialType {
	case "qualification":
		parent = &model.MaterialQualification{}
	case "performance":
		parent = &model.MaterialPerformance{}
	case "template":
		parent = &model.MaterialTemplate{}
	default:
		return nil, fmt.Errorf("unsupported material type: %s", materialType)
	}
	result := &DeleteMaterialResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=?", id, userID).First(parent).Error; err != nil {
			return err
		}
		var files []*model.MaterialFileInfo
		if err := tx.Where("material_id=?", id).Find(&files).Error; err != nil {
			return fmt.Errorf("查询素材文件: %w", err)
		}
		var images []*model.MaterialImageInfo
		if err := tx.Where("material_id=?", id).Find(&images).Error; err != nil {
			return fmt.Errorf("查询素材图片: %w", err)
		}
		keys := map[string]struct{}{}
		for _, file := range files {
			if key := strings.TrimSpace(file.ObjectKey); key != "" {
				keys[key] = struct{}{}
			}
		}
		for _, image := range images {
			if key := strings.TrimSpace(image.ObjectKey); key != "" {
				keys[key] = struct{}{}
			}
			if key := strings.TrimSpace(image.OriginObjectKey); key != "" {
				keys[key] = struct{}{}
			}
		}
		for key := range keys {
			result.ObjectKeys = append(result.ObjectKeys, key)
		}
		if err := tx.Where("material_id=?", id).Delete(&model.MaterialOcrResult{}).Error; err != nil {
			return fmt.Errorf("删除素材OCR: %w", err)
		}
		if err := tx.Where("material_id=?", id).Delete(&model.MaterialImageInfo{}).Error; err != nil {
			return fmt.Errorf("删除素材图片: %w", err)
		}
		if err := tx.Where("material_id=?", id).Delete(&model.MaterialFileInfo{}).Error; err != nil {
			return fmt.Errorf("删除素材文件: %w", err)
		}
		if err := tx.Delete(parent).Error; err != nil {
			return fmt.Errorf("删除素材主记录: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(result.ObjectKeys)
	return result, nil
}

func listMaterialsUnion(db *gorm.DB, ctx context.Context, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	tables := []string{"material_qualification", "material_performance", "material_template"}
	var all []*entity.MaterialListItem
	var total int64

	for _, table := range tables {
		p := param
		p.PageNum = 1
		p.PageSize = 10000 // fetch all, merge + paginate in Go
		list, count, err := listMaterialsGeneric(db, ctx, table, p)
		if err != nil {
			return nil, 0, err
		}
		total += count
		all = append(all, list...)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })

	offset := (param.PageNum - 1) * param.PageSize
	if offset >= len(all) {
		return []*entity.MaterialListItem{}, total, nil
	}
	end := offset + param.PageSize
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// MaterialSummary 跨表物料摘要
type MaterialSummary struct {
	ID          int64
	Name        string
	Type        string // "qualification", "performance", "template"
	Description string
	UserID      int64
	CompanyID   int32
	CreatedAt   time.Time
}

// LookupMaterial 跨三张物料表查询物料摘要
func (s *svcImpl) LookupMaterial(c *gin.Context, id int64) (*MaterialSummary, error) {
	if c == nil || id <= 0 {
		return nil, fmt.Errorf("invalid params")
	}
	tables := []struct {
		table string
		typ   string
	}{
		{"material_qualification", "qualification"},
		{"material_performance", "performance"},
		{"material_template", "template"},
	}
	for _, t := range tables {
		var row struct {
			ID          int64
			Name        string
			Description string
			UserID      int64
			CompanyID   int32
			CreatedAt   time.Time
		}
		err := s.db.WithContext(c).Table(t.table).
			Select("id, name, description, user_id, company_id, created_at").
			Where("id = ?", id).Scan(&row).Error
		if err != nil {
			continue
		}
		if row.ID > 0 {
			return &MaterialSummary{
				ID: row.ID, Name: row.Name, Type: t.typ,
				Description: row.Description, UserID: row.UserID,
				CompanyID: row.CompanyID, CreatedAt: row.CreatedAt,
			}, nil
		}
	}
	return nil, fmt.Errorf("material not found: %d", id)
}

func (s *svcImpl) LookupMaterialForUser(ctx context.Context, userID, id int64) (*MaterialSummary, error) {
	if ctx == nil || userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	tables := []struct {
		table string
		typ   string
	}{
		{"material_qualification", "qualification"},
		{"material_performance", "performance"},
		{"material_template", "template"},
	}
	for _, item := range tables {
		var row struct {
			ID          int64
			Name        string
			Description string
			UserID      int64
			CompanyID   int32
			CreatedAt   time.Time
		}
		err := s.db.WithContext(ctx).Table(item.table).
			Select("id, name, description, user_id, company_id, created_at").
			Where("id = ? AND user_id = ?", id, userID).Scan(&row).Error
		if err != nil {
			return nil, err
		}
		if row.ID > 0 {
			return &MaterialSummary{
				ID: row.ID, Name: row.Name, Type: item.typ,
				Description: row.Description, UserID: row.UserID,
				CompanyID: row.CompanyID, CreatedAt: row.CreatedAt,
			}, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// LookupMaterials 跨三张物料表批量查询物料摘要
func (s *svcImpl) LookupMaterials(c *gin.Context, ids []int64) ([]*MaterialSummary, error) {
	if c == nil || len(ids) == 0 {
		return nil, nil
	}
	// deduplicate
	seen := make(map[int64]bool, len(ids))
	clean := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil, nil
	}

	tables := []struct {
		table string
		typ   string
	}{
		{"material_qualification", "qualification"},
		{"material_performance", "performance"},
		{"material_template", "template"},
	}
	var result []*MaterialSummary
	for _, t := range tables {
		var rows []struct {
			ID          int64
			Name        string
			Description string
			UserID      int64
			CompanyID   int32
			CreatedAt   time.Time
		}
		if err := s.db.WithContext(c).Table(t.table).
			Select("id, name, description, user_id, company_id, created_at").
			Where("id IN ?", clean).Scan(&rows).Error; err != nil {
			continue
		}
		for _, row := range rows {
			result = append(result, &MaterialSummary{
				ID: row.ID, Name: row.Name, Type: t.typ,
				Description: row.Description, UserID: row.UserID,
				CompanyID: row.CompanyID, CreatedAt: row.CreatedAt,
			})
		}
	}
	return result, nil
}

func (s *svcImpl) LookupMaterialsForUser(ctx context.Context, userID int64, ids []int64) ([]*MaterialSummary, error) {
	if ctx == nil || userID <= 0 || len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[int64]bool, len(ids))
	clean := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	if len(clean) == 0 {
		return nil, nil
	}
	tables := []struct {
		table string
		typ   string
	}{
		{"material_qualification", "qualification"},
		{"material_performance", "performance"},
		{"material_template", "template"},
	}
	result := make([]*MaterialSummary, 0)
	for _, item := range tables {
		var rows []struct {
			ID          int64
			Name        string
			Description string
			UserID      int64
			CompanyID   int32
			CreatedAt   time.Time
		}
		if err := s.db.WithContext(ctx).Table(item.table).
			Select("id, name, description, user_id, company_id, created_at").
			Where("id IN ? AND user_id = ?", clean, userID).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			result = append(result, &MaterialSummary{
				ID: row.ID, Name: row.Name, Type: item.typ,
				Description: row.Description, UserID: row.UserID,
				CompanyID: row.CompanyID, CreatedAt: row.CreatedAt,
			})
		}
	}
	return result, nil
}

func (s *svcImpl) ListCompanies(c *gin.Context) ([]*entity.MaterialCompanyOption, error) {
	var all []*entity.MaterialCompanyOption
	tables := []string{"material_qualification", "material_performance", "material_template"}
	for _, table := range tables {
		var out []*entity.MaterialCompanyOption
		if err := s.db.WithContext(c).
			Table(table + " m").
			Select("DISTINCT m.company_id as company_id, c.name").
			Joins("JOIN company c ON m.company_id = c.id").
			Where("m.company_id IS NOT NULL AND m.company_id > 0").
			Scan(&out).Error; err != nil {
			continue
		}
		all = append(all, out...)
	}
	seen := make(map[int32]bool, len(all))
	uniq := make([]*entity.MaterialCompanyOption, 0, len(all))
	for _, o := range all {
		if o != nil && !seen[o.CompanyID] {
			seen[o.CompanyID] = true
			uniq = append(uniq, o)
		}
	}
	return uniq, nil
}

func (s *svcImpl) ListCompaniesForUser(ctx context.Context, userID int64) ([]*entity.MaterialCompanyOption, error) {
	if userID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var all []*entity.MaterialCompanyOption
	for _, table := range []string{"material_qualification", "material_performance", "material_template"} {
		var options []*entity.MaterialCompanyOption
		if err := s.db.WithContext(ctx).Table(table+" m").
			Select("DISTINCT m.company_id as company_id, c.name").
			Joins("JOIN company c ON m.company_id = c.id").
			Where("m.user_id = ? AND m.company_id > 0", userID).
			Scan(&options).Error; err != nil {
			return nil, err
		}
		all = append(all, options...)
	}
	seen := make(map[int32]bool, len(all))
	unique := make([]*entity.MaterialCompanyOption, 0, len(all))
	for _, option := range all {
		if option != nil && !seen[option.CompanyID] {
			seen[option.CompanyID] = true
			unique = append(unique, option)
		}
	}
	return unique, nil
}

func (s *svcImpl) ListUsers(c *gin.Context) ([]*entity.MaterialUserOption, error) {
	var all []*entity.MaterialUserOption
	tables := []string{"material_qualification", "material_performance", "material_template"}
	for _, table := range tables {
		var out []*entity.MaterialUserOption
		if err := s.db.WithContext(c).
			Table(table + " m").
			Select("DISTINCT m.user_id as user_id, CASE WHEN u.nickname <> '' THEN u.nickname ELSE u.username END as name").
			Joins("JOIN user u ON m.user_id = u.user_id").
			Where("m.user_id IS NOT NULL AND m.user_id > 0").
			Scan(&out).Error; err != nil {
			continue
		}
		all = append(all, out...)
	}
	seen := make(map[int64]bool, len(all))
	uniq := make([]*entity.MaterialUserOption, 0, len(all))
	for _, o := range all {
		if o != nil && !seen[o.UserID] {
			seen[o.UserID] = true
			uniq = append(uniq, o)
		}
	}
	return uniq, nil
}

func (s *svcImpl) ListUsersForUser(ctx context.Context, userID int64) ([]*entity.MaterialUserOption, error) {
	if userID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var option entity.MaterialUserOption
	err := s.db.WithContext(ctx).Table("user").
		Select("user_id, CASE WHEN nickname <> '' THEN nickname ELSE username END as name").
		Where("user_id = ?", userID).Scan(&option).Error
	if err != nil {
		return nil, err
	}
	if option.UserID == 0 {
		return []*entity.MaterialUserOption{}, nil
	}
	return []*entity.MaterialUserOption{&option}, nil
}

func (s *svcImpl) ListMaterialFiles(c *gin.Context, materialID int64) ([]*model.MaterialFileInfo, error) {
	mf := query.Use(s.db).MaterialFileInfo
	return mf.WithContext(c).Where(mf.MaterialID.Eq(materialID)).Order(mf.SortOrder, mf.ID).Find()
}

func (s *svcImpl) GetMaterialFile(c *gin.Context, id int64) (*model.MaterialFileInfo, error) {
	mf := query.Use(s.db).MaterialFileInfo
	return mf.WithContext(c).Where(mf.ID.Eq(id)).First()
}

func (s *svcImpl) GetMaterialFileByObjectKey(c *gin.Context, objectKey string) (*model.MaterialFileInfo, error) {
	mf := query.Use(s.db).MaterialFileInfo
	return mf.WithContext(c).Where(mf.ObjectKey.Eq(objectKey)).First()
}

func (s *svcImpl) CreateMaterialFile(c *gin.Context, f *model.MaterialFileInfo) error {
	mf := query.Use(s.db).MaterialFileInfo
	return mf.WithContext(c).Create(f)
}

func (s *svcImpl) UpdateMaterialFileInfo(c *gin.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 || len(fields) == 0 {
		return fmt.Errorf("invalid params")
	}
	mf := query.Use(s.db).MaterialFileInfo
	_, err := mf.WithContext(c).Where(mf.ID.Eq(id)).Updates(fields)
	return err
}

func (s *svcImpl) UpdateMaterialFileSortOrders(c *gin.Context, materialID int64, ids []int64) error {
	if materialID <= 0 || len(ids) == 0 {
		return fmt.Errorf("invalid params")
	}
	q := query.Use(s.db).MaterialFileInfo
	files, err := q.WithContext(c).Where(q.MaterialID.Eq(materialID), q.ID.In(ids...)).Find()
	if err != nil {
		return err
	}
	if len(files) != len(ids) {
		return fmt.Errorf("存在无效文件")
	}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".doc" && ext != ".docx" && ext != ".pdf" {
			return fmt.Errorf("仅支持文档文件排序")
		}
	}
	return s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		for idx, id := range ids {
			if err := tx.Table("material_file_info").
				Where("material_id = ? AND id = ?", materialID, id).
				Update("sort_order", int32(idx+1)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *svcImpl) DeleteMaterialFile(c *gin.Context, id int64) error {
	mf := query.Use(s.db).MaterialFileInfo
	_, err := mf.WithContext(c).Where(mf.ID.Eq(id)).Delete(&model.MaterialFileInfo{})
	return err
}

func (s *svcImpl) DeleteMaterialFileGraph(c *gin.Context, id int64) (*model.MaterialFileInfo, []*model.MaterialImageInfo, error) {
	if c == nil || id <= 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}
	var file model.MaterialFileInfo
	var images []*model.MaterialImageInfo
	err := s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).First(&file).Error; err != nil {
			return err
		}
		if err := tx.Where("material_file_id=?", id).Find(&images).Error; err != nil {
			return fmt.Errorf("查询素材文件图片: %w", err)
		}
		if err := tx.Where("file_id=?", id).Delete(&model.MaterialOcrResult{}).Error; err != nil {
			return fmt.Errorf("删除素材文件OCR: %w", err)
		}
		if err := tx.Where("material_file_id=?", id).Delete(&model.MaterialImageInfo{}).Error; err != nil {
			return fmt.Errorf("删除素材文件图片: %w", err)
		}
		if err := tx.Delete(&file).Error; err != nil {
			return fmt.Errorf("删除素材文件: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &file, images, nil
}

func (s *svcImpl) DeleteMaterialFilesByMaterialID(c *gin.Context, materialID int64) error {
	mf := query.Use(s.db).MaterialFileInfo
	_, err := mf.WithContext(c).Where(mf.MaterialID.Eq(materialID)).Delete(&model.MaterialFileInfo{})
	return err
}

func (s *svcImpl) ListMaterialImages(c *gin.Context, materialID int64) ([]*model.MaterialImageInfo, error) {
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Where(mi.MaterialID.Eq(materialID)).Order(mi.SortOrder, mi.ID).Find()
}

func (s *svcImpl) ListMaterialImagesByUserID(c *gin.Context, userID int64) ([]*model.MaterialImageInfo, error) {
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Where(mi.UserID.Eq(userID)).Order(mi.CreatedAt.Desc(), mi.ID.Desc()).Find()
}

func (s *svcImpl) GetMaxMaterialImageSortOrder(c *gin.Context, userID int64, materialID int64) (int32, error) {
	type Row struct {
		MaxSortOrder int32 `json:"max_sort_order"`
	}
	var row Row
	if err := s.db.WithContext(c).
		Table("material_image_info").
		Select("IFNULL(MAX(sort_order), 0) as max_sort_order").
		Where("user_id = ? AND material_id = ?", userID, materialID).
		Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.MaxSortOrder, nil
}

func (s *svcImpl) ListMaterialImagesByFileID(c *gin.Context, materialFileID int64) ([]*model.MaterialImageInfo, error) {
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Where(mi.MaterialFileID.Eq(materialFileID)).Order(mi.SortOrder, mi.ID).Find()
}

func (s *svcImpl) GetMaterialImage(c *gin.Context, id int64) (*model.MaterialImageInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("context is nil")
	}
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Where(mi.ID.Eq(id)).First()
}

func (s *svcImpl) GetMaterialImageByObjectKey(c *gin.Context, objectKey string) (*model.MaterialImageInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("context is nil")
	}
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Where(mi.ObjectKey.Eq(objectKey)).Or(mi.OriginObjectKey.Eq(objectKey)).First()
}

func (s *svcImpl) CreateMaterialImage(c *gin.Context, img *model.MaterialImageInfo) error {
	mi := query.Use(s.db).MaterialImageInfo
	return mi.WithContext(c).Create(img)
}

func (s *svcImpl) UpdateMaterialImageSortOrders(c *gin.Context, materialID int64, imageIDs []int64) error {
	if materialID <= 0 || len(imageIDs) == 0 {
		return fmt.Errorf("invalid params")
	}
	q := query.Use(s.db).MaterialImageInfo
	imgs, err := q.WithContext(c).Where(q.MaterialID.Eq(materialID), q.ID.In(imageIDs...)).Find()
	if err != nil {
		return err
	}
	if len(imgs) != len(imageIDs) {
		return fmt.Errorf("存在无效图片")
	}
	return s.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		for idx, id := range imageIDs {
			if err := tx.Table("material_image_info").
				Where("material_id = ? AND id = ?", materialID, id).
				Update("sort_order", int32(idx+1)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *svcImpl) UpdateMaterialImageInfo(c *gin.Context, id int64, fields map[string]interface{}) error {
	if id <= 0 || len(fields) == 0 {
		return fmt.Errorf("invalid params")
	}
	if c == nil {
		return fmt.Errorf("context is nil")
	}
	mi := query.Use(s.db).MaterialImageInfo
	_, err := mi.WithContext(c).Where(mi.ID.Eq(id)).Updates(fields)
	return err
}

func (s *svcImpl) DeleteMaterialImagesByMaterialID(c *gin.Context, materialID int64) error {
	mi := query.Use(s.db).MaterialImageInfo
	_, err := mi.WithContext(c).Where(mi.MaterialID.Eq(materialID)).Delete(&model.MaterialImageInfo{})
	return err
}

func (s *svcImpl) DeleteMaterialImagesByFileID(c *gin.Context, materialFileID int64) error {
	mi := query.Use(s.db).MaterialImageInfo
	_, err := mi.WithContext(c).Where(mi.MaterialFileID.Eq(materialFileID)).Delete(&model.MaterialImageInfo{})
	return err
}

func (s *svcImpl) DeleteMaterialImage(c *gin.Context, id int64) error {
	if c == nil {
		return fmt.Errorf("context is nil")
	}
	mi := query.Use(s.db).MaterialImageInfo
	_, err := mi.WithContext(c).Where(mi.ID.Eq(id)).Delete(&model.MaterialImageInfo{})
	return err
}

func (s *svcImpl) GetCompanyName(c *gin.Context, companyID int32) (string, error) {
	if companyID <= 0 {
		return "", nil
	}
	var name string
	err := s.db.WithContext(c).Table("company").Select("name").Where("id = ?", companyID).Scan(&name).Error
	if err != nil {
		return "", err
	}
	return name, nil
}

func (s *svcImpl) GetUserName(c *gin.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", nil
	}
	var result struct {
		Nickname string
		Username string
	}
	err := s.db.WithContext(c).Table("user").Select("nickname, username").Where("user_id = ?", userID).Scan(&result).Error
	if err != nil {
		return "", err
	}
	if result.Nickname != "" {
		return result.Nickname, nil
	}
	return result.Username, nil
}

// ===== MaterialOcrResult CRUD =====

func (s *svcImpl) GetOcrResultByMaterialID(c *gin.Context, materialID int64) ([]*model.MaterialOcrResult, error) {
	m := query.Use(s.db).MaterialOcrResult
	return m.WithContext(c).Where(m.MaterialID.Eq(materialID)).Order(m.ID).Find()
}

func (s *svcImpl) GetOcrResultByID(c *gin.Context, id int64) (*model.MaterialOcrResult, error) {
	m := query.Use(s.db).MaterialOcrResult
	return m.WithContext(c).Where(m.ID.Eq(id)).First()
}

func (s *svcImpl) CreateOcrResult(c *gin.Context, r *model.MaterialOcrResult) error {
	return query.Use(s.db).MaterialOcrResult.WithContext(c).Create(r)
}

func (s *svcImpl) UpdateOcrResult(c *gin.Context, id int64, fields map[string]interface{}) error {
	m := query.Use(s.db).MaterialOcrResult
	_, err := m.WithContext(c).Where(m.ID.Eq(id)).Updates(fields)
	return err
}

func (s *svcImpl) DeleteOcrResultsByMaterialID(c *gin.Context, materialID int64) error {
	m := query.Use(s.db).MaterialOcrResult
	_, err := m.WithContext(c).Where(m.MaterialID.Eq(materialID)).Delete(&model.MaterialOcrResult{})
	return err
}

// ========== 三张独立物料表 CRUD ==========

var typeDisplayNames = map[string]string{
	"material_qualification": "企业资质",
	"material_performance":   "企业业绩",
	"material_template":      "文档模板",
}

var typeCodes = map[string]string{
	"material_qualification": "qualification",
	"material_performance":   "performance",
	"material_template":      "template",
}

func buildMaterialListItem(m MaterialInfoRow, table string) *entity.MaterialListItem {
	tc := typeCodes[table]
	tn := typeDisplayNames[table]
	return &entity.MaterialListItem{
		ID: m.ID, Name: m.Name, Type: tc, TypeName: tn,
		Description: m.Description, CompanyID: m.CompanyID, CompanyName: m.CompanyName,
		UserID: m.UserID, UserName: m.UserName, CreatedTime: m.CreatedAt.Format("2006-1-2 15:04:05"),
	}
}

type MaterialInfoRow struct {
	ID          int64
	Name        string
	Description string
	UserID      int64
	CompanyID   int32
	UserName    string
	CompanyName string
	CreatedAt   time.Time
}

func listMaterialsGeneric(db *gorm.DB, ctx context.Context, table string, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	if param.AuthUserID <= 0 {
		return nil, 0, gorm.ErrRecordNotFound
	}
	var total int64
	alias := "m"
	base := db.WithContext(ctx).Table(table+" AS "+alias).
		Joins("LEFT JOIN user u ON u.user_id = m.user_id").
		Joins("LEFT JOIN company c ON c.id = m.company_id").
		Where("m.user_id = ?", param.AuthUserID)
	if param.Keyword != "" {
		kw := "%" + param.Keyword + "%"
		base = base.Where("(m.name LIKE ? OR m.description LIKE ?)", kw, kw)
	}
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*entity.MaterialListItem{}, 0, nil
	}

	var rows []MaterialInfoRow
	offset := (param.PageNum - 1) * param.PageSize
	if err := base.Select("m.id, m.name, m.description, m.user_id, m.company_id, m.created_at, u.nickname AS user_name, c.name AS company_name").
		Order("m.id DESC").Offset(offset).Limit(param.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	list := make([]*entity.MaterialListItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, buildMaterialListItem(r, table))
	}
	return list, total, nil
}

// ---- Qualification ----
func (s *svcImpl) CreateQualification(c *gin.Context, m *model.MaterialQualification) error {
	return s.db.WithContext(c).Create(m).Error
}
func (s *svcImpl) GetQualification(c *gin.Context, id int64) (*model.MaterialQualification, error) {
	var m model.MaterialQualification
	if err := s.db.WithContext(c).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *svcImpl) GetQualificationForUser(ctx context.Context, userID, id int64) (*model.MaterialQualification, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var material model.MaterialQualification
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&material).Error; err != nil {
		return nil, err
	}
	return &material, nil
}
func (s *svcImpl) ListQualifications(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	return listMaterialsGeneric(s.db, c, "material_qualification", param)
}
func (s *svcImpl) UpdateQualification(c *gin.Context, id int64, fields map[string]interface{}) error {
	return s.db.WithContext(c).Table("material_qualification").Where("id = ?", id).Updates(fields).Error
}
func (s *svcImpl) UpdateQualificationForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error {
	result := s.db.WithContext(ctx).Table("material_qualification").Where("id = ? AND user_id = ?", id, userID).Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *svcImpl) DeleteQualification(c *gin.Context, id int64) error {
	return s.db.WithContext(c).Table("material_qualification").Where("id = ?", id).Delete(&model.MaterialQualification{}).Error
}
func (s *svcImpl) DeleteQualificationForUser(ctx context.Context, userID, id int64) error {
	result := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.MaterialQualification{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---- Performance ----
func (s *svcImpl) CreatePerformance(c *gin.Context, m *model.MaterialPerformance) error {
	return s.db.WithContext(c).Create(m).Error
}
func (s *svcImpl) GetPerformance(c *gin.Context, id int64) (*model.MaterialPerformance, error) {
	var m model.MaterialPerformance
	if err := s.db.WithContext(c).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *svcImpl) GetPerformanceForUser(ctx context.Context, userID, id int64) (*model.MaterialPerformance, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var material model.MaterialPerformance
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&material).Error; err != nil {
		return nil, err
	}
	return &material, nil
}
func (s *svcImpl) ListPerformances(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	return listMaterialsGeneric(s.db, c, "material_performance", param)
}
func (s *svcImpl) UpdatePerformance(c *gin.Context, id int64, fields map[string]interface{}) error {
	return s.db.WithContext(c).Table("material_performance").Where("id = ?", id).Updates(fields).Error
}
func (s *svcImpl) UpdatePerformanceForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error {
	result := s.db.WithContext(ctx).Table("material_performance").Where("id = ? AND user_id = ?", id, userID).Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *svcImpl) DeletePerformance(c *gin.Context, id int64) error {
	return s.db.WithContext(c).Table("material_performance").Where("id = ?", id).Delete(&model.MaterialPerformance{}).Error
}
func (s *svcImpl) DeletePerformanceForUser(ctx context.Context, userID, id int64) error {
	result := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.MaterialPerformance{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---- Template ----
func (s *svcImpl) CreateTemplate(c *gin.Context, m *model.MaterialTemplate) error {
	return s.db.WithContext(c).Create(m).Error
}
func (s *svcImpl) GetTemplate(c *gin.Context, id int64) (*model.MaterialTemplate, error) {
	var m model.MaterialTemplate
	if err := s.db.WithContext(c).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *svcImpl) GetTemplateForUser(ctx context.Context, userID, id int64) (*model.MaterialTemplate, error) {
	if userID <= 0 || id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var material model.MaterialTemplate
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&material).Error; err != nil {
		return nil, err
	}
	return &material, nil
}
func (s *svcImpl) ListTemplates(c *gin.Context, param ListParam) ([]*entity.MaterialListItem, int64, error) {
	return listMaterialsGeneric(s.db, c, "material_template", param)
}
func (s *svcImpl) UpdateTemplate(c *gin.Context, id int64, fields map[string]interface{}) error {
	return s.db.WithContext(c).Table("material_template").Where("id = ?", id).Updates(fields).Error
}
func (s *svcImpl) UpdateTemplateForUser(ctx context.Context, userID, id int64, fields map[string]interface{}) error {
	result := s.db.WithContext(ctx).Table("material_template").Where("id = ? AND user_id = ?", id, userID).Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *svcImpl) DeleteTemplate(c *gin.Context, id int64) error {
	return s.db.WithContext(c).Table("material_template").Where("id = ?", id).Delete(&model.MaterialTemplate{}).Error
}
func (s *svcImpl) DeleteTemplateForUser(ctx context.Context, userID, id int64) error {
	result := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.MaterialTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
