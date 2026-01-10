package api

import (
	"common/rpc"
	"context"
	"user/pb"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h UserHandler) Register(ctx *gin.Context) {
	resp, err := rpc.UserClient.Register(context.TODO(), &pb.RegisterParams{})
	if err != nil {

	}
	uid := resp.Uid
	// uid 生成 token
	//ctx.JSON(200)
}
