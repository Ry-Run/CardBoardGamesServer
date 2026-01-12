package router

import (
	"common/config"
	"common/rpc"
	"gate/api"
	"gate/auth"

	"github.com/gin-gonic/gin"
)

// RegisterRouter
//
//	@Description: 创建 gin 服务器，并注册路由
func RegisterRouter() *gin.Engine {
	if config.Conf.Log.Level == "DEBUG" {
		// 打印更多信息
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化 grpc 客户端，调用 user、... 等 grpc 服务
	rpc.Init()
	r := gin.Default()
	// 绑定中间件，Cors() 处理跨域，
	r.Use(auth.Cors())
	userHandler := api.NewUserHandler()
	r.POST("/register", userHandler.Register)

	return r
}
