package router

import (
	"common/config"
	"gate/api"

	"github.com/gin-gonic/gin"
)

// RegisterRouter
//
//	@Description: 注册路由
func RegisterRouter() *gin.Engine {
	if config.Conf.Log.Level == "DEBUG" {
		// 打印更多信息
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	userHandler := api.NewUserHandler()
	r.POST("/register", userHandler.Register)

	return r
}
