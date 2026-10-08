package entity

import (
	"github.com/gin-gonic/gin"
)

// UpdateCompanyProfileReq 更新企业简介请求
type UpdateCompanyProfileReq struct {
	CompanyName           string `form:"company_name" binding:"required"`
	CompanyType           string `form:"company_type" binding:"required"`
	LegalRepresentative   string `form:"legal_representative" binding:"required"`
	RegistrationAuthority string `form:"registration_authority"`
	EstablishmentDate     string `form:"establishment_date"`
	UnifiedCreditCode     string `form:"unified_credit_code" binding:"required"`
	RegisteredCapital     string `form:"registered_capital"`
	RegisteredAddress     string `form:"registered_address" binding:"required"`
	BusinessScope         string `form:"business_scope"`
	RegistrationDate      string `form:"registration_date"`
	BusinessStartTime     string `form:"business_start_time"`
	BusinessEndTime       string `form:"business_end_time"`
}

func (r *UpdateCompanyProfileReq) Valid(c *gin.Context) error {
	return c.ShouldBind(r)
}

// OCRCompanyProfileResp OCR识别企业简介响应
type OCRCompanyProfileResp struct {
	CompanyName           string `json:"company_name"`
	CompanyType           string `json:"company_type"`
	LegalRepresentative   string `json:"legal_representative"`
	RegistrationAuthority string `json:"registration_authority"`
	EstablishmentDate     string `json:"establishment_date"`
	UnifiedCreditCode     string `json:"unified_credit_code"`
	RegisteredCapital     string `json:"registered_capital"`
	RegisteredAddress     string `json:"registered_address"`
	BusinessScope         string `json:"business_scope"`
	RegistrationDate      string `json:"registration_date"`
	BusinessStartTime     string `json:"business_start_time"`
	BusinessEndTime       string `json:"business_end_time"`
}
