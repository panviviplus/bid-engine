package entity

import (
	"fmt"
	"strings"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
)

const (
	// UserIDKey ctx中uid
	UserIDKey = "_userID"
	// UserInfoKey ctx中用户信息
	UserInfoKey = "_userInfo"

	// SuperAdminUserID 超级管理员
	SuperAdminUserID int64 = 1000000
)

// SetUserToCtx 设置用户id
func SetUserToCtx(c *gin.Context, u *model.User) {
	c.Set(UserInfoKey, u)
}

// GetUserFromCtx 获取用户信息
func GetUserFromCtx(c *gin.Context) *model.User {
	if v, exist := c.Get(UserInfoKey); exist {
		u, _ := v.(*model.User)
		if u != nil {
			return u
		}
	}
	return nil
}

// GetUsernameFromCtx 获取用户名称
func GetUsernameFromCtx(c *gin.Context) string {
	u := GetUserFromCtx(c)
	if u == nil {
		return ""
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}

// GetNickNameFromCtx 获取用户昵称
func GetNickNameFromCtx(c *gin.Context) string {
	if v, exist := c.Get(UserInfoKey); exist {
		u, _ := v.(*model.User)
		if u != nil {
			if u.Nickname != "" {
				return u.Nickname
			}
			return u.Username
		}
	}
	return ""
}

// GetUserIDFromCtx 获取用户id
func GetUserIDFromCtx(c *gin.Context) int64 {
	if v, exist := c.Get(UserInfoKey); exist {
		u, _ := v.(*model.User)
		if u != nil {
			return u.UserID
		}
	}
	return 0
}

// GetCompanyIDFromCtx 获取公司id
func GetCompanyIDFromCtx(c *gin.Context) int32 {
	if v, exist := c.Get(UserInfoKey); exist {
		u, _ := v.(*model.User)
		if u != nil {
			return u.CompanyID
		}
	}
	return 0
}

// GetRequestIDForGo 获取requestID
func GetRequestIDForGo(c *gin.Context) string {
	// 防御：上下文可能为nil
	if c == nil {
		return ""
	}
	// 1) 优先使用协程中手动设置的 _REQUEST_ID（不会依赖 c.Writer）
	if v, exist := c.Get("_REQUEST_ID"); exist {
		if requestID, ok := v.(string); ok && requestID != "" {
			return requestID
		}
	}
	// 2) 其次尝试从请求头读取（中间件通常也会写入 X-Request-ID），需确保 c.Request 非空
	if c.Request != nil {
		if h := c.GetHeader("X-Request-ID"); h != "" {
			return h
		}
		if h := c.GetHeader("X-Request-Id"); h != "" {
			return h
		}
	}
	// 3) 最后才调用 requestid.Get，但必须确保 c.Writer 非空，避免协程场景空指针
	if c.Writer != nil {
		if rid := requestid.Get(c); rid != "" {
			return rid
		}
	}
	return ""
}

// GetRequestID 获取requestID
func GetRequestID(c *gin.Context) string {
	if requestid.Get(c) != "" {
		return requestid.Get(c)
	}
	if v, exist := c.Get("_REQUEST_ID"); exist {
		requestID, _ := v.(string)
		return requestID
	}
	return ""
}

// SetRequestID 设置requestID
func SetRequestID(c *gin.Context, requestID string) {
	c.Set("_REQUEST_ID", requestID)
}

// Ctx 日志中添加ctx信息
func Ctx(ctx *gin.Context) []interface{} {
	var ret []interface{}
	//if traceID := GetRequestID(ctx); traceID != "" {
	//	ret = append(ret, "traceId", traceID)
	//}
	if traceID := GetRequestIDForGo(ctx); traceID != "" {
		ret = append(ret, "traceId", traceID)
	}

	if v, exist := ctx.Get("_SCRIPT"); exist {
		isScript, _ := v.(string)
		if isScript == "1" {
			ret = append(ret, "_SCRIPT", "1")
		}
	}
	if v, exist := ctx.Get("_DOC_ID"); exist {
		docID, _ := v.(string)
		if docID != "" {
			ret = append(ret, "_DOC_ID", docID)
		}
	}
	if userID := GetUserIDFromCtx(ctx); userID > 0 {
		ret = append(ret, UserIDKey, fmt.Sprint(userID))
	}
	return ret
}

// IsMiniprogram 判断是不是小程序
func IsMiniprogram(c *gin.Context) bool {
	return strings.Contains(c.Request.UserAgent(), "MicroMessenger")
}

// IsSuperAdmin 是不是超级管理员，即系统管理员
func IsSuperAdmin(c *gin.Context) bool {
	u := GetUserFromCtx(c)

	return u.Role == 1
}

// IsCompanyAdmin 是否是公司负责人
func IsCompanyAdmin(c *gin.Context) bool {
	// 超管，默认就有公司管理权限
	if IsSuperAdmin(c) {
		return true
	}
	return false
}

// IsCompanyOwner 当前用户是否是目标公司的负责人
func IsCompanyOwner(c *gin.Context, companyOwnerID int64) bool {
	// 超管，默认就有公司管理权限
	if IsSuperAdmin(c) {
		return true
	}

	user := GetUserFromCtx(c)
	if user == nil {
		return false
	}

	if user.UserID == companyOwnerID || IsCompanyAdmin(c) {
		return true
	}

	return false
}

