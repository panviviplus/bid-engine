package open

import (
	"github.com/gin-gonic/gin"

	"bid-engine/pkg/db/model"
	"bid-engine/pkg/entity"
	"bid-engine/pkg/handler"
)

func (s *svcImpl) GetUsers(c *gin.Context) {
	var (
		logger = s.logger.With(entity.Ctx(c)...)
		req    entity.GetUsersReq
	)
	if err := req.Valid(c); err != nil {
		logger.Warnw("参数错误", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeParam, err.Error(), nil)
		return
	}
	logger.Infow("请求参数解析结果", "req", req)
	// 获取用户信息
	users, err := s.user.GetUsers(c, req.UserIDs)
	if err != nil {
		logger.Warnw("获取用户信息错误", "err", err)
		handler.SendNormalResp(c, entity.ErrCodeInternal, err.Error(), nil)
		return
	}
	var ret []*entity.SimpleUser
	for _, u := range users {
		ret = append(ret, toSimpleUser(u))
	}
	// 构造返回结果
	handler.SendOKResp(c, ret)
}

func toSimpleUser(user *model.User) *entity.SimpleUser {
	if user == nil {
		return nil
	}
	nickname := user.Nickname
	username := user.Username
	// 现在都是手机号
	if len(username) == 11 {
		username = username[:3] + "****" + username[7:]
		if nickname == "" {
			nickname = "用户" + username[7:]
		}
	}
	return &entity.SimpleUser{
		UserID:     user.UserID,
		CompanyID:  user.CompanyID,
		Role:       user.Role,
		Nickname:   nickname,
		Username:   username,
		Status:     user.Status,
		CreateTime: user.CreateTime,
		UpdateTime: user.UpdateTime,
	}
}
