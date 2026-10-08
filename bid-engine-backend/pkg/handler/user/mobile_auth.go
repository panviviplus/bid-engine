package user

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"bid-engine/lib/common/storage"
	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/middleware"
)

const (
	errorCodeOtpNotMatch = 10004
	errorMsgOtpNotMatch  = "验证码不匹配，请确认后再输入"

	errorCodeOtpTooFast = 10020
	errorMsgOtpTooFast  = "发送过于频繁，请稍后再试"

	errorMsgRegister = "注册失败，请稍后重试"
)

func isProdEnv() bool {
	env := strings.ToLower(strings.TrimSpace(skbcfg.Get("env")))
	return env == "prod" || env == "production"
}

func genOtpCode() (string, error) {
	var sb strings.Builder
	for i := 0; i < 6; i += 1 {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteByte(byte('0' + n.Int64()))
	}
	return sb.String(), nil
}

func (s *svcImpl) verifyOtp(c *gin.Context, scene, mobile, code string) (bool, error) {
	ctx := context.Background()
	return s.otpStore.Verify(ctx, scene, mobile, code)
}

func (s *svcImpl) issueLoginCookies(c *gin.Context, userID int64) error {
	return IssueLoginCookies(c, userID)
}

// IssueLoginCookies lets other verified login methods reuse the existing session format.
func IssueLoginCookies(c *gin.Context, userID int64) error {
	jwt, err := middleware.GenJwtToken(userID, time.Now())
	if err != nil {
		return err
	}
	domain := getCookieDomain()
	middleware.SetJwtToken(c, jwt)
	c.SetCookie("uid", fmt.Sprint(userID), 86400*30, "/", domain, false, true)
	c.SetCookie("token", uuid.NewString(), 86400*30, "/", domain, false, true)
	return nil
}

func hashPassword(password string) (string, error) {
	pwd := strings.TrimSpace(password)
	if pwd == "" {
		return "", nil
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func comparePassword(stored, input string) bool {
	s := strings.TrimSpace(stored)
	i := strings.TrimSpace(input)
	if s == "" || i == "" {
		return false
	}
	if strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(s), []byte(i)) == nil
	}
	return s == i
}

func optionalRegistrationCompany(value string) (string, bool) {
	name := strings.TrimSpace(value)
	return name, name != ""
}

func (s *svcImpl) SendSmsCode(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.SendSmsCodeReq
	)
	_ = c.ShouldBindQuery(&req)
	if strings.TrimSpace(req.Mobile) == "" {
		_ = c.ShouldBind(&req)
	}
	req.Mobile = strings.TrimSpace(req.Mobile)
	req.Scene = strings.TrimSpace(strings.ToLower(req.Scene))
	if req.Scene == "" {
		req.Scene = "login"
	}
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	ctx := context.Background()

	// 频率限制检查（Redis）
	rl, err := s.otpStore.CheckRateLimit(ctx, req.Scene, req.Mobile)
	if err != nil {
		logger.Warnw("SendSmsCode --> 频率限制检查失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "发送失败", nil)
		return
	}
	if rl {
		handler.SendNormalResp(c, errorCodeOtpTooFast, errorMsgOtpTooFast, nil)
		return
	}

	code, err := genOtpCode()
	if err != nil {
		logger.Warnw("SendSmsCode --> 生成验证码失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "发送失败", nil)
		return
	}

	// 存储验证码到 Redis（5分钟有效）
	if err := s.otpStore.Set(ctx, req.Scene, req.Mobile, code); err != nil {
		logger.Warnw("SendSmsCode --> 存储验证码失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "发送失败", nil)
		return
	}

	// 设置频率限制（1分钟冷却）
	if err := s.otpStore.SetRateLimit(ctx, req.Scene, req.Mobile); err != nil {
		logger.Warnw("SendSmsCode --> 设置频率限制失败", "err", err)
	}

	// 调用 SMS Provider 发送（noop 模式仅打日志）
	if err := s.smsProvider.Send(ctx, req.Mobile, code, req.Scene); err != nil {
		logger.Warnw("SendSmsCode --> 短信发送失败", "err", err)
	}

	data := map[string]any{
		"expireSeconds": 300,
	}
	if !isProdEnv() {
		data["debugCode"] = code
	}
	handler.SendOKResp(c, data)
}

func (s *svcImpl) Register(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.RegisterReq
	)

	_ = c.ShouldBindQuery(&req)
	if strings.TrimSpace(req.Mobile) == "" {
		_ = c.ShouldBind(&req)
	}
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Mobile = strings.TrimSpace(req.Mobile)
	req.SmsCode = strings.TrimSpace(req.SmsCode)
	if err := req.Valid(c); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}

	ok, err := s.verifyOtp(c, "register", req.Mobile, req.SmsCode)
	if err != nil {
		logger.Warnw("Register --> 校验验证码失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, errorMsgRegister, nil)
		return
	}
	if !ok {
		handler.SendNormalResp(c, errorCodeOtpNotMatch, errorMsgOtpNotMatch, nil)
		return
	}

	// 检查手机号是否已注册（直接查 user 表）
	existUser, err := s.user.GetUserByMobile(c, req.Mobile)
	if err != nil {
		logger.Warnw("Register --> 查询用户失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, "查询用户失败", nil)
		return
	}
	if existUser != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "手机号已注册", nil)
		return
	}

	hashed, err := hashPassword(req.Password)
	if err != nil {
		logger.Warnw("Register --> 密码加密失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, errorMsgRegister, nil)
		return
	}

	q := query.Use(storage.GetDB())
	tx := q.Begin()
	now := time.Now().Unix()
	defer func() {
		if recover() != nil {
			_ = tx.Rollback()
		}
	}()

	companyDo := tx.Company.WithContext(c)
	companyID := int32(0)
	createdNewCompany := false
	if companyName, hasCompany := optionalRegistrationCompany(req.CompanyName); hasCompany {
		company, companyErr := companyDo.Where(tx.Company.Name.Eq(companyName)).First()
		if companyErr != nil && !errors.Is(companyErr, gorm.ErrRecordNotFound) {
			_ = tx.Rollback()
			logger.Warnw("Register --> 查询公司失败", "err", companyErr)
			handler.SendNormalResp(c, entity.ErrCodeDBRead, errorMsgRegister, nil)
			return
		}
		if errors.Is(companyErr, gorm.ErrRecordNotFound) || company == nil {
			company = &model.Company{
				Name:           companyName,
				ApproveAdminID: 0,
				ApplyUserID:    0,
				CreateTime:     now,
				UpdateTime:     now,
				Status:         0,
				DocNum:         0,
				LimitDocNum:    10000,
				OwnerID:        0,
			}
			if createErr := companyDo.Create(company); createErr != nil {
				if strings.Contains(createErr.Error(), "Duplicate entry") || strings.Contains(createErr.Error(), "Error 1062") {
					exist, existErr := companyDo.Where(tx.Company.Name.Eq(companyName)).First()
					if existErr != nil || exist == nil {
						_ = tx.Rollback()
						logger.Warnw("Register --> 查询重复公司失败", "err", existErr)
						handler.SendNormalResp(c, entity.ErrCodeDBRead, errorMsgRegister, nil)
						return
					}
					company = exist
				} else {
					_ = tx.Rollback()
					logger.Warnw("Register --> 创建公司失败", "err", createErr)
					handler.SendNormalResp(c, entity.ErrCodeDBWrite, errorMsgRegister, nil)
					return
				}
			} else {
				createdNewCompany = true
			}
		}
		companyID = company.ID
	}

	user := &model.User{
		CompanyID:     companyID,
		Nickname:      req.Nickname,
		Username:      req.Mobile,
		Mobile:        req.Mobile,
		Password:      hashed,
		Status:        entity.CommonStatusAvailable,
		CreateTime:    now,
		UpdateTime:    now,
		ViewedVersion: "",
		Role:          entity.RoleUser,
		AvatarFile:    "",
		Email:         "",
	}
	if createErr := tx.User.WithContext(c).Create(user); createErr != nil {
		_ = tx.Rollback()
		logger.Warnw("Register --> 创建用户失败", "err", createErr)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, errorMsgRegister, nil)
		return
	}

	if createdNewCompany {
		_, err = companyDo.Where(tx.Company.ID.Eq(companyID)).Updates(map[string]any{
			"owner_id":      user.UserID,
			"apply_user_id": user.UserID,
			"update_time":   now,
		})
		if err != nil {
			_ = tx.Rollback()
			logger.Warnw("Register --> 更新公司负责人失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeDBWrite, errorMsgRegister, nil)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Warnw("Register --> 提交事务失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBWrite, errorMsgRegister, nil)
		return
	}

	if err := s.issueLoginCookies(c, user.UserID); err != nil {
		logger.Warnw("Register --> 种票失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "登录失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}
