package entity

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

type GetUsersReq struct {
	UserIDs []int64 `json:"user_ids"`
}

// Valid 参数合法性校验
func (a *GetUsersReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if len(a.UserIDs) == 0 {
		return fmt.Errorf("user_ids 不能为空")
	}
	return nil
}

type SimpleUser struct {
	UserID     int64  `json:"user_id"`     // 管理main表用户id
	CompanyID  int32  `json:"company_id"`  // 公司ID
	Role       int32  `json:"role"`        // 角色,0:普通用户 1:系统管理员
	Nickname   string `json:"nickname"`    // 昵称
	Username   string `json:"username"`    // 用户名
	Status     int32  `json:"status"`      // 状态, 0:不可用 1:可用
	CreateTime int64  `json:"create_time"` // 添加时间
	UpdateTime int64  `json:"update_time"` // 更新时间
}
