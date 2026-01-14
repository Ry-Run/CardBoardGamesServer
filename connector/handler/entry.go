package handler

import (
	"common"
	"common/biz"
	"common/config"
	"common/jwts"
	"common/logs"
	"connector/request"
	"context"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/game"
	"framework/net"
)

type EntryHandler struct {
	userService *service.UserService
}

func (h EntryHandler) Entry(session *net.Session, body []byte) (any, error) {
	logs.Info("entry request params: %v", string(body))
	var req request.EntryReq
	err := json.Unmarshal(body, &req)
	if err != nil {
		return common.FailNoCtx(biz.RequestDataError), nil
	}
	// 校验 token
	uid, err := jwts.ParseToken(req.Token, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("parse token err: %v", err)
		return common.FailNoCtx(biz.TokenInfoError), nil
	}
	// 根据 uid 查询 mongo，用户不存在则生成一个
	user, err := h.userService.FindOrSaveUser(context.TODO(), uid, req.UserInfo)
	if err != nil {
		return common.FailNoCtx(biz.SqlError), nil
	}
	// 保存用户 uid 到 session
	session.Uid = uid
	return common.SuccessNoCtx(map[string]any{
		"userInfo": user,
		"config":   game.Conf.GetFrontGameConfig(),
	}), nil
}

func NewEntryHandler(r *repo.Manager) *EntryHandler {
	return &EntryHandler{
		userService: service.NewUserService(r),
	}
}
