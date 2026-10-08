package entity

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const (
	ExampleLibraryName = "示例知识库：澜舟科技"
)

// AddCompanyReq 添加公司
type AddCompanyReq struct {
	Name   string `json:"name"`
	UserID int64  `json:"userId"`
}

// Valid 参数合法性校验
func (a *AddCompanyReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if a.Name == "" {
		return fmt.Errorf("公司名不能为空")
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户id不能为空")
	}
	return nil
}

type AddCompanyOwnerReq struct {
	CompanyId int64 `json:"companyId"`
	UserID    int64 `json:"userId"`
}

// Valid 参数合法性校验
func (a *AddCompanyOwnerReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	if a.CompanyId == 0 {
		return fmt.Errorf("公司id不能为空")
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户id不能为空")
	}
	return nil
}

type EditCompanyReq struct {
	Id     int64 `json:"id"`
	UserID int64 `json:"userId"`
}

// Valid 参数合法性校验
func (a *EditCompanyReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindJSON(&a)
	_ = c.ShouldBindUri(&a)
	if a.Id == 0 {
		return fmt.Errorf("公司ID不能为空")
	}
	if a.UserID == 0 {
		return fmt.Errorf("用户名不能为空")
	}
	return nil
}

// ListCompanyReq1 公司列表查询请求
type ListCompanyReq1 struct {
	PageSize int    `json:"page_size" form:"page_size"` // 每页数量
	PageNum  int    `json:"page_num" form:"page_num"`   // 页码
	Name     string `json:"name" form:"name"`           // 公司名称筛选
}

// Valid 参数合法性校验
func (l *ListCompanyReq1) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&l)
	if l.PageSize <= 0 {
		l.PageSize = 10 // 默认每页10条
	}
	if l.PageNum <= 0 {
		l.PageNum = 1 // 默认第1页
	}
	if l.PageSize > 50 {
		l.PageSize = 10 // 最大每页50条
	}
	return nil
}

// ListCompanyReq 公司列表查询请求
type ListCompanyReq struct {
	PageSize int    `json:"pageSize" form:"pageSize"` // 每页数量
	PageNum  int    `json:"pageNum" form:"pageNum"`   // 页码
	Name     string `json:"name" form:"name"`         // 公司名称筛选
}

// Valid 参数合法性校验
func (l *ListCompanyReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&l)
	if l.PageSize <= 0 {
		l.PageSize = 10 // 默认每页10条
	}
	if l.PageNum <= 0 {
		l.PageNum = 1 // 默认第1页
	}
	if l.PageSize > 50 {
		l.PageSize = 10 // 最大每页50条
	}
	return nil
}

// CompanyListItem 公司列表项
type CompanyListItem struct {
	CompanyID           int32  `json:"company_id"`           // 公司ID
	CompanyName         string `json:"company_name"`         // 公司名称
	UnifiedCreditCode   string `json:"unified_credit_code"`  // 统一信用代码
	LegalRepresentative string `json:"legal_representative"` // 法定代表人
	QualificationCount  int64  `json:"qualification_count"`  // 资质数量
	ImageCount          int64  `json:"image_count"`          // 图片数量
}

// ListCompanyData 获取公司列表响应
type ListCompanyData struct {
	List     []*CompanyListItem `json:"list"`
	Total    int64              `json:"total"`
	PageNum  int                `json:"page_num"`
	PageSize int                `json:"page_size"`
}

type ListCompanyWithoutPageReq struct {
	Name string `json:"name" form:"name"` // 公司名称筛选
}

// Valid 参数合法性校验
func (l *ListCompanyWithoutPageReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&l)
	return nil
}

// DeleteCompanyReq 删除公司请求
type DeleteCompanyReq struct {
	Id int64 `json:"id"`
}

// Valid 参数合法性校验
func (d *DeleteCompanyReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindUri(&d)
	_ = c.ShouldBindQuery(&c)
	if d.Id == 0 {
		return fmt.Errorf("公司ID不能为空")
	}
	return nil
}

// CompanyDetailReq 公司详情请求
type CompanyDetailReq struct {
	Id int64 `json:"id" form:"id"`
}

// Valid 参数合法性校验
func (r *CompanyDetailReq) Valid(c *gin.Context) error {
	_ = c.ShouldBindQuery(&r)
	if r.Id == 0 {
		return fmt.Errorf("公司ID不能为空")
	}
	return nil
}

// CompanyInfo 公司信息
type CompanyInfo struct {
	ID          int32  `json:"id"`          // 公司ID
	Name        string `json:"name"`        // 公司名称
	OwnerID     int64  `json:"ownerId"`     // 公司负责人ID
	OwnerName   string `json:"ownerName"`   // 公司负责人姓名
	OwnerMobile string `json:"ownerMobile"` //公司负责人电话
	TeamNum     int    `json:"teamNum"`     // 公司旗下的团队数量
	CreateTime  int64  `json:"createTime"`  // 创建时间
	UpdateTime  int64  `json:"updateTime"`  // 更新时间
	Status      int32  `json:"status"`      // 公司状态
}

// ListCompanyResp 公司列表响应
type ListCompanyResp struct {
	Total int            `json:"total"` // 总数
	List  []*CompanyInfo `json:"list"`  // 公司列表
}

// DepartmentInfo 部门信息
type DepartmentInfo struct {
	DepartmentID       int32             `json:"departmentId"`       // 部门ID
	CompanyID          int32             `json:"companyId"`          // 公司ID
	ParentDepartmentID int32             `json:"parentDepartmentId"` // 父部门ID
	Name               string            `json:"name"`               // 部门名称
	CreateTime         int64             `json:"createTime"`         // 创建时间
	UpdateTime         int64             `json:"updateTime"`         // 更新时间
	Status             int32             `json:"status"`             // 状态
	Children           []*DepartmentInfo `json:"children"`           // 子部门
}

// CompanyDetailResp 公司详情响应
type CompanyDetailResp struct {
	Company *CompanyInfo `json:"company"` // 公司信息
}
