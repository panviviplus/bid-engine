package user

import (
	"bid-engine/pkg/db/model"
	"bid-engine/pkg/db/query"
	"bid-engine/pkg/entity"

	"github.com/gin-gonic/gin"
)

func (s *svcImpl) GetBaseInfo(c *gin.Context, userID int64, mobile string) (*UserInfo, error) {
	logger := s.logger.With(entity.Ctx(c)...)
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
		user *model.User
		err  error
	)
	if userID > 0 {
		user, err = dbDo.Where(mc.UserID.Eq(userID)).First()
	} else if mobile != "" {
		user, err = dbDo.Where(mc.Mobile.Eq(mobile)).First()
	}
	if err != nil {
		logger.Warnw("数据库-获取用户信息错误", "err", err.Error())
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &UserInfo{
		UserId:   user.UserID,
		Username: user.Username,
		Nickname: user.Nickname,
		PhoneNum: user.Mobile,
	}, nil
}

func (s *svcImpl) GetBaseInfoByUserName(c *gin.Context, userName string) (*UserInfo, error) {
	var (
		mc   = query.Use(s.db).User
		dbDo = mc.WithContext(c)
	)
	user, err := dbDo.Where(mc.Username.Eq(userName)).First()
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return &UserInfo{
		UserId:   user.UserID,
		Username: user.Username,
		Nickname: user.Nickname,
		PhoneNum: user.Mobile,
	}, nil
}
