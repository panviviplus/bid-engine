package user

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

func publicBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	base := strings.TrimSpace(skbcfg.Get("properties.public_base_url"))
	if base == "" {
		base = strings.TrimSpace(skbcfg.Get("properties.api_base_url"))
	}
	if base != "" {
		return strings.TrimRight(base, "/")
	}
	scheme := "http"
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func (s *svcImpl) Info(c *gin.Context) {
	var logger = s.logger.With(entity.Ctx(c)...)
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		logger.Warnw("Info --> 从ctx获取用户ID错误")
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "获取用户信息错误", nil)
		return
	}

	// 从数据库重新读取最新用户数据（auth 中间件缓存的 ctx 数据在编辑资料后不会自动刷新）
	u, err := s.user.GetUser(c, userID)
	if err != nil || u == nil {
		logger.Warnw("Info --> 查询用户失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "获取用户信息错误", nil)
		return
	}

	var companyId int64
	var companyName string
	if u.CompanyID > 0 {
		company, err := s.company.GetCompany(c, u.CompanyID)
		if err != nil {
			logger.Warnw("Info --> 根据公司ID获取公司失败", "companyID", u.CompanyID, "err", err)
		}
		if company != nil {
			companyId = int64(company.ID)
			companyName = company.Name
		}
	}

	// 构建用户信息响应
	userInfoResp := entity.UserInfoResponse{
		Name:       u.Nickname,
		CompanyId:  companyId,
		Company:    companyName,
		UserID:     u.UserID,
		Mobile:     u.Mobile,
		Email:      u.Email,
		Status:     u.Status,
		UpdateTime: u.UpdateTime,
		UserRole:   s.JudgeCurUserIdentity(c, u.UserID),
	}
	userInfoResp.AvatarFile = u.AvatarFile
	if strings.TrimSpace(u.AvatarFile) != "" {
		if base := publicBaseURL(c); base != "" {
			userInfoResp.AvatarURL = base + "/user/avatar?ts=" + strconv.FormatInt(u.UpdateTime, 10)
		}
	}
	handler.SendOKResp(c, userInfoResp)
}

func (s *svcImpl) UpdateProfile(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
	)

	// 1. 认证检查（提前到绑定之前，避免无效操作）
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	u, err := s.user.GetUser(c, userID)
	if err != nil {
		logger.Warnw("UpdateProfile --> 查询用户失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询用户失败", nil)
		return
	}
	if u == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	// 2. 直接从 multipart form 读取字段（避免 Gin ShouldBind + *string 指针在 FormMultipart
	//    绑定器下的静默失败：当 form key 不存在时 isSet=false，指针保持 nil，handler 不更新任何字段但返回成功）
	nickname := strings.TrimSpace(c.PostForm("nickname"))
	mobile := strings.TrimSpace(c.PostForm("mobile"))
	email := strings.TrimSpace(c.PostForm("email"))

	// 3. 参数校验
	// 飞书新用户的未验证联系电话不得写进可用于短信登录的 mobile 字段。
	if u.Mobile == "" && mobile != "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请在联系资料中填写联系电话；未验证号码不能用于登录", nil)
		return
	}
	if mobile != "" {
		if len(mobile) != 11 {
			handler.SendNormalResp(c, entity.ErrCodeParam, "手机号码必须为11位", nil)
			return
		}
		for _, r := range mobile {
			if r < '0' || r > '9' {
				handler.SendNormalResp(c, entity.ErrCodeParam, "手机号码格式不正确", nil)
				return
			}
		}
	}

	// 4. 头像文件上传处理
	objKey := strings.TrimSpace(u.AvatarFile)
	fh, err := c.FormFile("file")
	if err != nil {
		logger.Infow("UpdateProfile --> 未接收到头像文件或解析失败", "method", c.Request.Method, "contentType", c.ContentType(), "err", err)
	} else if fh == nil || fh.Size == 0 {
		logger.Infow("UpdateProfile --> 头像文件为空", "method", c.Request.Method, "contentType", c.ContentType())
	} else {
		logger.Infow("UpdateProfile --> 接收到头像文件", "filename", fh.Filename, "size", fh.Size)
		const maxSize = 5 * 1024 * 1024
		if fh.Size > maxSize {
			handler.SendNormalResp(c, entity.ErrCodeParam, "头像大小不能超过5MB", nil)
			return
		}
		ext := strings.ToLower(filepath.Ext(filepath.Base(fh.Filename)))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif":
		default:
			handler.SendNormalResp(c, entity.ErrCodeParam, "头像格式仅支持jpg/jpeg/png/gif", nil)
			return
		}
		if err := os.MkdirAll("./tmp", 0777); err != nil {
			logger.Warnw("UpdateProfile --> 创建临时目录失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, "上传失败", nil)
			return
		}
		nowNano := time.Now().UnixNano()
		localPath := filepath.Join("./tmp", fmt.Sprintf("avatar-%d-%d%s", userID, nowNano, ext))
		if err := c.SaveUploadedFile(fh, localPath); err != nil {
			logger.Warnw("UpdateProfile --> 保存头像到本地失败", "filename", fh.Filename, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, "上传失败", nil)
			return
		}
		defer func() { _ = os.Remove(localPath) }()
		if objKey == "" {
			objKey = fmt.Sprintf("bid-engine/userinfo/%d%s", userID, ext)
		}
		if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
			logger.Warnw("UpdateProfile --> 上传头像到OSS失败", "object", objKey, "err", err)
			handler.SendNormalResp(c, entity.ErrCodeOSSWrite, "上传失败", nil)
			return
		}
		logger.Infow("UpdateProfile --> 上传头像到OSS成功", "object", objKey)
		u.AvatarFile = strings.TrimSpace(objKey)
	}

	// 5. 应用字段更新
	avatarChanged := u.AvatarFile != strings.TrimSpace(objKey)
	if nickname != "" {
		u.Nickname = nickname
	}
	if mobile != "" {
		u.Mobile = mobile
	}
	u.Email = email // 邮箱允许设为空（清空）
	u.UpdateTime = time.Now().Unix()

	// 6. 精确字段更新（只更新变化的列，避免 Save 的全表覆写风险）
	if err := s.user.UpdateUserFields(c, u.UserID, u.Nickname, u.Mobile, u.Email, u.AvatarFile, u.UpdateTime); err != nil {
		logger.Warnw("UpdateProfile --> 更新用户失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "更新用户失败", nil)
		return
	}
	// 刷新 auth 中间件缓存的 ctx 用户数据，确保后续请求（包括 /info）能读到最新值
	entity.SetUserToCtx(c, u)
	_ = avatarChanged // 标记头像是否变更（预留扩展点）

	// 7. 构建响应
	var companyId int64
	var companyName string
	if u.CompanyID > 0 {
		company, err := s.company.GetCompany(c, u.CompanyID)
		if err != nil {
			logger.Warnw("UpdateProfile --> 根据公司ID获取公司失败", "companyID", u.CompanyID, "err", err)
		}
		if company != nil {
			companyId = int64(company.ID)
			companyName = company.Name
		}
	}
	name := u.Nickname
	if name == "" {
		name = u.Mobile
	}
	userInfoResp := entity.UserInfoResponse{
		Name:       name,
		CompanyId:  companyId,
		Company:    companyName,
		UserID:     u.UserID,
		Mobile:     u.Mobile,
		Email:      u.Email,
		Status:     u.Status,
		UpdateTime: u.UpdateTime,
		UserRole:   s.JudgeCurUserIdentity(c, u.UserID),
	}
	userInfoResp.AvatarFile = u.AvatarFile
	if strings.TrimSpace(u.AvatarFile) != "" {
		if base := publicBaseURL(c); base != "" {
			userInfoResp.AvatarURL = base + "/user/avatar?ts=" + strconv.FormatInt(u.UpdateTime, 10)
		}
	}
	handler.SendOKResp(c, userInfoResp)
}

func (s *svcImpl) ChangePassword(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.ChangePasswordReq
	)
	if err := req.Valid(c); err != nil {
		logger.Warnw("ChangePassword --> 参数错误", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	userID := entity.GetUserIDFromCtx(c)
	u, err := s.user.GetUser(c, userID)
	if err != nil {
		logger.Warnw("ChangePassword --> 查询用户失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询用户失败", nil)
		return
	}
	if u == nil {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}

	// 验证旧密码
	if !comparePassword(u.Password, req.OldPassword) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "旧密码不正确", nil)
		return
	}

	// 新密码不能与旧密码相同
	if comparePassword(u.Password, req.NewPassword) {
		handler.SendNormalResp(c, entity.ErrCodeParam, "新密码不能与旧密码相同", nil)
		return
	}

	hashed, err := hashPassword(req.NewPassword)
	if err != nil {
		logger.Warnw("ChangePassword --> 密码加密失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "修改密码失败", nil)
		return
	}

	u.Password = hashed
	u.UpdateTime = time.Now().Unix()
	if err := s.user.UpdateUser(c, u); err != nil {
		logger.Warnw("ChangePassword --> 更新密码失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, "修改密码失败", nil)
		return
	}

	handler.SendOKResp(c, nil)
}

func (s *svcImpl) GetAvatar(c *gin.Context) {
	var logger = s.logger.With(entity.Ctx(c)...)
	// 从数据库读取最新 user 数据（auth 中间件缓存的 ctx 在编辑资料后可能已过时）
	userID := entity.GetUserIDFromCtx(c)
	if userID <= 0 {
		logger.Warnw("GetAvatar --> 从ctx获取用户ID错误")
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "获取用户信息错误", nil)
		return
	}
	u, err := s.user.GetUser(c, userID)
	if err != nil || u == nil {
		logger.Warnw("GetAvatar --> 查询用户失败", "userID", userID, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "获取用户信息错误", nil)
		return
	}
	objKey := strings.TrimSpace(u.AvatarFile)
	if objKey == "" {
		handler.SendNormalResp(c, entity.ErrCodeParam, "用户头像为空", nil)
		return
	}
	ct := "application/octet-stream"
	ext := strings.ToLower(filepath.Ext(objKey))
	switch ext {
	case ".jpg", ".jpeg":
		ct = "image/jpeg"
	case ".png":
		ct = "image/png"
	case ".gif":
		ct = "image/gif"
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", ct)
	reader, err := s.oss.Open(entity.ConvertContext(c), objKey)
	if err != nil {
		logger.Warnw("GetAvatar --> 打开头像对象失败", "object", objKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeS3Read, "获取头像失败", nil)
		return
	}
	defer func() { _ = reader.Close() }()
	c.Status(200)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Warnw("GetAvatar --> 写入响应失败", "object", objKey, "err", err)
	}
}

func (s *svcImpl) UploadAvatar(c *gin.Context) {
	var logger = s.logger.With(entity.Ctx(c)...)
	if entity.GetUserIDFromCtx(c) <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "用户未登录", nil)
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "头像文件不能为空", nil)
		return
	}
	const maxSize = 5 * 1024 * 1024
	if fh.Size > maxSize {
		handler.SendNormalResp(c, entity.ErrCodeParam, "头像大小不能超过5MB", nil)
		return
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(fh.Filename)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif":
	default:
		handler.SendNormalResp(c, entity.ErrCodeParam, "头像格式仅支持jpg/jpeg/png/gif", nil)
		return
	}
	if err := os.MkdirAll("./tmp", 0777); err != nil {
		logger.Warnw("UploadAvatar --> 创建临时目录失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "上传失败", nil)
		return
	}
	userID := entity.GetUserIDFromCtx(c)
	nowNano := time.Now().UnixNano()
	localPath := filepath.Join("./tmp", fmt.Sprintf("avatar-%d-%d%s", userID, nowNano, ext))
	if err := c.SaveUploadedFile(fh, localPath); err != nil {
		logger.Warnw("UploadAvatar --> 保存头像到本地失败", "filename", fh.Filename, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "上传失败", nil)
		return
	}
	defer func() { _ = os.Remove(localPath) }()

	objKey := fmt.Sprintf("avatar/%d/%d%s", userID, nowNano, ext)
	if err := s.oss.Put(entity.ConvertContext(c), objKey, localPath); err != nil {
		logger.Warnw("UploadAvatar --> 上传头像到OSS失败", "object", objKey, "err", err)
		handler.SendNormalResp(c, entity.ErrCodeOSSWrite, "上传失败", nil)
		return
	}
	var avatarURL string
	if u, e := s.oss.GetPresignedURL(entity.ConvertContext(c), objKey, 24*time.Hour); e == nil && u != nil {
		avatarURL = u.String()
	}
	handler.SendOKResp(c, struct {
		AvatarFile string `json:"avatar_file"`
		AvatarURL  string `json:"avatar_url"`
	}{
		AvatarFile: objKey,
		AvatarURL:  avatarURL,
	})
}

func (s *svcImpl) CancelApply(c *gin.Context) {
	// 本地模式不支持审批流程
	handler.SendOKResp(c, nil)
}

// GetUserBySNFromIAM 根据SN码从IAM获取用户信息
func (s *svcImpl) GetUserBySNFromIAM(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
	)
	// 从URL参数获取SN码
	sn := c.Param("sn")
	if sn == "" {
		logger.Warnw("SN码参数为空")
		handler.SendNormalResp(c, entity.ErrCodeParam, "SN码不能为空", nil)
		return
	}
	logger.Infow("根据SN码获取IAM用户信息", "sn", sn)

	// 本地模式不支持IAM查询
	handler.SendNormalResp(c, entity.ErrCodeParam, "本地模式不支持IAM用户查询", nil)
}

func currentCompanyUserOption(current *model.User, requestedCompanyID int64, companyName string) (entity.UserBasicInfo, bool) {
	if current == nil || current.UserID <= 0 || current.CompanyID <= 0 || int64(current.CompanyID) != requestedCompanyID {
		return entity.UserBasicInfo{}, false
	}
	name := strings.TrimSpace(current.Nickname)
	if name == "" {
		name = strings.TrimSpace(current.Username)
	}
	return entity.UserBasicInfo{
		UserID:      current.UserID,
		Name:        name,
		Mobile:      current.Mobile,
		Username:    current.Username,
		Status:      current.Status,
		CompanyID:   requestedCompanyID,
		CompanyName: companyName,
	}, true
}

// ListCompanyUser 返回当前登录用户在目标公司的本人信息。
func (s *svcImpl) ListCompanyUser(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.ListCompanyUserReq
	)
	if err := req.Valid(c); err != nil {
		logger.Warnw("ListCompanyUser --> 参数错误：", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	current := entity.GetUserFromCtx(c)
	if current == nil || current.UserID <= 0 || int64(current.CompanyID) != req.CompanyId {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公司信息不存在", nil)
		return
	}

	company, err := s.company.GetCompany(c, int32(req.CompanyId))
	if err != nil {
		logger.Errorw("ListCompanyUser --> 查询公司失败：", "err", err, "companyId", req.CompanyId)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "查询公司失败", nil)
		return
	}

	item, ok := currentCompanyUserOption(current, req.CompanyId, company.Name)
	if !ok {
		handler.SendNormalResp(c, entity.ErrCodeNotFound, "公司信息不存在", nil)
		return
	}
	handler.SendOKResp(c, []entity.UserBasicInfo{item})
}

// JudgeCurUserIdentity 判断目标用户的身份：admin\company_owner\member
func (s *svcImpl) JudgeCurUserIdentity(c *gin.Context, userId int64) string {
	logger := s.logger.With(entity.Ctx(c)...)
	// 参数兜底
	if userId <= 0 {
		return entity.CurUserIdentityOfCommonMember
	}

	// 判断依据：
	// 1、超管账号，判断依据就是user表中的记录，role字段为1；
	// 2、公司负责人，判断依据是：当前用户是任何一个公司的owner即可；对应company表的owner_id字段；
	// 3、普通成员，判断依据是，不是前两种身份，那就属于成员！

	// 1) 超管判定：优先取上下文已注入的用户（兼容LOCAL_DEV模式），其次查DB
	if cUser := entity.GetUserFromCtx(c); cUser != nil && cUser.Role == 1 {
		return entity.CurUserIdentityOfSystemAdmin
	}
	u, err := s.user.GetUser(c, userId)
	if err != nil {
		logger.Warnw("JudgeCurUserIdentity --> 获取用户信息失败", "userId", userId, "err", err)
	}
	if u != nil && u.Role == 1 {
		return entity.CurUserIdentityOfSystemAdmin
	}

	// 2) 公司负责人判定：任意 company.owner_id == userId
	if u != nil && u.CompanyID > 0 {
		company, err := s.company.GetCompany(c, u.CompanyID)
		if err != nil {
			logger.Warnw("JudgeCurUserIdentity --> 根据公司ID获取公司失败", "companyID", u.CompanyID, "err", err)
		} else if company != nil && company.OwnerID == userId {
			return entity.CurUserIdentityOfCompanyOwner
		}
	}
	// 兜底优化：直接判断是否存在任意公司负责人为该用户（且公司已认证）
	if ok, err := s.company.HasCompanyOwner(c, userId); err != nil {
		logger.Warnw("JudgeCurUserIdentity --> 判断公司负责人存在失败", "userId", userId, "err", err)
	} else if ok {
		return entity.CurUserIdentityOfCompanyOwner
	}

	// 3) 兜底：普通成员
	return entity.CurUserIdentityOfCommonMember
}

func (s *svcImpl) AuthLogin(c *gin.Context) {
	// 本地模式不支持OAuth单点登录
	handler.SendNormalResp(c, entity.ErrCodeParam, "本地模式不支持单点登录", nil)
}
