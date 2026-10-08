package admin

import (
	"github.com/gin-gonic/gin"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

func (s *svcImpl) AddCompany(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) AddCompanyOwner(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) ListCompany(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) DeleteCompany(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) EditCompany(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) CompanyDetail(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) ListCompanyWithoutPage(c *gin.Context) {
	handler.SendOKResp(c, []interface{}{})
}
