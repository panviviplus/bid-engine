package oauth

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	skbcfg "bid-engine/pkg/config"

	"bid-engine/pkg/entity"
	"bid-engine/lib/common/logtool"
)

var (
	instance Service
	once     sync.Once
)

// Service 用户服务接口定义
type Service interface {
	// GetLoginURL 获取登录地址
	GetLoginURL() string

	// GetOAuthToken 获取oauth认证token
	GetOAuthToken(c *gin.Context, code string) (string, error)

	// GetOAuthProfile  授权获取用户资料
	GetOAuthProfile(c *gin.Context, token string) (*entity.AuthUser, error)
}

// svcImpl 用户操作相关
type svcImpl struct {
	logger      *zap.SugaredLogger
	baseURL     string
	clientID    string
	secret      string
	redirectURI string

	httpClient *http.Client
}

// GetInstance 创建Term的实例
func GetInstance() Service {
	once.Do(func() {
		tr := &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{
			Timeout:   15 * time.Second,
			Transport: tr,
		}
		instance = &svcImpl{
			logger:      logtool.GetLogger().Sugar(),
			baseURL:     skbcfg.Get("properties.oauth_base_url"),
			clientID:    skbcfg.Get("properties.oauth_client_id"),
			secret:      skbcfg.Get("properties.oauth_secret"),
			redirectURI: skbcfg.Get("properties.oauth_redirect_uri"),
			httpClient:  client,
		}
	})
	return instance
}
