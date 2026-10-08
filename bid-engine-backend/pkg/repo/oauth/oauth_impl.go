package oauth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"bid-engine/pkg/entity"
	"bid-engine/lib/common/logtool"

	"github.com/gin-gonic/gin"
)

const (
	authorizeAPI = "" //获取登录地址接口
	tokenAPI     = "" //获取access_token接口
	profileAPI   = "" //获取用户信息接口
)

// TokenResp 授权获取token返回结果
type TokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ProfileAttributes 用户资料详情
type ProfileAttributes struct {
	Company        string `json:"company"`        // 公司
	CredentialType string `json:"credentialType"` // 邮箱
	Email          string `json:"email"`          // 邮箱
	ID             string `json:"id"`             // 1
	RealName       string `json:"realname"`       // Zadmin
	UserName       string `json:"userName"`       // 用户姓名 admin
}

// ProfileResp 授权获取用户资料返回结果
type ProfileResp struct {
	ID       string `json:"id"`       // 1
	RealName string `json:"realname"` // Zadmin
	Phone    string `json:"phone"`
}

func (s *svcImpl) GetLoginURL() string {
	return fmt.Sprintf("%s/%s?response_type=code&client_id=%s&redirect_uri=%s",
		s.baseURL, authorizeAPI, s.clientID, s.redirectURI)
}

// GetOAuthToken 获取oauth认证token
func (s *svcImpl) GetOAuthToken(c *gin.Context, code string) (string, error) {
	logger := s.logger.With(logtool.Ctx(c)...)
	logger.Infow("获取OAuth的Token", "code", code)
	var (
		reqURL = s.baseURL + "/" + tokenAPI
		now    = time.Now()
	)
	// 构造请求参数
	params := url.Values{}
	params.Add("grant_type", "authorization_code")
	params.Add("client_id", s.clientID)
	params.Add("client_secret", s.secret)
	params.Add("code", code)
	params.Add("redirect_uri", s.redirectURI)
	reqURL += "?" + params.Encode()
	resp, err := s.httpClient.Get(reqURL)
	logger.Infow("accessToken 调用结束",
		"reqURL", reqURL, "cost", time.Since(now).Milliseconds())
	if err != nil {
		logger.Warnw("accessToken 调用失败", "err", err.Error())
		return "", err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Infow("accessToken 返回结果", "body", string(body))
	var tokenResp TokenResp
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		logger.Warnw("accessToken 解析错误", "err", err.Error(), "body", string(body))
		return "", err
	}
	return tokenResp.AccessToken, nil
}

func (s *svcImpl) GetOAuthProfile(c *gin.Context, token string) (*entity.AuthUser, error) {
	logger := s.logger.With(logtool.Ctx(c)...)
	logger.Infow("获取OAuth的Profile", "token", token)
	var (
		baseURL   = s.baseURL + "/" + profileAPI
		params    = url.Values{}
		oAuthResp ProfileResp
		now       = time.Now()
	)
	// 构造请求参数
	reqURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	params.Set("access_token", token)
	urlPath := reqURL.String() + "?" + params.Encode()
	logger.Infow("请求完整url", "urlPath", urlPath)
	// 发送http请求
	req, _ := http.NewRequest("GET", urlPath, nil)
	// 发送http请求
	resp, err := s.httpClient.Do(req)
	logger.Infow("profile 返回结果", "cost", time.Since(now).Milliseconds())
	if err != nil {
		logger.Warnw("profile 调用失败", "err", err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Infow("profile 返回结果", "body", string(body))
	if err = json.Unmarshal(body, &oAuthResp); err != nil {
		logger.Warnw("profile 结果解析失败", "err", err.Error())
		return nil, err
	}
	return toAuthUser(oAuthResp), nil
}

func toAuthUser(profile ProfileResp) *entity.AuthUser {
	if profile.ID == "" {
		return nil
	}
	return &entity.AuthUser{
		ID:       profile.ID,
		NickName: profile.RealName,
		PhoneNum: profile.Phone,
		Username: profile.ID,
	}
}
