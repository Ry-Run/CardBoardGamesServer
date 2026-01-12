package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.Request 是读请求
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			// c.Header 是设置响应头
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Token")
			c.Header("Access-Control-Expose-Headers", "Access-Control-Allow-Headers, Token")
			c.Header("Access-Control-Max-Age", "172800")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Set("content-type", "application/json")
		}
		if method == "OPTIONS" {
			//c.JSON(200, Controller.R(200, nil, "Options Request"))
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}
