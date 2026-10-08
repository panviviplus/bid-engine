package feishu

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"bid-engine/lib/common/logtool"
	"bid-engine/pkg/handler/user"
	feishurepo "bid-engine/pkg/repo/feishu"
	redisrepo "bid-engine/pkg/repo/redis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const challengeTTL = 5 * time.Minute
const loginCookie = "feishu_login_challenge"
const bindCookie = "feishu_bind_challenge"

var errChallengeExpired = errors.New("一次性码已过期，请重新获取")

type challenge struct {
	Purpose   string `json:"purpose"`
	UserID    int64  `json:"user_id"`
	Status    string `json:"status"`
	OpenID    string `json:"open_id"`
	TenantKey string `json:"tenant_key"`
}

type Service struct {
	appID         string
	appSecret     string
	publicBaseURL string
	redis         *redis.Client
	repo          *feishurepo.Repository
	logger        *zap.SugaredLogger
}

var once sync.Once
var instance *Service

func GetInstance() *Service {
	once.Do(func() {
		instance = &Service{
			appID:         strings.TrimSpace(os.Getenv("FEISHU_APP_ID")),
			appSecret:     strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET")),
			publicBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("FEISHU_PUBLIC_BASE_URL")), "/"),
			redis:         redisrepo.GetInstance().Client(),
			repo:          feishurepo.NewRepository(),
			logger:        logtool.GetLogger().Sugar(),
		}
	})
	return instance
}

func (s *Service) Enabled() bool { return s.appID != "" && s.appSecret != "" }

func (s *Service) botURL() string {
	return "https://applink.feishu.cn/client/bot/open?appId=" + s.appID
}

func challengeKey(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "feishu:challenge:" + hex.EncodeToString(hash[:])
}

func codeKey(code string) string { return "feishu:code:" + code }

func newToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

func setChallengeCookie(c *gin.Context, name, token string, maxAge int) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (s *Service) startChallenge(c *gin.Context, purpose string, userID int64) (gin.H, error) {
	if !s.Enabled() {
		return nil, errors.New("飞书接入尚未配置")
	}
	ipHash := sha256.Sum256([]byte(c.ClientIP()))
	rateKey := "feishu:challenge:rate:" + hex.EncodeToString(ipHash[:12])
	count, err := s.redis.Incr(c.Request.Context(), rateKey).Result()
	if err != nil {
		return nil, err
	}
	if count == 1 {
		_ = s.redis.Expire(c.Request.Context(), rateKey, 5*time.Minute).Err()
	}
	if count > 10 {
		return nil, errors.New("一次性码获取过于频繁，请稍后再试")
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	item := challenge{Purpose: purpose, UserID: userID, Status: "waiting"}
	raw, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	key := challengeKey(token)
	if err := s.redis.Set(c.Request.Context(), key, raw, challengeTTL).Err(); err != nil {
		return nil, err
	}
	var code string
	for i := 0; i < 6; i++ {
		code, err = newCode()
		if err != nil {
			return nil, err
		}
		ok, setErr := s.redis.SetNX(c.Request.Context(), codeKey(code), token, challengeTTL).Result()
		if setErr != nil {
			return nil, setErr
		}
		if ok {
			break
		}
		code = ""
	}
	if code == "" {
		return nil, errors.New("一次性码生成失败")
	}
	name := loginCookie
	if purpose == "bind" {
		name = bindCookie
	}
	setChallengeCookie(c, name, token, int(challengeTTL.Seconds()))
	command := "登录 "
	if purpose == "bind" {
		command = "绑定 "
	}
	return gin.H{"code": code, "command": command + code, "bot_url": s.botURL(),
		"expires_at": time.Now().Add(challengeTTL).Unix()}, nil
}

func (s *Service) currentChallenge(c *gin.Context, purpose string, userID int64) (*challenge, string, error) {
	name := loginCookie
	if purpose == "bind" {
		name = bindCookie
	}
	token, err := c.Cookie(name)
	if err != nil || token == "" {
		return nil, "", errChallengeExpired
	}
	raw, err := s.redis.Get(c.Request.Context(), challengeKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, "", errChallengeExpired
	}
	if err != nil {
		return nil, "", err
	}
	var item challenge
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, "", err
	}
	if item.Purpose != purpose || (purpose == "bind" && item.UserID != userID) {
		return nil, "", errors.New("一次性码与当前账号不匹配")
	}
	return &item, token, nil
}

func (s *Service) completeChallenge(c *gin.Context, purpose string, userID int64) error {
	item, token, err := s.currentChallenge(c, purpose, userID)
	if err != nil {
		return err
	}
	if item.Status != "confirmed" || item.OpenID == "" {
		return errors.New("请先在飞书中发送一次性码")
	}
	removed, err := s.redis.GetDel(c.Request.Context(), challengeKey(token)).Bytes()
	if err != nil {
		return errors.New("一次性码已被使用，请重新获取")
	}
	var claimed challenge
	if err := json.Unmarshal(removed, &claimed); err != nil {
		return err
	}
	if claimed.Status != "confirmed" || claimed.Purpose != purpose || claimed.OpenID != item.OpenID {
		return errors.New("一次性码状态已变更，请重新获取")
	}
	if purpose == "bind" {
		err = s.repo.Bind(c.Request.Context(), s.appID, item.OpenID, item.TenantKey, userID)
		setChallengeCookie(c, bindCookie, "", -1)
		return err
	}
	loginUserID, err := s.repo.LoginOrRegister(c.Request.Context(), s.appID, item.OpenID, item.TenantKey)
	if err != nil {
		return err
	}
	if err := user.IssueLoginCookies(c, loginUserID); err != nil {
		return err
	}
	setChallengeCookie(c, loginCookie, "", -1)
	return nil
}

func (s *Service) confirmCode(ctx context.Context, purpose, code, openID, tenantKey, messageID string) error {
	if purpose != "login" && purpose != "bind" {
		return nil
	}
	if len(code) != 8 {
		return nil
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return nil
		}
	}
	if openID == "" || messageID == "" {
		return nil
	}
	limitKey := "feishu:invalid:" + openID
	n, err := s.redis.Get(ctx, limitKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if n >= 5 {
		return nil
	}
	dedupeKey := "feishu:message:" + messageID
	claimed, err := s.redis.SetNX(ctx, dedupeKey, "1", 24*time.Hour).Result()
	if err != nil || !claimed {
		return err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = s.redis.Del(context.Background(), dedupeKey).Err()
		}
	}()
	token, err := s.redis.Get(ctx, codeKey(code)).Result()
	if errors.Is(err, redis.Nil) {
		count, incrErr := s.redis.Incr(ctx, limitKey).Result()
		if incrErr == nil && count == 1 {
			_ = s.redis.Expire(ctx, limitKey, 5*time.Minute).Err()
		}
		succeeded = true
		return incrErr
	}
	if err != nil {
		return err
	}
	key := challengeKey(token)
	raw, err := s.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		succeeded = true
		return nil
	}
	if err != nil {
		return err
	}
	var item challenge
	if err := json.Unmarshal(raw, &item); err != nil {
		return err
	}
	if item.Purpose != purpose || item.Status != "waiting" {
		succeeded = true
		return nil
	}
	claimedToken, err := s.redis.GetDel(ctx, codeKey(code)).Result()
	if errors.Is(err, redis.Nil) {
		succeeded = true
		return nil
	}
	if err != nil {
		return err
	}
	if claimedToken != token {
		succeeded = true
		return nil
	}
	item.Status, item.OpenID, item.TenantKey = "confirmed", openID, tenantKey
	raw, err = json.Marshal(item)
	if err != nil {
		return err
	}
	ttl, err := s.redis.TTL(ctx, key).Result()
	if err != nil {
		return err
	}
	if ttl <= 0 {
		succeeded = true
		return nil
	}
	if err := s.redis.Set(ctx, key, raw, ttl).Err(); err != nil {
		return err
	}
	succeeded = true
	return nil
}
