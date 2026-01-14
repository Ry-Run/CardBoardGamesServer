package service

import (
	"common/logs"
	"common/utils"
	"connector/request"
	"context"
	"core/dao"
	"core/models/entity"
	"core/repo"
	"fmt"
	"framework/game"
	"time"
)

type UserService struct {
	userDao *dao.UserDao
}

// 通过 uid 查询 user 有则返回，没有则新增
func (s *UserService) FindOrSaveUser(ctx context.Context, uid string, info request.UserInfo) (*entity.User, error) {
	user, err := s.userDao.FindUserByUid(ctx, uid)
	if err != nil {
		logs.Error("[UserService] FindOrSaveUser FindUserByUid err: %v", err)
		return nil, err
	}
	if user == nil {
		// save
		user = &entity.User{
			Uid: uid,
			// 1.viper 读取配置时，如果用 map 取数据，会把 map 的 key 统一转为全小写
			// 2.这里可能使用了 json 库读取的值，数字类型解析到 any 容器时，默认使用 float64
			Gold:          int64(game.Conf.GameConfig["startgold"]["value"].(float64)),
			Avatar:        utils.Default(info.Avatar, "Common/head_icon_default"),
			Nickname:      utils.Default(info.NickName, fmt.Sprintf("%s%s", "qy", uid)),
			Sex:           info.Sex,
			CreateTime:    time.Now().UnixMilli(),
			LastLoginTime: time.Now().UnixMilli(),
		}
		err = s.userDao.Save(ctx, user)
		if err != nil {
			logs.Error("[UserService] FindOrSaveUser save err: %v", err)
			return nil, err
		}
	}
	return user, err
}

func NewUserService(r *repo.Manager) *UserService {
	return &UserService{
		userDao: dao.NewUserDao(r),
	}
}
