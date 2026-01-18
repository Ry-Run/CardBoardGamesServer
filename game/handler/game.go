package handler

import (
	"common"
	"common/biz"
	"core/repo"
	"core/service"
	"encoding/json"
	"fmt"
	"framework/remote"
	"game/logic"
	"game/models/request"
)

type GameHandler struct {
	m           *logic.UnionManager
	userService *service.UserService
}

func (h *GameHandler) RoomMessageNotify(session *remote.Session, msg []byte) any {
	if len(session.GetUid()) <= 0 {
		return common.FailNoCtx(biz.InvalidUsers)
	}
	var req request.RoomMessageReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.FailNoCtx(biz.RequestDataError)
	}
	// room 处理业务
	roomId, ok := session.Get("roomId")
	if !ok {
		return common.FailNoCtx(biz.NotInRoom)
	}
	room := h.m.GetRoomById(fmt.Sprintf("%v", roomId))
	if room == nil {
		return common.FailNoCtx(biz.RoomNotExist)
	}
	room.RoomMessageHandler(session, req)
	return nil // Notify 消息不需要响应
}

func (h *GameHandler) GameMessageNotify(session *remote.Session, msg []byte) any {
	if len(session.GetUid()) <= 0 {
		return common.FailNoCtx(biz.InvalidUsers)
	}
	// room 处理业务
	roomId, ok := session.Get("roomId")
	if !ok {
		return common.FailNoCtx(biz.NotInRoom)
	}
	room := h.m.GetRoomById(fmt.Sprintf("%v", roomId))
	if room == nil {
		return common.FailNoCtx(biz.RoomNotExist)
	}
	room.GameMessageHandler(session, msg)
	return nil // Notify 消息不需要响应
}

func NewGameHandler(r *repo.Manager, manager *logic.UnionManager) *GameHandler {
	return &GameHandler{
		m:           manager,
		userService: service.NewUserService(r),
	}
}
