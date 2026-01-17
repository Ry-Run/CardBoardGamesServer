package handler

import (
	"common"
	"common/biz"
	"context"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/remote"
	"game/logic"
	"game/models/request"
)

type UnionHandler struct {
	m           *logic.UnionManager
	userService *service.UserService
}

func (u UnionHandler) CreateRoom(session *remote.Session, msg []byte) any {
	// union 联盟持有多个房间
	// unionManager 管理多个联盟
	// room 房间关联 game（接口 实现多个不同的游戏）
	// 1.接收参数
	uid := session.GetUid()
	if len(uid) <= 0 {
		return common.FailNoCtx(biz.InvalidUsers)
	}

	var req request.CreateRoomReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.FailNoCtx(biz.RequestDataError)
	}
	// 2.根据 session 用户 id 查询用户的信息
	user, err := u.userService.FindUserByUid(context.TODO(), uid)
	if err != nil {
		return common.FailNoCtx(err)
	}
	if user == nil {
		return common.FailNoCtx(biz.InvalidUsers)
	}
	// 3.根据游戏规则、游戏类型、用户信息 创建房间
	// todo 检查 session 是否 已经存在 roomId，如果有则代表用户已经在房间中了，就不能再次创建房间
	union := u.m.GetUnion(req.UnionID)
	err = union.CreateRoom(u.userService, session, req, user)
	if err != nil {
		return common.FailNoCtx(err)
	}
	return common.SuccessNoCtx(nil)
}

func NewUnionHandler(r *repo.Manager, manager *logic.UnionManager) *UnionHandler {
	return &UnionHandler{
		m:           manager,
		userService: service.NewUserService(r),
	}
}
