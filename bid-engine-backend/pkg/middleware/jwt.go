package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

const (
	jwtExpireDuration = time.Hour * 24 * 30
	jwtSecret         = "bid-engine-kPL2mN9qR3tW7zY4bX6vC1dF5gH8j"
	jwtDefaultKey     = "bid-engine-authorization"
)

// UserClaims jwt保存的用户信息
type UserClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// GenJwtToken 生成JWT
func GenJwtToken(userID int64, now time.Time) (string, error) {
	// 创建一个我们自己的声明
	c := UserClaims{
		userID,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtExpireDuration)),
			Issuer:    "bid-engine", // 签发人
		},
	}
	// 使用指定的签名方法创建签名对象
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	// 使用指定的secret签名并获得完整的编码后的字符串token
	return token.SignedString([]byte(jwtSecret))
}

// ParseJwtToken 解析JWT
func ParseJwtToken(tokenVal string) (*UserClaims, error) {
	// 解析token
	token, err := jwt.ParseWithClaims(tokenVal, &UserClaims{}, func(token *jwt.Token) (i interface{}, err error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid { // 校验token
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// SetJwtToken 设置jwt 到cookie
func SetJwtToken(c *gin.Context, tokenVal string) {
	domain := getCookieDomain()
	c.SetCookie(jwtIdentifyKey, tokenVal, 86400*30, "/", domain, false, true)
}

func getCookieDomain() string {
	return cookieDomain
}

// DeleteJwtToken 删除jwt
func DeleteJwtToken(c *gin.Context) {
	domain := getCookieDomain()
	c.SetCookie(jwtIdentifyKey, "", -1, "/", domain, false, true)
}
