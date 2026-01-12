package service

import (
	"common/biz"
	"common/logs"
	"context"
	"core/dao"
	"core/models/entity"
	"core/models/request"
	"core/repo"
	"framework/msError"
	"time"
	"user/pb"
)

// 创建账户
type AccountService struct {
	// 获得默认 grpc 实现，返回客户端一个 Unimplemented 错误，同时避免编译期间没实现所有接口而报错
	pb.UnimplementedUserServiceServer

	accountDao *dao.AccountDao
	redisDao   *dao.RedisDao
}

func NewAccountService(manager *repo.Manager) *AccountService {
	return &AccountService{
		accountDao: dao.NewAccountDao(manager),
		redisDao:   dao.NewRedisDao(manager),
	}
}

func (a *AccountService) Register(ctx context.Context, req *pb.RegisterParams) (*pb.RegisterResponse, error) {
	// 注册业务
	var account entity.Account
	switch req.LoginPlatform {
	case request.WeiXin:
		ac, err := a.wxRegister(req)
		if err != nil {
			return &pb.RegisterResponse{}, msError.GrpcError(err)
		}
		account = ac
	default:
		panic("unhandled default case")
	}

	logs.Info("register server be called, req=%v", req)
	return &pb.RegisterResponse{
		Uid: account.Uid,
	}, nil
}

func (a *AccountService) wxRegister(req *pb.RegisterParams) (entity.Account, *msError.Error) {
	// 1.封装 account 结构，存入 mongo，mongo 会生成
	account := entity.Account{
		WxAccount:  req.Account,
		CreateTime: time.Now(),
	}
	// 2.生成一个数字作为用户的唯一 id，redis 自增
	uid, err := a.redisDao.NextAccountId()
	if err != nil {
		return account, biz.SqlError
	}
	account.Uid = uid
	err = a.accountDao.Save(context.TODO(), &account)
	if err != nil {
		return account, biz.SqlError
	}
	return account, nil
}
