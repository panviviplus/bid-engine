package feedback

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/feedback"
)

// OptionsResp 反馈筛选项
type OptionsResp struct {
	Types []string `json:"types"`
}

func (s *svcImpl) Options(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	// 鉴权
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	var types []string
	var err error
	if entity.IsSuperAdmin(c) {
		types, err = s.repo.GetTypes(c)
	} else {
		types, err = s.repo.GetTypesForUser(c.Request.Context(), entity.GetUserIDFromCtx(c))
	}
	if err != nil {
		logger.Warnw("获取反馈类型失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "获取筛选项失败", nil)
		return
	}

	// 添加固定反馈类型并去重
	fixedTypes := []string{"suggest", "question", "other"}
	typeSet := make(map[string]struct{})

	// 添加数据库返回的类型
	for _, t := range types {
		typeSet[t] = struct{}{}
	}

	// 添加固定类型
	for _, ft := range fixedTypes {
		typeSet[ft] = struct{}{}
	}

	// 转换回切片
	allTypes := make([]string, 0, len(typeSet))
	for t := range typeSet {
		allTypes = append(allTypes, t)
	}

	handler.SendOKResp(c, OptionsResp{Types: allTypes})
}

func (s *svcImpl) SearchRecords(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	// 登录校验
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	var req entity.SearchFeedbackRecordsReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	// 构造查询参数
	keyword := strings.TrimSpace(req.Keyword)
	fType := strings.TrimSpace(req.Type)
	start := req.StartTime
	end := req.EndTime

	param := feedback.SearchParam{
		SearchParam: entity.SearchParam{
			PageSize: req.PageSize,
			PageNum:  req.PageNum,
		},
		Type:      fType,
		StartTime: start,
		EndTime:   end,
	}
	param.Query = keyword
	// 权限控制：系统管理员可看全部；否则仅能看自己的反馈记录
	if !entity.IsSuperAdmin(c) {
		param.UserID = entity.GetUserIDFromCtx(c)
	}

	list, total, err := s.repo.SearchRecords(c, param)
	if err != nil {
		logger.Warnw("查询反馈记录失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询失败", nil)
		return
	}
	// 构造响应值：批量查用户手机号并拼装返回
	var resp []entity.SearchFeedbackRespItem
	if len(list) > 0 {
		// 收集并去重 userID
		idSet := make(map[int64]struct{}, len(list))
		userIDs := make([]int64, 0, len(list))
		for _, it := range list {
			if _, ok := idSet[it.UserID]; !ok {
				idSet[it.UserID] = struct{}{}
				userIDs = append(userIDs, it.UserID)
			}
		}

		// 批量获取用户信息
		users, uErr := s.user.GetUsers(c, userIDs)
		if uErr != nil {
			logger.Warnw("获取用户信息失败", "err", uErr)
		}
		uMap := make(map[int64]*model.User, len(users))
		for _, u := range users {
			uMap[u.UserID] = u
		}

		for _, it := range list {
			mobile := ""
			if u, ok := uMap[it.UserID]; ok && u != nil {
				mobile = u.Mobile
			}
			resp = append(resp, entity.SearchFeedbackRespItem{
				Id:          it.ID,
				UserId:      it.UserID,
				UserName:    it.Nickname,
				Mobile:      mobile,
				Type:        it.Type,
				Description: it.Description,
				Status:      it.Status,
				CreateTime:  it.CreateTime.Format("2006-01-02 15:04:05"),
				UpdateTime:  it.UpdateTime.Format("2006-01-02 15:04:05"),
			})
		}
	}

	handler.SendOKResp(c, map[string]interface{}{
		"total": total,
		"items": resp,
	})
}

func (s *svcImpl) GetRecord(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	// 登录校验
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	idStr := c.Param("record_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "record_id 参数错误", nil)
		return
	}
	var rec *model.FeedbackRecord
	var err error
	if entity.IsSuperAdmin(c) {
		rec, err = s.repo.GetRecord(c, id)
	} else {
		rec, err = s.repo.GetRecordForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), id)
	}
	if err != nil {
		logger.Warnw("获取反馈记录失败", "id", id, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "获取失败", nil)
		return
	}

	feedbackUser, _ := s.user.GetUser(c, rec.UserID)

	// 解析对象键为列表，使用项目统一分隔符
	parsePhotoKeys := func(s string) []string {
		s = strings.TrimSpace(s)
		if s == "" {
			return []string{}
		}
		parts := strings.Split(s, entity.FeedbackPhotoKeySep)
		res := make([]string, 0, len(parts))
		for _, k := range parts {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			res = append(res, k)
		}
		return res
	}

	// 专用返回结构体
	type FeedbackRecordDetailResp struct {
		Id          int64    `json:"id"`
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Status      int32    `json:"status"`
		UserId      int64    `json:"userId"`
		Nickname    string   `json:"nickname"`
		Mobile      string   `json:"mobile"`
		CompanyId   int32    `json:"companyId"`
		PhotoKeys   []string `json:"photoKeys"`
		CreateTime  string   `json:"createTime"`
		UpdateTime  string   `json:"updateTime"`
	}

	resp := FeedbackRecordDetailResp{
		Id:          rec.ID,
		Type:        rec.Type,
		Description: rec.Description,
		Status:      rec.Status,
		UserId:      rec.UserID,
		Nickname:    rec.Nickname,
		CompanyId:   rec.CompanyID,
		PhotoKeys:   parsePhotoKeys(rec.PhotoKeys),
		CreateTime:  rec.CreateTime.Format("2006-01-02 15:04:05"),
		UpdateTime:  rec.UpdateTime.Format("2006-01-02 15:04:05"),
	}
	if feedbackUser != nil {
		resp.Mobile = feedbackUser.Mobile
	}

	handler.SendOKResp(c, resp)
}

func (s *svcImpl) AddRecord(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	// 登录校验
	reqUser := entity.GetUserFromCtx(c)
	if reqUser == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	var req entity.AddFeedbackRecordReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	// 预置的图片keys（通过表单字段传递），使用统一分隔符
	photoKeys := make([]string, 0)

	form, _ := c.MultipartForm()
	if form != nil {
		// 仅接收名为 "file" 的字段（业界常规做法：多张图使用同名字段多次传入或数组形式）
		var files []*multipart.FileHeader
		if hs := form.File["file"]; len(hs) > 0 {
			files = append(files, hs...)
		}
		if hs := form.File["file[]"]; len(hs) > 0 {
			files = append(files, hs...)
		}
		// 兜底：部分客户端只会上传单个名为 file 的文件
		if len(files) == 0 {
			if fh, err := c.FormFile("file"); err == nil && fh != nil {
				files = []*multipart.FileHeader{fh}
			}
		}

		if len(files) > 0 {
			_ = os.MkdirAll("./tmp", 0777)
			clean := func(name string) string {
				reg := regexp.MustCompile(`[^\p{Han}A-Za-z0-9_-]+`)
				n := reg.ReplaceAllString(name, "")
				return strings.ReplaceAll(n, "丨", "")
			}
			const maxSize = 10 * 1024 * 1024
			for _, fh := range files {
				if fh == nil {
					continue
				}
				if fh.Size > maxSize {
					handler.SendNormalResp(c, entity.ErrCodeParam, "图片大小不能超过10MB", nil)
					return
				}
				base := filepath.Base(fh.Filename)
				ext := strings.ToLower(filepath.Ext(base))
				nameOnly := strings.TrimSuffix(base, ext)
				fileNameWithExt := clean(nameOnly) + ext

				localPath := filepath.Join("./tmp", fmt.Sprintf("feedback-%d-%s", time.Now().UnixNano(), fileNameWithExt))
				if err := c.SaveUploadedFile(fh, localPath); err != nil {
					logger.Warnw("保存图片到本地失败", "filename", fh.Filename, "err", err)
					handler.SendNormalResp(c, entity.ErrCodeInternal, "保存图片失败", nil)
					return
				}
				defer func(p string) { _ = os.Remove(p) }(localPath)

				objKey := fmt.Sprintf("feedback/%s/%d-%s", time.Now().Format("20060102"), time.Now().UnixNano(), fileNameWithExt)
				if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
					logger.Warnw("上传图片到OSS失败", "object", objKey, "err", err)
					handler.SendNormalResp(c, entity.ErrCodeOSSWrite, "图片上传失败", nil)
					return
				}
				photoKeys = append(photoKeys, objKey)
			}
		}
	}

	now := time.Now()
	// 去重一次，避免重复的 key
	finalKeys := make([]string, 0, len(photoKeys))
	seen := make(map[string]struct{}, len(photoKeys))
	for _, k := range photoKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		finalKeys = append(finalKeys, k)
	}

	rec := &model.FeedbackRecord{
		Type:        req.Type,
		Description: req.Description,
		UserID:      reqUser.UserID,
		Nickname:    reqUser.Nickname,
		CompanyID:   reqUser.CompanyID,
		PhotoKeys:   strings.Join(finalKeys, entity.FeedbackPhotoKeySep),
		CreateTime:  now,
		UpdateTime:  now,
		Status:      entity.CommonStatusUnavailable,
	}
	if req.Status != nil {
		rec.Status = *req.Status
	}

	if err := s.repo.AddRecord(c, rec); err != nil {
		logger.Warnw("新增反馈记录失败", "record", rec, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "添加失败", nil)
		return
	}
	handler.SendOKResp(c, rec)
}

func (s *svcImpl) UpdateRecord(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	// 登录校验
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	idStr := c.Param("record_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "record_id 参数错误", nil)
		return
	}

	var req entity.UpdateFeedbackRecordReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	var err error
	if entity.IsSuperAdmin(c) {
		err = s.repo.UpdateDescription(c, id, req.Description)
	} else {
		err = s.repo.UpdateDescriptionForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), id, req.Description)
	}
	if err != nil {
		logger.Warnw("更新反馈记录失败", "id", id, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"id": id})
}

func (s *svcImpl) DeleteRecord(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)

	// 登录校验
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	var req entity.DeleteFeedbackRecordReq
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	var err error
	if entity.IsSuperAdmin(c) {
		err = s.repo.DeleteRecord(c, req.RecordID)
	} else {
		err = s.repo.DeleteRecordForUser(c.Request.Context(), entity.GetUserIDFromCtx(c), req.RecordID)
	}
	if err != nil {
		logger.Warnw("删除反馈记录失败", "id", req.RecordID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "删除失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"id": req.RecordID})
}

func (s *svcImpl) UpdateStatus(c *gin.Context) {
	logger := s.logger.With(entity.Ctx(c)...)
	// 仅系统管理员可操作
	if !entity.IsSuperAdmin(c) {
		handler.SendNormalResp(c, entity.ErrCodeForbidden, entity.ErrMsgForbidden, nil)
		return
	}

	idStr := c.Param("record_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if id <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "record_id 参数错误", nil)
		return
	}

	// 获取当前记录状态
	rec, err := s.repo.GetRecord(c, id)
	if err != nil {
		logger.Warnw("获取反馈记录失败", "id", id, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "获取失败", nil)
		return
	}

	// 切换状态：0未处理 -> 1已处理，1已处理 -> 0未处理
	newStatus := entity.CommonStatusAvailable
	if rec.Status == entity.CommonStatusAvailable {
		newStatus = entity.CommonStatusUnavailable
	}

	if err := s.repo.UpdateStatus(c, id, newStatus); err != nil {
		logger.Warnw("更新反馈记录状态失败", "id", id, "status", newStatus, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新失败", nil)
		return
	}
	handler.SendOKResp(c, map[string]any{"id": id, "status": newStatus})
}
