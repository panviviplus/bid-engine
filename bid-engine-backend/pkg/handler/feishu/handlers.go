package feishu

import (
	"errors"
	"strings"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	feishurepo "bid-engine/pkg/repo/feishu"
	"github.com/gin-gonic/gin"
)

func (s *Service) LoginChallenge(c *gin.Context) { s.createChallenge(c, "login", 0) }
func (s *Service) BindingChallenge(c *gin.Context) {
	s.createChallenge(c, "bind", entity.GetUserIDFromCtx(c))
}

func (s *Service) createChallenge(c *gin.Context, purpose string, userID int64) {
	if purpose == "bind" && userID <= 0 {
		handler.SendNormalResp(c, entity.ErrCodeNotLogin, "请先登录", nil)
		return
	}
	data, err := s.startChallenge(c, purpose, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	controls := gin.H{"code": data["code"], "command": data["command"], "bot_url": data["bot_url"], "expires_at": data["expires_at"]}
	handler.SendOKResp(c, controls)
}

func (s *Service) LoginStatus(c *gin.Context) { s.challengeStatus(c, "login", 0) }
func (s *Service) BindingStatus(c *gin.Context) {
	s.challengeStatus(c, "bind", entity.GetUserIDFromCtx(c))
}

func (s *Service) challengeStatus(c *gin.Context, purpose string, userID int64) {
	c.Header("Cache-Control", "no-store")
	item, _, err := s.currentChallenge(c, purpose, userID)
	if errors.Is(err, errChallengeExpired) {
		handler.SendOKResp(c, gin.H{"status": "expired"})
		return
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	handler.SendOKResp(c, gin.H{"status": item.Status})
}

func (s *Service) LoginComplete(c *gin.Context) { s.finishChallenge(c, "login", 0) }
func (s *Service) BindingComplete(c *gin.Context) {
	s.finishChallenge(c, "bind", entity.GetUserIDFromCtx(c))
}

func (s *Service) finishChallenge(c *gin.Context, purpose string, userID int64) {
	if err := s.completeChallenge(c, purpose, userID); err != nil {
		s.fail(c, err)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *Service) BindingInfo(c *gin.Context) {
	uid := entity.GetUserIDFromCtx(c)
	bound, err := s.repo.BindingByUser(c.Request.Context(), s.appID, uid)
	if err != nil {
		s.fail(c, err)
		return
	}
	profile, err := s.repo.ExtraProfile(c.Request.Context(), uid)
	if err != nil {
		s.fail(c, err)
		return
	}
	out := gin.H{"bound": bound != nil, "notify_enabled": false,
		"contact_mobile": profile.ContactMobile, "company_display_name": profile.CompanyDisplayName}
	if bound != nil {
		out["notify_enabled"] = bound.NotifyEnabled == 1
	}
	handler.SendOKResp(c, out)
}

func (s *Service) Unbind(c *gin.Context) {
	if err := s.repo.Unbind(c.Request.Context(), s.appID, entity.GetUserIDFromCtx(c)); err != nil {
		s.fail(c, err)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *Service) SetNotify(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "请选择是否接收飞书提醒", nil)
		return
	}
	if err := s.repo.SetNotify(c.Request.Context(), s.appID, entity.GetUserIDFromCtx(c), *req.Enabled); err != nil {
		s.fail(c, err)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *Service) UpdateExtraProfile(c *gin.Context) {
	var req struct {
		ContactMobile      string `json:"contact_mobile"`
		CompanyDisplayName string `json:"company_display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.SendNormalResp(c, entity.ErrCodeParam, "资料格式错误", nil)
		return
	}
	req.ContactMobile = strings.TrimSpace(req.ContactMobile)
	req.CompanyDisplayName = strings.TrimSpace(req.CompanyDisplayName)
	if len([]rune(req.ContactMobile)) > 32 || len([]rune(req.CompanyDisplayName)) > 255 {
		handler.SendNormalResp(c, entity.ErrCodeParam, "资料内容过长", nil)
		return
	}
	if err := s.repo.SaveExtraProfile(c.Request.Context(), entity.GetUserIDFromCtx(c), req.ContactMobile, req.CompanyDisplayName); err != nil {
		s.fail(c, err)
		return
	}
	handler.SendOKResp(c, nil)
}

func (s *Service) fail(c *gin.Context, err error) {
	if errors.Is(err, feishurepo.ErrAlreadyBound) || errors.Is(err, feishurepo.ErrNoAlternateLogin) || errors.Is(err, feishurepo.ErrUserUnavailable) {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	if strings.Contains(err.Error(), "一次性码") || strings.Contains(err.Error(), "请先在飞书") || strings.Contains(err.Error(), "尚未配置") {
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	s.logger.Errorw("飞书接入请求失败", "path", c.Request.URL.Path, "err", err)
	handler.SendNormalResp(c, entity.ErrCodeInternal, "操作失败，请稍后重试", nil)
}
