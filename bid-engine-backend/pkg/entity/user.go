package entity

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"bid-engine/lib/common/config"
)

const (
	CurUserIdentityOfSystemAdmin  = "admin"
	CurUserIdentityOfCompanyOwner = "company_owner"
	CurUserIdentityOfCommonMember = "member"
)

// LoginReq 登录请求参数
type LoginReq struct {
	Mobile    string `json:"mobile" form:"mobile"`
	SmsCode   string `json:"sms_code" form:"sms_code"`
	Password  string `json:"password" form:"password"`
	LoginType string `json:"login_type" form:"login_type"`
	Ticket    string `json:"ticket" form:"ticket"`
	ClientID  string `json:"clientID" form:"clientID"`
}

// Valid 参数合法性校验
func (a *LoginReq) Valid(c *gin.Context) error {
	if a.Mobile == "" {
		return fmt.Errorf("账号不能为空")
	}
	loginType := strings.TrimSpace(strings.ToLower(a.LoginType))
	if loginType == "" {
		loginType = "password"
	}
	if loginType == "sms" {
		if a.SmsCode == "" {
			return fmt.Errorf("验证码不能为空")
		}
		return nil
	}
	if a.Password == "" && a.SmsCode == "" {
		return fmt.Errorf("密码不能为空")
	}
	return nil
}

type SendSmsCodeReq struct {
	Mobile string `json:"mobile" form:"mobile"`
	Scene  string `json:"scene" form:"scene"`
}

func (a *SendSmsCodeReq) Valid(c *gin.Context) error {
	if a.Mobile == "" {
		return fmt.Errorf("手机号不能为空")
	}
	return nil
}

type RegisterReq struct {
	CompanyName string `json:"company_name" form:"company_name"`
	Nickname    string `json:"nickname" form:"nickname"`
	Mobile      string `json:"mobile" form:"mobile"`
	SmsCode     string `json:"sms_code" form:"sms_code"`
	Password    string `json:"password" form:"password"`
}

func (a *RegisterReq) Valid(c *gin.Context) error {
	if strings.TrimSpace(a.Mobile) == "" {
		return fmt.Errorf("手机号不能为空")
	}
	if strings.TrimSpace(a.SmsCode) == "" {
		return fmt.Errorf("验证码不能为空")
	}
	if strings.TrimSpace(a.Password) == "" {
		return fmt.Errorf("密码不能为空")
	}
	if err := ValidatePassword(a.Password); err != nil {
		return err
	}
	return nil
}

var (
	deployType     string
	onceDeployType sync.Once
)

func DeployLocal() bool {
	onceDeployType.Do(func() {
		deployType = config.GetConfig().GetProperty("deploy_type")
	})
	if deployType == "local" {
		return true
	}
	return false
}

type AuthLoginReq struct {
	Code string `form:"code"`
	Stat string `form:"stat"`
}

func (a *AuthLoginReq) Valid(c *gin.Context) error {
	_ = c.ShouldBind(&a)
	if a.Code == "" {
		return fmt.Errorf("code 空")
	}
	return nil
}

// AuthUser 授权用户通用信息
type AuthUser struct {
	ID       string `json:"id"`       // 用户id
	Username string `json:"username"` // 用户名
	NickName string `json:"nickName"` // 昵称
	PhoneNum string `json:"phoneNum"` // 手机号
	Email    string `json:"email"`    // 邮箱
	Avatar   string `json:"avatar"`   // 头像
	WorkNum  string `json:"workNum"`  //工号
	Company  string `json:"company"`  //公司
}

// GetIAMUserReq 根据SN码获取IAM用户信息请求
type GetIAMUserReq struct {
	Username string `uri:"username" comment:"SN码"` // SN码（工号）
}

// Valid 参数合法性校验
func (a *GetIAMUserReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&a)
	if a.Username == "" {
		return fmt.Errorf("SN码不能为空")
	}
	return nil
}

// IAMUser IAM用户信息
type IAMUser struct {
	Username    string `json:"username" comment:"SN码,工号"`
	Nickname    string `json:"nickname" comment:"姓名"`
	Mobile      string `json:"mobile" comment:"手机号码"`
	CompanyName string `json:"company_name" comment:"公司名称"`
}

// MenuNode 菜单节点
type MenuNode struct {
	ID       int32      `json:"id"`        // 权限配置ID
	ParentID int32      `json:"parent_id"` // 父权限ID
	Name     string     `json:"name"`      // 权限名称
	Icon     string     `json:"icon"`      //菜单icon
	Href     string     `json:"href"`      //菜单icon路径
	Children []MenuNode `json:"children"`  // 子菜单
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	Name       string `json:"name"`        // 用户姓名，即user表的nickname字段
	UserID     int64  `json:"user_id"`     // 用户ID
	Mobile     string `json:"mobile"`      // 手机号
	CompanyId  int64  `json:"companyId"`   // 所属公司ID
	Company    string `json:"company"`     // 公司名
	Status     int32  `json:"status"`      // 状态
	DocNum     int32  `json:"doc_num"`     // 文档数量
	UpdateTime int64  `json:"update_time"` // 更新时间
	LibraryID  int32  `json:"library_id"`  // 知识库ID
	CanUpload  bool   `json:"can_upload"`  // 公司还能否上传文件
	UserRole   string `json:"userRole"`    //当前用户的角色，枚举：admin\company_owner\member
	Email      string `json:"email"`       // 邮箱
	AvatarURL  string `json:"avatar_url"`  // 头像可访问URL（通常为短期签名URL）
	AvatarFile string `json:"avatar_file"` // 头像OSS objectKey，前端可用于自行拼接头像URL
}

// ChangePasswordReq 修改密码请求
type ChangePasswordReq struct {
	OldPassword     string `json:"old_password" form:"old_password"`
	NewPassword     string `json:"new_password" form:"new_password"`
	ConfirmPassword string `json:"confirm_password" form:"confirm_password"`
}

// Valid 修改密码参数校验
func (a *ChangePasswordReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(a); err != nil {
		return fmt.Errorf("参数错误")
	}
	if strings.TrimSpace(a.OldPassword) == "" {
		return fmt.Errorf("旧密码不能为空")
	}
	if err := ValidatePassword(a.NewPassword); err != nil {
		return err
	}
	if a.NewPassword != a.ConfirmPassword {
		return fmt.Errorf("两次输入的新密码不一致")
	}
	return nil
}

type UpdateUserProfileReq struct {
	Nickname *string `json:"nickname" form:"nickname"`
	Mobile   *string `json:"mobile" form:"mobile"`
	Email    *string `json:"email" form:"email"`
}

func (a *UpdateUserProfileReq) Valid(c *gin.Context) error {
	if err := c.ShouldBind(a); err != nil {
		return fmt.Errorf("参数错误:%s", err.Error())
	}
	if a.Mobile != nil {
		m := strings.TrimSpace(*a.Mobile)
		if m != "" {
			if len(m) != 11 {
				return fmt.Errorf("手机号码必须为11位")
			}
			for _, r := range m {
				if r < '0' || r > '9' {
					return fmt.Errorf("手机号码格式不正确")
				}
			}
		}
	}
	return nil
}

type ListCompanyUserReq struct {
	CompanyId int64 `json:"companyId" form:"companyId"` // 公司ID
}

// Valid 参数合法性校验
func (a *ListCompanyUserReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&a)
	if a.CompanyId == 0 {
		return fmt.Errorf("公司ID不能为空")
	}
	return nil
}

// UserBasicInfo 用户基本信息返回结构体
type UserBasicInfo struct {
	UserID      int64  `json:"user_id"`      // 用户ID
	Name        string `json:"name"`         // 用户姓名
	Mobile      string `json:"mobile"`       // 手机号
	Username    string `json:"username"`     // 用户名
	Status      int32  `json:"status"`       // 用户/账号 状态
	CompanyID   int64  `json:"company_id"`   // 公司ID
	CompanyName string `json:"company_name"` // 公司名
}
