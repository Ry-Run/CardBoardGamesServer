package handler

import (
	"common/logs"
	"core/repo"
	"core/service"
	"framework/remote"
)

type UserHandler struct {
	userService *service.UserService
}

func (h UserHandler) UpdateUserAddress(session *remote.Session, msg []byte) error {
	logs.Info("UpdateUserAddress msg: %v", string(msg))

	return nil
}

func NewUserHandler(r *repo.Manager) *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(r),
	}
}
