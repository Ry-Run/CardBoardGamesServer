package handler

import (
	"common"
	"common/biz"
	"common/logs"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/remote"
	"hall/model/request"
	"hall/model/response"
)

type UserHandler struct {
	userService *service.UserService
}

func (u UserHandler) UpdateUserAddress(session *remote.Session, msg []byte) any {
	logs.Info("UpdateUserAddress msg: %v", string(msg))
	var req request.UpdateUserAddressReq
	err := json.Unmarshal(msg, &req)
	if err != nil {
		return common.FailNoCtx(biz.RequestDataError)
	}
	err = u.userService.UpdateUserAddressByUid(session.GetUid(), req)
	if err != nil {
		return common.FailNoCtx(biz.SqlError)
	}
	resp := response.UpdateUserAddressResp{
		Res: common.Result{
			Code: biz.OK,
		},
		UpdateUserData: req,
	}
	return resp
}

func NewUserHandler(r *repo.Manager) *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(r),
	}
}
