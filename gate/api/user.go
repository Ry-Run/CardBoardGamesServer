package api

import (
	"common"
	"common/biz"
	"common/config"
	"common/jwts"
	"common/logs"
	"common/rpc"
	"context"
	"framework/msError"
	"time"
	"user/pb"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type UserHandler struct {
}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h UserHandler) Register(ctx *gin.Context) {
	// 接收参数
	var req pb.RegisterParams
	err2 := ctx.ShouldBindJSON(&req)
	if err2 != nil {
		common.Fail(ctx, biz.RequestDataError)
		return
	}

	resp, err := rpc.UserClient.Register(context.TODO(), &req)
	if err != nil {
		logs.Error("rpc call UserClient.Register failed, err=%v", err)
		common.Fail(ctx, msError.ToError(err))
		return
	}
	uid := resp.Uid
	if len(uid) == 0 {
		common.Fail(ctx, biz.SqlError)
		return
	}
	logs.Info("uid: %v", uid)
	// uid 生成 token jwt 格式：A.B.C   A 部分头定义加密算法，B 存储数据，C存储部分签名
	claims := jwts.CustomClaims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Conf.Jwt.Exp) * 24 * time.Hour)),
		},
	}
	token, err := jwts.GenToken(&claims, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("jwt gen failed, err=%v", err)
		common.Fail(ctx, biz.Fail)
		return
	}

	result := map[string]any{
		"token": token,
		"serverInfo": map[string]any{
			"host": config.Conf.Services["connector"].ClientHost,
			"port": config.Conf.Services["connector"].ClientPort,
		},
	}
	common.Success(ctx, result)
}
