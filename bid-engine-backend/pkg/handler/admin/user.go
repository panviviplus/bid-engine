package admin

import (
	"github.com/gin-gonic/gin"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

func (s *svcImpl) AddUser(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) ListUser(c *gin.Context) {
	handler.SendOKResp(c, []interface{}{})
}

func (s *svcImpl) UpdateUser(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) UploadExcel(c *gin.Context) {
	handler.SendNormalResp(c, entity.ErrCodeParam, "管理后台功能暂未开放", nil)
}

func (s *svcImpl) ListUserWithoutPage(c *gin.Context) {
	handler.SendOKResp(c, []interface{}{})
}
