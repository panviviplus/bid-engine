package middleware

import (
	"os"
	"strings"

	"bid-engine/pkg/db/model"
	"bid-engine/lib/common/logtool"

	"github.com/gin-gonic/gin"

	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
	"bid-engine/pkg/repo/user"
)

// AuthMiddleware 基于JWT的认证中间件
func AuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 本地开发模式：注入超级管理员身份，免登录
		if os.Getenv("LOCAL_DEV") == "true" {
			entity.SetUserToCtx(c, &model.User{
				UserID:   entity.SuperAdminUserID,
				Username: "local-dev",
				Nickname: "本地开发",
				Role:     1, // 系统管理员
				Status:   1, // 可用
			})
			c.Next()
			return
		}

		userIDInt := getUserID(c)
		if userIDInt == 0 {
			logtool.GetLogger().Sugar().With(entity.Ctx(c)...).Infow(
				"认证失败，userID为0（cookie缺失或token解析失败）",
				"path", c.Request.URL.Path,
				"cookie_key", jwtIdentifyKey,
				"host", c.Request.Host,
			)
			return
		}
		// 获取用户角色和所在公司
		uInfo, _ := user.GetInstance().GetUser(c, userIDInt)
		if uInfo == nil {
			handler.SendLoginResp(c)
			c.Abort()
			return
		} else {
			if uInfo.Status == entity.CommonStatusUnavailable {
				handler.SendNormalResp(c, entity.ErrCodeUserLocked, "用户已被锁定", nil)
				c.Abort()
				return
			}
		}
		entity.SetUserToCtx(c, uInfo)
		c.Next()
	}
}

func getUserID(c *gin.Context) int64 {
	token, err := c.Cookie(jwtIdentifyKey)
	if err != nil {
		handler.SendLoginResp(c)
		c.Abort()
		return 0
	}
	tokenUser, err := ParseJwtToken(token)
	if err != nil {
		handler.SendLoginResp(c)
		c.Abort()
		return 0
	}
	return tokenUser.UserID
}

func OpenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Authorization header is required", nil)
			c.Abort()
			return
		}
		// 检查前缀是否是 Bearer
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Authorization format must be Bearer {token}", nil)
			c.Abort()
			return
		}
		token := parts[1]
		apiKey := getOpenAPIKey()
		// 未配置 OpenAPI 令牌时一律拒绝，避免空令牌被无条件放行
		if apiKey == "" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "OpenAPI access is disabled", nil)
			c.Abort()
			return
		}
		if token != apiKey {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Invalid token", nil)
			c.Abort()
			return
		}
		// 校验通过，继续处理请求
		c.Next()
	}
}

// SpecialOpenAuth recall openapi专用中间件，默认注入超管身份
func SpecialOpenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Authorization header is required", nil)
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Authorization format must be Bearer {token}", nil)
			c.Abort()
			return
		}
		token := parts[1]
		apiKey := getOpenAPIKey()
		// 未配置 OpenAPI 令牌时一律拒绝，避免空令牌被无条件放行
		if apiKey == "" {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "OpenAPI access is disabled", nil)
			c.Abort()
			return
		}
		if token != apiKey {
			handler.SendNormalResp(c, entity.ErrCodeNoAuth, "Invalid token", nil)
			c.Abort()
			return
		}

		// 默认注入超级管理员身份到上下文（userId=1000000）
		const uid = entity.SuperAdminUserID
		uInfo, _ := user.GetInstance().GetUser(c, uid)
		if uInfo != nil {
			// 若库里存在该用户，校验状态并注入
			if uInfo.Status == entity.CommonStatusUnavailable {
				handler.SendNormalResp(c, entity.ErrCodeUserLocked, "用户已被锁定", nil)
				c.Abort()
				return
			}
			entity.SetUserToCtx(c, uInfo)
		} else {
			entity.SetUserToCtx(c, &model.User{
				UserID:   uid,
				Username: "openapi",
			})
		}

		c.Next()
	}
}
