package api

import "github.com/gin-gonic/gin"

type UserHandler struct {
}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h UserHandler) Register(*gin.Context) {

}
