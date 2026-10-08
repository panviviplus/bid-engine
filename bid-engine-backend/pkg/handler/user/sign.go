package user

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	skbcfg "bid-engine/pkg/config"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/middleware"
)

const (
	errorCodeLogin           = 10006 // 用户登录失败
	errorMsgLogin            = "登录失败，请稍后重试"
	errorCodeSmsCodeNotMatch = 10004            // 验证码不匹配
	errorMsgSmsCodeNotMatch  = "验证码不匹配，请确认后再输入" // 验证码不匹配
	errorCodePasswordNotSet  = 10022
	errorMsgPasswordNotSet   = "未设置密码，请使用验证码登录"
	errorCodeMobileNotExist  = 10023
	errorMsgMobileNotExist   = "手机号未注册，请先注册"
)

func (s *svcImpl) Login(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.LoginReq
	)

	_ = c.ShouldBindQuery(&req)
	_ = c.ShouldBind(&req)
	logger.Debugw("Login --> 登录参数绑定成功：", "手机号", req.Mobile)

	if err := req.Valid(c); err != nil {
		logger.Errorw("Login --> 登录参数校验不通过：", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	req.Mobile = strings.TrimSpace(req.Mobile)
	req.SmsCode = strings.TrimSpace(req.SmsCode)
	req.Password = strings.TrimSpace(req.Password)

	loginType := strings.TrimSpace(strings.ToLower(req.LoginType))
	if loginType == "" {
		loginType = "password"
	}

	dbUser, err := s.user.GetUserByMobile(c, req.Mobile)
	if err != nil {
		logger.Warnw("Login --> 手机号查询user表记录失败：", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeDBRead, err.Error(), nil)
		return
	}
	if dbUser == nil || dbUser.Mobile == "" {
		logger.Warnw("Login --> 用户不存在！", "手机号", req.Mobile)
		handler.SendNormalResp(c, errorCodeMobileNotExist, errorMsgMobileNotExist, nil)
		return
	}
	if dbUser.Status == 0 {
		logger.Warnw("Login --> 用户状态为0，不可用状态。", "userID", dbUser.UserID)
		handler.SendNormalResp(c, errorCodeLogin, errorMsgLogin, nil)
		return
	}
	if loginType == "sms" {
		ok, err := s.verifyOtp(c, "login", req.Mobile, req.SmsCode)
		if err != nil {
			logger.Warnw("Login --> 校验验证码失败", "err", err)
			handler.SendNormalResp(c, entity.ErrCodeInternal, errorMsgLogin, nil)
			return
		}
		if !ok {
			handler.SendNormalResp(c, errorCodeSmsCodeNotMatch, errorMsgSmsCodeNotMatch, nil)
			return
		}
	} else {
		inputPwd := req.Password
		if inputPwd == "" {
			inputPwd = req.SmsCode
		}
		if strings.TrimSpace(dbUser.Password) == "" {
			handler.SendNormalResp(c, errorCodePasswordNotSet, errorMsgPasswordNotSet, nil)
			return
		}
		if !comparePassword(dbUser.Password, inputPwd) {
			logger.Warnw("Login --> 用户密码校验不通过", "mobile", req.Mobile)
			handler.SendNormalResp(c, errorCodeLogin, errorMsgLogin, nil)
			return
		}
	}

	if err := s.issueLoginCookies(c, dbUser.UserID); err != nil {
		logger.Warnw("Login --> 种票失败", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, "登录失败", nil)
		return
	}
	handler.SendOKResp(c, nil)
}

var (
	cookieDomain string
	onceUserIDs  sync.Once
)

func getCookieDomain() string {
	onceUserIDs.Do(func() {
		cookieDomain = skbcfg.Get("cookie_domain")
	})
	return cookieDomain
}

func (s *svcImpl) Logout(c *gin.Context) {
	domain := getCookieDomain()
	middleware.DeleteJwtToken(c)

	c.SetCookie("uid", "", -1, "/", domain, false, true)
	c.SetCookie("token", "", -1, "/", domain, false, true)

	handler.SendOKResp(c, nil)
}
