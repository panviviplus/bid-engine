package material

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/converter"
	repoMaterial "bid-engine/pkg/repo/material"
	"bid-engine/pkg/utils"
)

var materialTypeOptions = []*entity.MaterialTypeOption{
	{Type: "template", Name: "模板素材"},
	{Type: "qualification", Name: "企业资质"},
	{Type: "performance", Name: "企业业绩"},
}

func (s *svcImpl) Types(c *gin.Context) {
	handler.SendOKResp(c, materialTypeOptions)
}

func (s *svcImpl) Companies(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	items, err := s.repo.ListCompaniesForUser(c.Request.Context(), userID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	handler.SendOKResp(c, items)
}

func (s *svcImpl) Users(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	items, err := s.repo.ListUsersForUser(c.Request.Context(), userID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	handler.SendOKResp(c, items)
}

func (s *svcImpl) List(c *gin.Context) {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialListReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	param := repoMaterial.ListParam{
		PageNum:    req.PageNum,
		PageSize:   req.PageSize,
		Keyword:    req.Keyword,
		Type:       req.Type,
		AuthUserID: entity.GetUserIDFromCtx(c),
	}
	list, total, err := s.repo.ListMaterials(c, param)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	for _, it := range list {
		it.TypeName = materialTypeName(it.Type)
	}
	handler.SendOKResp(c, &entity.MaterialListResp{List: list, Total: total})
}

// ========== 类型专属 List 方法（读写各自独立表） ==========

func (s *svcImpl) ListQualifications(c *gin.Context) {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialListReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	param := repoMaterial.ListParam{
		PageNum: req.PageNum, PageSize: req.PageSize, Keyword: req.Keyword,
		AuthUserID: entity.GetUserIDFromCtx(c),
	}
	list, total, err := s.repo.ListQualifications(c, param)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	handler.SendOKResp(c, &entity.MaterialListResp{List: list, Total: total})
}

func (s *svcImpl) ListPerformances(c *gin.Context) {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialListReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	param := repoMaterial.ListParam{
		PageNum: req.PageNum, PageSize: req.PageSize, Keyword: req.Keyword,
		AuthUserID: entity.GetUserIDFromCtx(c),
	}
	list, total, err := s.repo.ListPerformances(c, param)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	handler.SendOKResp(c, &entity.MaterialListResp{List: list, Total: total})
}

func (s *svcImpl) ListTemplates(c *gin.Context) {
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialListReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	param := repoMaterial.ListParam{
		PageNum: req.PageNum, PageSize: req.PageSize, Keyword: req.Keyword,
		AuthUserID: entity.GetUserIDFromCtx(c),
	}
	list, total, err := s.repo.ListTemplates(c, param)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	handler.SendOKResp(c, &entity.MaterialListResp{List: list, Total: total})
}

// ========== 类型专属 CRUD ==========

// ========== 企业资质 CRUD（表: material_qualification） ==========

func (s *svcImpl) AddQualification(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialAddReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	files, err := getMultipartFiles(c, 5)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if len(files) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择文件", nil)
		return
	}
	if err := validateMixedFiles(files); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info := &model.MaterialQualification{
		Name: req.Name, Description: req.Description, UserID: u.UserID, CompanyID: u.CompanyID,
	}
	if err := s.repo.CreateQualification(c, info); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "新增失败", nil)
		return
	}
	s.saveUploadedFiles(c, info.ID, u.UserID, u.CompanyID, files)
	s.triggerPostCreateAsync(c, info.ID, "qualification", req)
	handler.SendOKResp(c, map[string]interface{}{"id": info.ID})
}

func (s *svcImpl) DetailQualification(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info, err := s.repo.GetQualificationForUser(c.Request.Context(), u.UserID, uri.ID)
	if err != nil || info == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	s.renderMaterialDetail(c, info.ID, info.Name, "qualification", info.Description, info.UserID, info.CompanyID, info.CreatedAt)
}

func (s *svcImpl) UpdateQualification(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetQualificationForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	var req entity.MaterialUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if req.Name = strings.TrimSpace(req.Name); entity.ValidateName(req.Name) != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, entity.ValidateName(req.Name).Error(), nil)
		return
	}
	if len([]rune(req.Description)) > 1000 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "description 超过最大长度 1000", nil)
		return
	}
	if err := s.repo.UpdateQualificationForUser(c.Request.Context(), userID, uri.ID, map[string]interface{}{
		"name": req.Name, "description": req.Description,
	}); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) DeleteQualification(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetQualificationForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	// 先取出文件 id，用于随后清理预览转换缓存（素材删除后文件行会级联消失）
	fileIDs := s.listMaterialFileIDs(c, uri.ID)
	result, err := s.repo.DeleteMaterialGraphForUser(c.Request.Context(), userID, uri.ID, "qualification")
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	s.deleteMaterialObjectKeys(c, result.ObjectKeys)
	s.deleteMaterialPreviewCaches(c, uri.ID, fileIDs)
	handler.SendOKResp(c, nil)
}

// ========== 企业业绩 CRUD（表: material_performance） ==========

func (s *svcImpl) AddPerformance(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialAddReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	files, err := getMultipartFiles(c, 5)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if len(files) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择文件", nil)
		return
	}
	if err := validateMixedFiles(files); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info := &model.MaterialPerformance{
		Name: req.Name, Description: req.Description, UserID: u.UserID, CompanyID: u.CompanyID,
	}
	if err := s.repo.CreatePerformance(c, info); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "新增失败", nil)
		return
	}
	s.saveUploadedFiles(c, info.ID, u.UserID, u.CompanyID, files)
	s.triggerPostCreateAsync(c, info.ID, "performance", req)
	handler.SendOKResp(c, map[string]interface{}{"id": info.ID})
}

func (s *svcImpl) DetailPerformance(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info, err := s.repo.GetPerformanceForUser(c.Request.Context(), u.UserID, uri.ID)
	if err != nil || info == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	s.renderMaterialDetail(c, info.ID, info.Name, "performance", info.Description, info.UserID, info.CompanyID, info.CreatedAt)
}

func (s *svcImpl) UpdatePerformance(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetPerformanceForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	var req entity.MaterialUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if req.Name = strings.TrimSpace(req.Name); entity.ValidateName(req.Name) != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, entity.ValidateName(req.Name).Error(), nil)
		return
	}
	if len([]rune(req.Description)) > 1000 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "description 超过最大长度 1000", nil)
		return
	}
	if err := s.repo.UpdatePerformanceForUser(c.Request.Context(), userID, uri.ID, map[string]interface{}{
		"name": req.Name, "description": req.Description,
	}); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) DeletePerformance(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetPerformanceForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	// 先取出文件 id，用于随后清理预览转换缓存（素材删除后文件行会级联消失）
	fileIDs := s.listMaterialFileIDs(c, uri.ID)
	result, err := s.repo.DeleteMaterialGraphForUser(c.Request.Context(), userID, uri.ID, "performance")
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	s.deleteMaterialObjectKeys(c, result.ObjectKeys)
	s.deleteMaterialPreviewCaches(c, uri.ID, fileIDs)
	handler.SendOKResp(c, nil)
}

// ========== 文档模板 CRUD（表: material_template） ==========

func (s *svcImpl) AddTemplate(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialAddReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	files, err := getMultipartFiles(c, 5)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if len(files) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择文件", nil)
		return
	}
	if err := validateMixedFiles(files); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info := &model.MaterialTemplate{
		Name: req.Name, Description: req.Description, UserID: u.UserID, CompanyID: u.CompanyID,
	}
	if err := s.repo.CreateTemplate(c, info); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "新增失败", nil)
		return
	}
	s.saveUploadedFiles(c, info.ID, u.UserID, u.CompanyID, files)
	handler.SendOKResp(c, map[string]interface{}{"id": info.ID})
}

func (s *svcImpl) DetailTemplate(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	info, err := s.repo.GetTemplateForUser(c.Request.Context(), u.UserID, uri.ID)
	if err != nil || info == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	s.renderMaterialDetail(c, info.ID, info.Name, "template", info.Description, info.UserID, info.CompanyID, info.CreatedAt)
}

func (s *svcImpl) UpdateTemplate(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetTemplateForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	var req entity.MaterialUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if req.Name = strings.TrimSpace(req.Name); entity.ValidateName(req.Name) != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, entity.ValidateName(req.Name).Error(), nil)
		return
	}
	if len([]rune(req.Description)) > 1000 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "description 超过最大长度 1000", nil)
		return
	}
	if err := s.repo.UpdateTemplateForUser(c.Request.Context(), userID, uri.ID, map[string]interface{}{
		"name": req.Name, "description": req.Description,
	}); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) DeleteTemplate(c *gin.Context) {
	userID := entity.GetUserIDFromCtx(c)
	var uri entity.MaterialIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if _, err := s.repo.GetTemplateForUser(c.Request.Context(), userID, uri.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	// 先取出文件 id，用于随后清理预览转换缓存（素材删除后文件行会级联消失）
	fileIDs := s.listMaterialFileIDs(c, uri.ID)
	result, err := s.repo.DeleteMaterialGraphForUser(c.Request.Context(), userID, uri.ID, "template")
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	s.deleteMaterialObjectKeys(c, result.ObjectKeys)
	s.deleteMaterialPreviewCaches(c, uri.ID, fileIDs)
	handler.SendOKResp(c, nil)
}

// materialPreviewCacheKey 预览转换缓存键，与 PreviewFile 必须保持一致
func materialPreviewCacheKey(materialID, fileID int64) string {
	return fmt.Sprintf("material/%d/preview/%d.pdf", materialID, fileID)
}

// listMaterialFileIDs 取素材下的文档文件 id（失败时返回空集，不影响主流程）
func (s *svcImpl) listMaterialFileIDs(c *gin.Context, materialID int64) []int64 {
	files, err := s.repo.ListMaterialFiles(c, materialID)
	if err != nil {
		return nil
	}
	ids := make([]int64, 0, len(files))
	for _, f := range files {
		if f != nil && f.ID > 0 {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

// deleteMaterialPreviewCaches 清理预览转换缓存（对象不存在时忽略）
func (s *svcImpl) deleteMaterialPreviewCaches(c *gin.Context, materialID int64, fileIDs []int64) {
	if materialID <= 0 || len(fileIDs) == 0 {
		return
	}
	logger := s.logger.With(entity.Ctx(c)...)
	for _, id := range fileIDs {
		if id <= 0 {
			continue
		}
		if err := s.oss.Delete(entity.ConvertContext(c), materialPreviewCacheKey(materialID, id)); err != nil {
			logger.Warnw("删除预览缓存失败", "material_id", materialID, "file_id", id, "err", err)
		}
	}
}

func (s *svcImpl) deleteMaterialObjectKeys(c *gin.Context, objectKeys []string) {
	logger := s.logger.With(entity.Ctx(c)...)
	for _, objectKey := range objectKeys {
		if err := s.oss.Delete(entity.ConvertContext(c), objectKey); err != nil {
			logger.Warnw("删除素材对象存储失败", "object", objectKey, "err", err)
		}
	}
}

func (s *svcImpl) Gallery(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	imgs, err := s.repo.ListMaterialImagesByUserID(c, u.UserID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}

	grouped := make(map[int64][]*entity.MaterialImageItem)
	materialIDSet := make(map[int64]struct{})
	for _, im := range imgs {
		if im == nil {
			continue
		}
		item := &entity.MaterialImageItem{
			ID:              im.ID,
			MaterialID:      im.MaterialID,
			MaterialFileID:  im.MaterialFileID,
			Name:            im.Name,
			Size:            im.Size,
			OriginObjectKey: im.OriginObjectKey,
			ObjectKey:       im.ObjectKey,
			SortOrder:       im.SortOrder,
			EditFlow:        im.EditFlow,
			CreatedTime:     im.CreatedAt.Unix(),
		}
		grouped[im.MaterialID] = append(grouped[im.MaterialID], item)
		if im.MaterialID > 0 {
			materialIDSet[im.MaterialID] = struct{}{}
		}
	}

	materialIDs := make([]int64, 0, len(materialIDSet))
	for id := range materialIDSet {
		materialIDs = append(materialIDs, id)
	}
	infos, err := s.repo.LookupMaterialsForUser(c.Request.Context(), u.UserID, materialIDs)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	nameMap := make(map[int64]string, len(infos))
	for _, it := range infos {
		if it == nil {
			continue
		}
		nameMap[it.ID] = it.Name
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i] == nil && infos[j] == nil {
			return false
		}
		if infos[i] == nil {
			return false
		}
		if infos[j] == nil {
			return true
		}
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})

	groups := make([]*entity.MaterialGalleryGroup, 0, len(materialIDs)+1)
	groups = append(groups, &entity.MaterialGalleryGroup{
		MaterialID:   0,
		MaterialName: "默认",
		Images:       grouped[0],
	})
	for _, it := range infos {
		if it == nil || it.ID <= 0 {
			continue
		}
		groups = append(groups, &entity.MaterialGalleryGroup{
			MaterialID:   it.ID,
			MaterialName: nameMap[it.ID],
			Images:       grouped[it.ID],
		})
	}

	handler.SendOKResp(c, &entity.MaterialGalleryResp{Groups: groups})
}

func (s *svcImpl) GalleryUpload(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	files, err := getMultipartFiles(c, 5)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if len(files) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择文件", nil)
		return
	}
	if err = validateGalleryImageFiles(files); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if err = os.MkdirAll("./tmp", 0o777); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}

	maxSort, err := s.repo.GetMaxMaterialImageSortOrder(c, u.UserID, 0)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	nextSort := maxSort + 1

	keepOriginalName := strings.EqualFold(strings.TrimSpace(c.PostForm("keepOriginalName")), "true")

	ids := make([]int64, 0, len(files))
	for _, fh := range files {
		id, _, err2 := s.saveOneGalleryImageAndRecord(c, u.UserID, u.CompanyID, keepOriginalName, fh, nextSort)
		if err2 != nil {
			logger.Warnw("保存图库图片失败", "err", err2)
			handler.SendNormalResp(c, entity.ErrCodeInternal, err2.Error(), nil)
			return
		}
		ids = append(ids, id)
		nextSort++
	}
	rid := entity.GetRequestIDForGo(c)
	ac := c.Copy()
	if rid != "" {
		entity.SetRequestID(ac, rid)
	}
	go s.asyncFillImageNameDescForUserGallery(ac, u.UserID)
	handler.SendOKResp(c, map[string]interface{}{"ids": ids, "count": len(ids)})
}

func (s *svcImpl) GalleryDelete(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialGalleryDeleteReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	deleted := make([]int64, 0, len(req.ImageIDs))
	ossKeys := make(map[string]struct{})

	for _, id := range req.ImageIDs {
		img, err := s.repo.GetMaterialImage(c, id)
		if err != nil || img == nil {
			continue
		}
		if img.MaterialID == 0 {
			if img.UserID != u.UserID {
				handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
				return
			}
		} else {
			summary, err2 := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, img.MaterialID)
			if err2 != nil || summary == nil {
				continue
			}
		}

		if k := strings.TrimSpace(img.ObjectKey); k != "" {
			ossKeys[k] = struct{}{}
		}
		if k := strings.TrimSpace(img.OriginObjectKey); k != "" {
			ossKeys[k] = struct{}{}
		}

		if err := s.repo.DeleteMaterialImage(c, img.ID); err != nil {
			handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
			return
		}
		deleted = append(deleted, img.ID)
	}

	for k := range ossKeys {
		if err := s.oss.Delete(entity.ConvertContext(c), k); err != nil {
			logger.Warnw("删除对象存储失败", "object", k, "err", err)
		}
	}

	handler.SendOKResp(c, map[string]interface{}{"deleted_ids": deleted, "count": len(deleted)})
}

func (s *svcImpl) GalleryMove(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialGalleryMoveReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	img, err := s.repo.GetMaterialImage(c, req.ImageID)
	if err != nil || img == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
		return
	}
	if img.MaterialID == 0 {
		if img.UserID != u.UserID {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
			return
		}
	} else {
		summary, err2 := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, img.MaterialID)
		if err2 != nil || summary == nil {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
			return
		}
	}

	if req.ToMaterialID > 0 {
		target, err2 := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, req.ToMaterialID)
		if err2 != nil || target == nil {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "目标素材不存在", nil)
			return
		}
	}

	if err := s.repo.UpdateMaterialImageInfo(c, req.ImageID, map[string]interface{}{"material_id": req.ToMaterialID}); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) AddFile(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialFileAddUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, uri.ID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	var req entity.MaterialFileAddReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	files, err := getMultipartFiles(c, 5)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if len(files) == 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择文件", nil)
		return
	}
	if err := validateMixedFiles(files); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if err2 := os.MkdirAll("./tmp", 0o777); err2 != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}

	existingDocs, _ := s.repo.ListMaterialFiles(c, uri.ID)
	maxDocSort := int32(0)
	for _, f := range existingDocs {
		if f == nil {
			continue
		}
		if fileGroupByExt(filepath.Ext(f.Name)) == "doc" && f.SortOrder > maxDocSort {
			maxDocSort = f.SortOrder
		}
	}
	nextDocSort := maxDocSort + 1

	existingImages, _ := s.repo.ListMaterialImages(c, uri.ID)
	maxImageSort := int32(0)
	for _, im := range existingImages {
		if im != nil && im.SortOrder > maxImageSort {
			maxImageSort = im.SortOrder
		}
	}
	nextImageSort := maxImageSort + 1

	imageFiles, docFiles := splitFiles(files)

	for _, fh := range imageFiles {
		_, _, err := s.saveOneImageAndRecords(c, uri.ID, summary.UserID, summary.CompanyID, fh, nextImageSort)
		if err != nil {
			logger.Warnw("保存素材图片失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, err.Error(), nil)
			return
		}
		nextImageSort++
	}
	var lastDocFileID int64
	for _, fh := range docFiles {
		fileID, objKey, err := s.saveOneDocAndRecords(c, uri.ID, fh, nextDocSort)
		if err != nil {
			logger.Warnw("保存素材文档失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, err.Error(), nil)
			return
		}
		lastDocFileID = fileID
		nextDocSort++
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if req.AutoAnalysis && (ext == ".doc" || ext == ".docx") {
			updatedSort, err := s.extractImagesFromDocAndSave(c, uri.ID, summary.UserID, summary.CompanyID, fh.Filename, objKey, nextImageSort)
			if err != nil {
				logger.Warnw("抽取文档图片失败", "err", err)
				handler.SendNormalResp(c, entity.ErrCodeInternal, err.Error(), nil)
				return
			}
			nextImageSort = updatedSort
		}
	}

	if req.AutoAnalysis {
		rid := entity.GetRequestIDForGo(c)
		ac := c.Copy()
		if rid != "" {
			entity.SetRequestID(ac, rid)
		}
		go s.asyncFillImageNameDescByMaterialID(ac, uri.ID)
	}
	if summary.Type == "qualification" || summary.Type == "performance" {
		rid := entity.GetRequestIDForGo(c)
		ac := c.Copy()
		if rid != "" {
			entity.SetRequestID(ac, rid)
		}
		go s.asyncRunOcr(ac, uri.ID)
	}
	handler.SendOKResp(c, map[string]interface{}{"file_id": lastDocFileID})
}

func (s *svcImpl) DeleteFile(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialFileIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	f, err := s.repo.GetMaterialFile(c, uri.ID)
	if err != nil || f == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "文件不存在", nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, f.MaterialID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	deletedFile, deletedImages, err := s.repo.DeleteMaterialFileGraph(c, f.ID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	s.deleteMaterialPreviewCaches(c, f.MaterialID, []int64{f.ID})
	keys := map[string]struct{}{}
	if key := strings.TrimSpace(deletedFile.ObjectKey); key != "" {
		keys[key] = struct{}{}
	}
	for _, image := range deletedImages {
		if key := strings.TrimSpace(image.ObjectKey); key != "" {
			keys[key] = struct{}{}
		}
		if key := strings.TrimSpace(image.OriginObjectKey); key != "" {
			keys[key] = struct{}{}
		}
	}
	for key := range keys {
		if err := s.oss.Delete(entity.ConvertContext(c), key); err != nil {
			logger.Warnw("删除对象存储失败", "object", key, "err", err)
		}
	}

	handler.SendOKResp(c, nil)
}

func (s *svcImpl) DeleteImage(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialImageIDUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	img, err := s.repo.GetMaterialImage(c, uri.ID)
	if err != nil || img == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, img.MaterialID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	keys := map[string]struct{}{}
	if k := strings.TrimSpace(img.ObjectKey); k != "" {
		keys[k] = struct{}{}
	}
	if k := strings.TrimSpace(img.OriginObjectKey); k != "" {
		keys[k] = struct{}{}
	}
	if err := s.repo.DeleteMaterialImage(c, img.ID); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	for k := range keys {
		if err := s.oss.Delete(entity.ConvertContext(c), k); err != nil {
			logger.Warnw("删除对象存储失败", "object", k, "err", err)
		}
	}

	handler.SendOKResp(c, nil)
}

func (s *svcImpl) SortImageFiles(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialSortOrderReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, req.MaterialID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	if err := s.repo.UpdateMaterialImageSortOrders(c, req.MaterialID, req.FileIDs); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, err.Error(), nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) SortDocFiles(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var req entity.MaterialSortOrderReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, req.MaterialID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	if err := s.repo.UpdateMaterialFileSortOrders(c, req.MaterialID, req.FileIDs); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, err.Error(), nil)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *svcImpl) DownloadFile(c *gin.Context) {
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	objectKey := strings.TrimPrefix(c.Param("objectKey"), "/")
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "objectKey 参数错误", nil)
		return
	}
	var fileName string
	var materialID int64
	var imgRec *model.MaterialImageInfo
	f, err := s.repo.GetMaterialFileByObjectKey(c, objectKey)
	if err == nil && f != nil {
		fileName = f.Name
		materialID = f.MaterialID
	} else {
		img, err2 := s.repo.GetMaterialImageByObjectKey(c, objectKey)
		if err2 != nil || img == nil {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "文件不存在", nil)
			return
		}
		imgRec = img
		fileName = img.Name
		materialID = img.MaterialID
	}
	if materialID == 0 && imgRec != nil {
		if imgRec.UserID != u.UserID {
			handler.SendNormalResp(c, entity.ErrCodeNotFound, "文件不存在", nil)
			return
		}
		rc, err := s.oss.Open(entity.ConvertContext(c), objectKey)
		if err != nil {
			handler.SendNormalResp(c, entity.ErrCodeS3Read, "下载失败", nil)
			return
		}
		defer rc.Close()

		ext := strings.ToLower(filepath.Ext(fileName))
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Header("Content-Type", mimeType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", urlQueryEscape(fileName)))
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, rc)
		return
	}
	info, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, materialID)
	if err != nil || info == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}
	rc, err := s.oss.Open(entity.ConvertContext(c), objectKey)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeS3Read, "下载失败", nil)
		return
	}
	defer rc.Close()

	ext := strings.ToLower(filepath.Ext(fileName))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", urlQueryEscape(fileName)))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

func (s *svcImpl) PreviewFile(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var uri entity.MaterialFilePreviewUri
	if err := uri.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	f, err := s.repo.GetMaterialFile(c, uri.ID)
	if err != nil || f == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "文件不存在", nil)
		return
	}
	info, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, f.MaterialID)
	if err != nil || info == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}

	if err := os.MkdirAll("./tmp", 0o777); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}

	ext := strings.ToLower(filepath.Ext(f.Name))
	base := strings.TrimSuffix(filepath.Base(f.Name), ext)
	isOfficeDoc := ext == ".doc" || ext == ".docx"
	// 转换结果缓存键：素材文件不可变（重传会产生新 id），因此 file_id → 同一个 PDF 是稳定映射
	cacheKey := materialPreviewCacheKey(f.MaterialID, f.ID)

	// 1) 命中缓存直接回源，避免每次预览都跑一遍 LibreOffice
	if isOfficeDoc {
		cachedPath := filepath.Join("./tmp", fmt.Sprintf("material-preview-cache-%d-%d.pdf", f.MaterialID, f.ID))
		if err := s.oss.Get(entity.ConvertContext(c), cacheKey, cachedPath); err == nil {
			if st, statErr := os.Stat(cachedPath); statErr == nil && st.Size() > 0 {
				defer func() { _ = os.Remove(cachedPath) }()
				serveMaterialPreviewPdf(c, cachedPath, base, true)
				return
			}
			_ = os.Remove(cachedPath)
		}
	}

	localPath := filepath.Join("./tmp", fmt.Sprintf("material-preview-%d-%s%s", time.Now().UnixNano(), base, ext))
	defer func() { _ = os.Remove(localPath) }()

	if err := s.oss.Get(entity.ConvertContext(c), f.ObjectKey, localPath); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeS3Read, "预览失败", nil)
		return
	}

	pdfPath := localPath
	if isOfficeDoc {
		p, err := converter.GetInstance().ConvertToPDF(entity.ConvertContext(c), localPath)
		if err != nil {
			logger.Warnw("Office 转 PDF 失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, "预览失败", nil)
			return
		}
		pdfPath = p
		defer func() {
			if pdfPath != "" && pdfPath != localPath {
				_ = os.Remove(pdfPath)
			}
		}()
		// 2) 回写缓存：失败只记日志，不影响本次预览
		if err := s.oss.Put(entity.ConvertContext(c), cacheKey, pdfPath); err != nil {
			logger.Warnw("预览缓存写入失败", "key", cacheKey, "err", err)
		}
	}
	if strings.ToLower(filepath.Ext(pdfPath)) != ".pdf" {
		handler.SendNormalResp(c, entity.ErrCodeFileNotSupport, "不支持预览该文件类型", nil)
		return
	}

	serveMaterialPreviewPdf(c, pdfPath, base, false)
}

// serveMaterialPreviewPdf 以 PDF 流回源预览结果。
// fromCache=true 表示内容来自转换缓存，可让浏览器/pdf.js 复用，避免分段请求触发重复转换。
func serveMaterialPreviewPdf(c *gin.Context, pdfPath, base string, fromCache bool) {
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", urlQueryEscape(base+".pdf")))
	if fromCache {
		c.Header("Cache-Control", "private, max-age=3600")
	}
	c.File(pdfPath)
}

func (s *svcImpl) EditImage(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	u := entity.GetUserFromCtx(c)
	if u == nil || u.UserID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	if err := os.MkdirAll("./tmp", 0o777); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "file 不能为空", nil)
		return
	}
	materialID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("materialId")), 10, 64)
	if err != nil || materialID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "materialId 参数错误", nil)
		return
	}
	imageID, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("imageId")), 10, 64)
	if err != nil || imageID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "imageId 参数错误", nil)
		return
	}
	opType64, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("opType")), 10, 64)
	if err != nil || (opType64 != 0 && opType64 != 1) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "opType 参数错误", nil)
		return
	}
	editFlowStr := c.PostForm("editFlow")
	s.handleImageEdit(c, logger, u, materialID, imageID, int(opType64), fh, editFlowStr)
}

func (s *svcImpl) handleImageEdit(
	c *gin.Context,
	logger *zap.SugaredLogger,
	u *model.User,
	materialID int64,
	imageID int64,
	opType int,
	fh *multipart.FileHeader,
	editFlowStr string,
) {
	if err := os.MkdirAll("./tmp", 0o777); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}
	imgRec, err := s.repo.GetMaterialImage(c, imageID)
	if err != nil || imgRec == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "图片不存在", nil)
		return
	}
	if materialID <= 0 {
		materialID = imgRec.MaterialID
	}
	if imgRec.MaterialID != materialID {
		handler.SendNormalResp(c, entity.ErrCodeParam, "参数错误", nil)
		return
	}
	summary, err := s.repo.LookupMaterialForUser(c.Request.Context(), u.UserID, materialID)
	if err != nil || summary == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "素材不存在", nil)
		return
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == "" {
		ext = ".png"
	}
	localPath := filepath.Join("./tmp", fmt.Sprintf("material-image-edit-%d-%d%s", imageID, time.Now().UnixNano(), ext))
	defer func() { _ = os.Remove(localPath) }()
	if err := c.SaveUploadedFile(fh, localPath); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeInternal, "保存文件失败", nil)
		return
	}

	objKey := fmt.Sprintf("material/%d/images/%s/%d-%d%s", materialID, time.Now().Format("20060102"), time.Now().UnixNano(), imageID, ext)
	if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeOSSWrite, "上传失败", nil)
		return
	}
	if err := s.repo.UpdateMaterialImageInfo(c, imageID, map[string]interface{}{"object_key": objKey, "edit_flow": editFlowStr}); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "保存失败", nil)
		return
	}
	fileRec, _ := s.repo.GetMaterialFile(c, imgRec.MaterialFileID)
	if fileRec != nil && fileGroupByExt(filepath.Ext(fileRec.Name)) == "image" {
		_ = s.repo.UpdateMaterialFileInfo(c, fileRec.ID, map[string]interface{}{"object_key": objKey, "size": fh.Size})
	}

	if opType == 1 {
		handler.SendOKResp(c, nil)
		return
	}

	// 解析新编辑器的操作流
	baseFlow, err := entity.ParseNewEditorImageFlow(editFlowStr)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "editFlow 参数错误", nil)
		return
	}
	if baseFlow.Image.Width <= 0 || baseFlow.Image.Height <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "editFlow 缺少基准尺寸", nil)
		return
	}

	// 查询该素材下的所有图片
	allImages, err := s.repo.ListMaterialImages(c, materialID)
	if err != nil {
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}

	// 准备批量处理
	tempDir := filepath.Join("./tmp", fmt.Sprintf("batch-material-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		logger.Errorw("创建临时目录失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "创建临时目录失败", nil)
		return
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// 构建下载任务列表（排除当前编辑的图片）
	type downloadTask struct {
		index int
		im    *model.MaterialImageInfo
	}
	var downloadTasks []downloadTask
	for _, im := range allImages {
		if im == nil || im.ID == imageID {
			continue
		}
		originKey := strings.TrimSpace(im.OriginObjectKey)
		if originKey == "" {
			continue
		}
		downloadTasks = append(downloadTasks, downloadTask{
			index: len(downloadTasks),
			im:    im,
		})
	}

	if len(downloadTasks) == 0 {
		handler.SendOKResp(c, nil)
		return
	}

	// 构建图片ID到图片信息的映射
	imageTaskMap := make(map[int64]*model.MaterialImageInfo)
	for _, task := range downloadTasks {
		imageTaskMap[task.im.ID] = task.im
	}

	// 并发下载原图
	downloadResults := utils.ExecuteConcurrent(len(downloadTasks), 10, func(taskIndex int) (interface{}, error) {
		task := downloadTasks[taskIndex]
		originExt := strings.ToLower(filepath.Ext(task.im.OriginObjectKey))
		if originExt == "" {
			originExt = ".png"
		}
		tempInputPath := filepath.Join(tempDir, fmt.Sprintf("input-%d%s", task.im.ID, originExt))
		downloadKey := task.im.OriginObjectKey

		err := s.oss.Get(entity.ConvertContext(c), downloadKey, tempInputPath)
		if err != nil {
			return task.im.ID, fmt.Errorf("下载原图失败: %w", err)
		}

		return task.im.ID, nil
	})

	// 收集成功的下载结果
	var imageTasks []utils.ImageTask
	var downloadFailedIds []int64
	var downloadSuccessCount int

	for _, result := range downloadResults {
		imageID, ok := result.Data.(int64)
		if !ok || imageID <= 0 {
			continue
		}

		im := imageTaskMap[imageID]
		if im == nil {
			continue
		}

		if result.Error != nil {
			downloadFailedIds = append(downloadFailedIds, imageID)
			logger.Errorw("下载原图失败", "err", result.Error, "image_id", imageID)
			continue
		}

		originExt := strings.ToLower(filepath.Ext(im.OriginObjectKey))
		if originExt == "" {
			originExt = ".png"
		}
		tempInputPath := filepath.Join(tempDir, fmt.Sprintf("input-%d%s", im.ID, originExt))
		imageTasks = append(imageTasks, utils.ImageTask{
			ID:        strconv.FormatInt(im.ID, 10),
			ImagePath: tempInputPath,
		})
		downloadSuccessCount++
	}

	if len(imageTasks) == 0 {
		logger.Warnw("所有图片下载失败", "totalCount", len(downloadTasks))
		handler.SendOKResp(c, map[string]interface{}{
			"total":   len(downloadTasks),
			"success": 0,
			"fail":    len(downloadTasks),
		})
		return
	}

	// 批量应用图片操作（等比缩放）
	batchResult, err := utils.ApplyImageOperationsV2(imageTasks, baseFlow, tempDir, logger)
	if err != nil {
		logger.Errorw("批量处理图片失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "批量处理图片失败", nil)
		return
	}

	// 收集成功的结果
	var successResults []utils.ImageResult
	for _, result := range batchResult.Results {
		if result.Success {
			successResults = append(successResults, result)
		}
	}

	// 并发上传和处理结果
	type uploadInfo struct {
		imageID      int64
		updateFields map[string]interface{}
		targetObjKey string
		im           *model.MaterialImageInfo
	}

	uploadResults := utils.ExecuteConcurrent(len(successResults), 10, func(taskIndex int) (interface{}, error) {
		result := successResults[taskIndex]
		imageID, _ := strconv.ParseInt(result.ID, 10, 64)
		im := imageTaskMap[imageID]
		if im == nil {
			return nil, fmt.Errorf("找不到图片记录: %s", result.ID)
		}

		// 计算每张图片的缩放比例，并生成对应的 editFlow
		var perImageFlow string
		if result.OutputPath != "" {
			if tgtImg, oerr := imaging.Open(result.OutputPath); oerr == nil {
				tw := float64(tgtImg.Bounds().Dx())
				th := float64(tgtImg.Bounds().Dy())
				fw := baseFlow.Image.Width
				fh := baseFlow.Image.Height
				if fw <= 0 || fh <= 0 {
					fw, fh = tw, th
				}
				kx := tw / fw
				ky := th / fh
				k := kx
				if ky < k {
					k = ky
				}
				m := map[string]interface{}{
					"image": map[string]interface{}{
						"width":  tw,
						"height": th,
					},
					"texts":  make([]map[string]interface{}, 0, len(baseFlow.Texts)),
					"shapes": make([]map[string]interface{}, 0, len(baseFlow.Shapes)),
				}
				for _, t := range baseFlow.Texts {
					m["texts"] = append(m["texts"].([]map[string]interface{}), map[string]interface{}{
						"id":       t.ID,
						"text":     t.Text,
						"x":        t.X * k,
						"y":        t.Y * k,
						"color":    t.Color,
						"fontSize": t.FontSize * k,
					})
				}
				for _, sItem := range baseFlow.Shapes {
					typ := strings.ToLower(sItem.Type)
					if typ == "mosaic" {
						ms := int(math.Round(float64(sItem.MosaicSize) * k))
						if ms < 1 {
							ms = 1
						}
						m["shapes"] = append(m["shapes"].([]map[string]interface{}), map[string]interface{}{
							"id":         sItem.ID,
							"type":       "mosaic",
							"x":          sItem.X * k,
							"y":          sItem.Y * k,
							"w":          sItem.W * k,
							"h":          sItem.H * k,
							"mosaicSize": ms,
						})
					} else {
						fill := sItem.Fill
						m["shapes"] = append(m["shapes"].([]map[string]interface{}), map[string]interface{}{
							"id":        sItem.ID,
							"type":      typ,
							"x":         sItem.X * k,
							"y":         sItem.Y * k,
							"w":         sItem.W * k,
							"h":         sItem.H * k,
							"color":     sItem.Color,
							"lineWidth": sItem.LineWidth * k,
							"lineDash":  sItem.LineDash,
							"fill":      fill,
						})
					}
				}
				if jb, merr := json.Marshal(m); merr == nil {
					perImageFlow = string(jb)
				}
			}
		}
		if strings.TrimSpace(perImageFlow) == "" {
			perImageFlow = editFlowStr
		}

		// 生成新的 objectKey
		originExt := strings.ToLower(filepath.Ext(im.OriginObjectKey))
		if originExt == "" {
			originExt = ".png"
		}
		targetObjKey := fmt.Sprintf("material/%d/images/%s/%d-%d%s", materialID, time.Now().Format("20060102"), time.Now().UnixNano(), im.ID, originExt)

		updateFields := map[string]interface{}{
			"object_key": targetObjKey,
			"edit_flow":  perImageFlow,
		}

		// 上传到 OSS
		if err := s.oss.Put(entity.ConvertContext(c), targetObjKey, result.OutputPath); err != nil {
			return nil, fmt.Errorf("上传OSS失败: %w", err)
		}

		// 同步更新关联的 file 记录
		f, _ := s.repo.GetMaterialFile(c, im.MaterialFileID)
		if f != nil && fileGroupByExt(filepath.Ext(f.Name)) == "image" {
			_ = s.repo.UpdateMaterialFileInfo(c, f.ID, map[string]interface{}{"object_key": targetObjKey})
		}

		return uploadInfo{
			imageID:      imageID,
			updateFields: updateFields,
			targetObjKey: targetObjKey,
			im:           im,
		}, nil
	})

	// 批量更新数据库
	uploadSuccessCount := 0
	uploadFailCount := 0
	for _, r := range uploadResults {
		if r.Error != nil {
			uploadFailCount++
			logger.Errorw("上传失败", "err", r.Error)
			continue
		}
		info, ok := r.Data.(uploadInfo)
		if !ok {
			uploadFailCount++
			continue
		}
		if err := s.repo.UpdateMaterialImageInfo(c, info.imageID, info.updateFields); err != nil {
			uploadFailCount++
			logger.Errorw("更新数据库失败", "err", err, "image_id", info.imageID)
			continue
		}
		uploadSuccessCount++
	}

	logger.Infow("批量处理完成",
		"total", len(downloadTasks),
		"downloadSuccess", downloadSuccessCount,
		"uploadSuccess", uploadSuccessCount,
		"uploadFail", uploadFailCount,
	)

	handler.SendOKResp(c, map[string]interface{}{
		"total":   len(downloadTasks),
		"success": uploadSuccessCount,
		"fail":    len(downloadTasks) - uploadSuccessCount,
	})
}

func materialTypeName(t string) string {
	for _, it := range materialTypeOptions {
		if it.Type == t {
			return it.Name
		}
	}
	return t
}

func getMultipartFiles(c *gin.Context, maxCount int) ([]*multipart.FileHeader, error) {
	form, _ := c.MultipartForm()
	if form == nil {
		if fh, err := c.FormFile("file"); err == nil && fh != nil {
			return []*multipart.FileHeader{fh}, nil
		}
		return nil, nil
	}
	var files []*multipart.FileHeader
	if hs := form.File["file"]; len(hs) > 0 {
		files = append(files, hs...)
	}
	if hs := form.File["file[]"]; len(hs) > 0 {
		files = append(files, hs...)
	}
	if len(files) > maxCount {
		return nil, fmt.Errorf("单次上传文件不超过%d个", maxCount)
	}
	return files, nil
}

func validateSameGroup(files []*multipart.FileHeader) (string, error) {
	var group string
	for _, fh := range files {
		if fh == nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		g := fileGroupByExt(ext)
		if g == "" {
			return "", fmt.Errorf("不支持的文件类型: %s", ext)
		}
		if group == "" {
			group = g
		} else if group != g {
			return "", fmt.Errorf("仅支持上传同类型文件")
		}
	}
	return group, nil
}

func validateFileSizes(files []*multipart.FileHeader, group string) error {
	for _, fh := range files {
		if fh == nil {
			continue
		}
		if group == "image" {
			if fh.Size > 5*1024*1024 {
				return fmt.Errorf("图片大小不能超过5MB")
			}
		} else {
			if fh.Size > 200*1024*1024 {
				return fmt.Errorf("文档大小不能超过200MB")
			}
		}
	}
	return nil
}

func validateMixedFiles(files []*multipart.FileHeader) error {
	for _, fh := range files {
		if fh == nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		group := fileGroupByExt(ext)
		if group == "" {
			return fmt.Errorf("不支持的文件类型: %s", ext)
		}
		if group == "image" {
			if fh.Size > 5*1024*1024 {
				return fmt.Errorf("图片大小不能超过5MB")
			}
			continue
		}
		if fh.Size > 200*1024*1024 {
			return fmt.Errorf("文档大小不能超过200MB")
		}
	}
	return nil
}

func validateGalleryImageFiles(files []*multipart.FileHeader) error {
	for _, fh := range files {
		if fh == nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return fmt.Errorf("不支持的文件类型: %s", ext)
		}
		if fh.Size > 5*1024*1024 {
			return fmt.Errorf("图片大小不能超过5MB")
		}
	}
	return nil
}

func splitFiles(files []*multipart.FileHeader) (imageFiles []*multipart.FileHeader, docFiles []*multipart.FileHeader) {
	for _, fh := range files {
		if fh == nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		group := fileGroupByExt(ext)
		if group == "image" {
			imageFiles = append(imageFiles, fh)
			continue
		}
		if group == "doc" {
			docFiles = append(docFiles, fh)
		}
	}
	return imageFiles, docFiles
}

func fileGroupByExt(ext string) string {
	if isImageExt(ext) {
		return "image"
	}
	switch strings.ToLower(ext) {
	case ".doc", ".docx", ".pdf":
		return "doc"
	default:
		return ""
	}
}

func isImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func (s *svcImpl) saveOneDocAndRecords(c *gin.Context, materialID int64, fh *multipart.FileHeader, sortOrder int32) (int64, string, error) {
	clean := func(name string) string {
		reg := regexp.MustCompile(`[^\p{Han}A-Za-z0-9_.-]+`)
		n := reg.ReplaceAllString(name, "")
		return strings.ReplaceAll(n, "丨", "")
	}
	base := filepath.Base(fh.Filename)
	ext := strings.ToLower(filepath.Ext(base))
	nameOnly := strings.TrimSuffix(base, ext)
	fileNameWithExt := clean(nameOnly) + ext

	localPath := filepath.Join("./tmp", fmt.Sprintf("material-%d-%d-%s", materialID, time.Now().UnixNano(), fileNameWithExt))
	if err := c.SaveUploadedFile(fh, localPath); err != nil {
		return 0, "", fmt.Errorf("保存文件失败")
	}
	defer func() { _ = os.Remove(localPath) }()

	objKey := fmt.Sprintf("material/%d/%s/%d-%s", materialID, time.Now().Format("20060102"), time.Now().UnixNano(), fileNameWithExt)
	if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
		return 0, "", fmt.Errorf("上传失败")
	}

	fileRec := &model.MaterialFileInfo{
		MaterialID: materialID,
		Name:       base,
		Size:       fh.Size,
		ObjectKey:  objKey,
		SortOrder:  sortOrder,
	}
	if err := s.repo.CreateMaterialFile(c, fileRec); err != nil {
		return 0, "", fmt.Errorf("保存失败")
	}

	return fileRec.ID, objKey, nil
}

func (s *svcImpl) saveOneGalleryImageAndRecord(c *gin.Context, userID int64, companyID int32, keepOriginalName bool, fh *multipart.FileHeader, sortOrder int32) (int64, string, error) {
	clean := func(name string) string {
		reg := regexp.MustCompile(`[^\p{Han}A-Za-z0-9_.-]+`)
		n := reg.ReplaceAllString(name, "")
		return strings.ReplaceAll(n, "丨", "")
	}
	base := filepath.Base(fh.Filename)
	ext := strings.ToLower(filepath.Ext(base))
	nameOnly := strings.TrimSuffix(base, ext)
	fileNameWithExt := clean(nameOnly) + ext
	if fileNameWithExt == "" {
		fileNameWithExt = fmt.Sprintf("image-%d%s", time.Now().UnixNano(), ext)
	}

	localPath := filepath.Join("./tmp", fmt.Sprintf("material-gallery-%d-%d-%s", userID, time.Now().UnixNano(), fileNameWithExt))
	if err := c.SaveUploadedFile(fh, localPath); err != nil {
		return 0, "", fmt.Errorf("保存文件失败")
	}
	defer func() { _ = os.Remove(localPath) }()

	objKey := fmt.Sprintf("material/gallery/%d/%s/%d-%s", userID, time.Now().Format("20060102"), time.Now().UnixNano(), fileNameWithExt)
	if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
		return 0, "", fmt.Errorf("上传失败")
	}

	name := ""
	if keepOriginalName {
		name = base
	}
	imgRec := &model.MaterialImageInfo{
		MaterialID:      0,
		MaterialFileID:  0,
		Name:            name,
		Description:     "",
		Size:            fh.Size,
		OriginObjectKey: objKey,
		ObjectKey:       objKey,
		SortOrder:       sortOrder,
		EditFlow:        "",
		UserID:          userID,
		CompanyID:       companyID,
	}
	if err := s.repo.CreateMaterialImage(c, imgRec); err != nil {
		return 0, "", fmt.Errorf("保存失败")
	}
	return imgRec.ID, objKey, nil
}

func (s *svcImpl) saveOneImageAndRecords(c *gin.Context, materialID int64, userID int64, companyID int32, fh *multipart.FileHeader, sortOrder int32) (int64, string, error) {
	clean := func(name string) string {
		reg := regexp.MustCompile(`[^\p{Han}A-Za-z0-9_.-]+`)
		n := reg.ReplaceAllString(name, "")
		return strings.ReplaceAll(n, "丨", "")
	}
	base := filepath.Base(fh.Filename)
	ext := strings.ToLower(filepath.Ext(base))
	nameOnly := strings.TrimSuffix(base, ext)
	fileNameWithExt := clean(nameOnly) + ext
	if fileNameWithExt == "" {
		fileNameWithExt = fmt.Sprintf("image-%d%s", time.Now().UnixNano(), ext)
	}

	localPath := filepath.Join("./tmp", fmt.Sprintf("material-img-%d-%d-%s", materialID, time.Now().UnixNano(), fileNameWithExt))
	if err := c.SaveUploadedFile(fh, localPath); err != nil {
		return 0, "", fmt.Errorf("保存文件失败")
	}
	defer func() { _ = os.Remove(localPath) }()

	objKey := fmt.Sprintf("material/%d/images/%s/%d-%s", materialID, time.Now().Format("20060102"), time.Now().UnixNano(), fileNameWithExt)
	if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
		return 0, "", fmt.Errorf("上传失败")
	}

	imgRec := &model.MaterialImageInfo{
		MaterialID:      materialID,
		MaterialFileID:  0,
		Name:            "", //不保存文件名，后续会由智能体自动填充
		Size:            fh.Size,
		OriginObjectKey: objKey,
		ObjectKey:       objKey,
		SortOrder:       sortOrder,
		EditFlow:        "",
		UserID:          userID,
		CompanyID:       companyID,
	}
	if err := s.repo.CreateMaterialImage(c, imgRec); err != nil {
		return 0, "", fmt.Errorf("保存失败")
	}
	return imgRec.ID, objKey, nil
}

func (s *svcImpl) extractImagesFromDocAndSave(c *gin.Context, materialID int64, userID int64, companyID int32, filename, objectKey string, startSort int32) (int32, error) {
	rc, err := s.oss.Open(entity.ConvertContext(c), objectKey)
	if err != nil {
		return startSort, fmt.Errorf("读取文档失败")
	}
	defer rc.Close()

	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.TrimSuffix(filepath.Base(filename), ext)
	localPath := filepath.Join("./tmp", fmt.Sprintf("material-doc-%d-%d-%s%s", materialID, time.Now().UnixNano(), base, ext))
	defer func() { _ = os.Remove(localPath) }()

	f, err := os.Create(localPath)
	if err != nil {
		return startSort, fmt.Errorf("保存文档失败")
	}
	if _, err2 := io.Copy(f, rc); err2 != nil {
		_ = f.Close()
		return startSort, fmt.Errorf("保存文档失败")
	}
	_ = f.Close()

	imgs, err := utils.DocExtractImagesAuto(localPath)
	if err != nil {
		return startSort, fmt.Errorf("抽取图片失败")
	}
	if len(imgs) == 0 {
		return startSort, nil
	}
	nextSort := startSort
	for i, im := range imgs {
		decoded, _, err := image.Decode(bytes.NewReader(im.Content))
		if err != nil {
			return startSort, fmt.Errorf("抽取图片失败")
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, decoded); err != nil {
			return startSort, fmt.Errorf("抽取图片失败")
		}
		pngBytes := buf.Bytes()

		imgName := fmt.Sprintf("%s_extract_%02d.png", base, i+1)
		tmpImgPath := filepath.Join("./tmp", fmt.Sprintf("material-img-%d-%d-%s", materialID, time.Now().UnixNano(), imgName))
		if err := os.WriteFile(tmpImgPath, pngBytes, 0o644); err != nil {
			return startSort, fmt.Errorf("保存图片失败")
		}

		imgObjKey := fmt.Sprintf("material/%d/images/%s/%d-%s", materialID, time.Now().Format("20060102"), time.Now().UnixNano(), imgName)
		if err := s.oss.Put(entity.ConvertContext(c), imgObjKey, tmpImgPath); err != nil {
			_ = os.Remove(tmpImgPath)
			return startSort, fmt.Errorf("上传图片失败")
		}
		_ = os.Remove(tmpImgPath)
		imgRec := &model.MaterialImageInfo{
			MaterialID:      materialID,
			MaterialFileID:  0,
			Name:            "",
			Description:     "",
			Size:            int64(len(pngBytes)),
			OriginObjectKey: imgObjKey,
			ObjectKey:       imgObjKey,
			SortOrder:       nextSort,
			EditFlow:        "",
			UserID:          userID,
			CompanyID:       companyID,
		}
		if err := s.repo.CreateMaterialImage(c, imgRec); err != nil {
			return startSort, fmt.Errorf("保存图片失败")
		}
		nextSort++
	}
	return nextSort, nil
}

func collectObjectKeys(files []*model.MaterialFileInfo, images []*model.MaterialImageInfo) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	for _, f := range files {
		if f == nil {
			continue
		}
		add(f.ObjectKey)
	}
	for _, im := range images {
		if im == nil {
			continue
		}
		add(im.ObjectKey)
		add(im.OriginObjectKey)
	}
	return out
}

func urlQueryEscape(s string) string {
	replacer := strings.NewReplacer(
		" ", "%20",
		"!", "%21",
		"\"", "%22",
		"#", "%23",
		"$", "%24",
		"%", "%25",
		"&", "%26",
		"'", "%27",
		"(", "%28",
		")", "%29",
		"*", "%2A",
		"+", "%2B",
		",", "%2C",
		"/", "%2F",
		":", "%3A",
		";", "%3B",
		"<", "%3C",
		"=", "%3D",
		">", "%3E",
		"?", "%3F",
		"@", "%40",
		"[", "%5B",
		"\\", "%5C",
		"]", "%5D",
		"^", "%5E",
		"`", "%60",
		"{", "%7B",
		"|", "%7C",
		"}", "%7D",
	)
	return replacer.Replace(s)
}

// ========== 共享辅助方法 ==========

// saveUploadedFiles 保存上传的文件到 OSS 和 DB
func (s *svcImpl) saveUploadedFiles(c *gin.Context, materialID int64, userID int64, companyID int32, files []*multipart.FileHeader) {
	if err := os.MkdirAll("./tmp", 0o777); err != nil {
		return
	}
	imageFiles, docFiles := splitFiles(files)
	nextImageSort := int32(1)
	nextDocSort := int32(1)
	for _, fh := range imageFiles {
		_, _, err := s.saveOneImageAndRecords(c, materialID, userID, companyID, fh, nextImageSort)
		if err != nil {
			s.logger.With(entity.Ctx(c)...).Warnw("保存图片失败", "name", fh.Filename, "err", err)
			continue
		}
		nextImageSort++
	}
	for _, fh := range docFiles {
		_, _, err := s.saveOneDocAndRecords(c, materialID, fh, nextDocSort)
		if err != nil {
			s.logger.With(entity.Ctx(c)...).Warnw("保存文档失败", "name", fh.Filename, "err", err)
			continue
		}
		nextDocSort++
	}
}

// triggerPostCreateAsync 触发创建后的异步任务（OCR 等）
func (s *svcImpl) triggerPostCreateAsync(c *gin.Context, materialID int64, materialType string, req entity.MaterialAddReq) {
	if materialType == "qualification" || materialType == "performance" {
		go s.asyncRunOcr(c, materialID)
	}
}

// renderMaterialDetail 渲染素材详情响应
func (s *svcImpl) renderMaterialDetail(c *gin.Context, materialID int64, name, materialType, description string, userID int64, companyID int32, createdAt time.Time) {
	companyName, _ := s.repo.GetCompanyName(c, companyID)
	userName, _ := s.repo.GetUserName(c, userID)

	files, _ := s.repo.ListMaterialFiles(c, materialID)
	images, _ := s.repo.ListMaterialImages(c, materialID)

	imageFiles := make([]*entity.MaterialFileItem, 0)
	docFiles := make([]*entity.MaterialFileItem, 0)
	for _, f := range files {
		item := &entity.MaterialFileItem{
			ID: f.ID, MaterialID: f.MaterialID, Name: f.Name, Size: f.Size,
			ObjectKey: f.ObjectKey, SortOrder: f.SortOrder, CreatedTime: f.CreatedAt.Unix(),
		}
		if isImageExt(filepath.Ext(f.Name)) {
			imageFiles = append(imageFiles, item)
		} else {
			docFiles = append(docFiles, item)
		}
	}
	for _, im := range images {
		imageFiles = append(imageFiles, &entity.MaterialFileItem{
			ID: im.ID, MaterialID: im.MaterialID, Name: im.Name, Size: im.Size,
			ObjectKey: im.ObjectKey, SortOrder: im.SortOrder, CreatedTime: im.CreatedAt.Unix(),
			MaterialFileID: im.MaterialFileID, OriginObjectKey: im.OriginObjectKey, EditFlow: im.EditFlow,
		})
	}

	resp := &entity.MaterialDetailResp{
		ID: materialID, Name: name, Type: materialType, TypeName: materialTypeName(materialType),
		Description: description, UserID: userID, UserName: userName,
		CompanyID: companyID, CompanyName: companyName,
		CreatedTime: createdAt.Format("2006-1-2 15:04:05"),
		ImageFiles:  imageFiles, DocFiles: docFiles,
	}
	handler.SendOKResp(c, resp)
}
