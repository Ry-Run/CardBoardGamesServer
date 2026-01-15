package logic

import (
	"core/service"
	"framework/msError"
	"framework/remote"
	"game/component/room"
	"game/models/request"
)

type Union struct {
	id    int64
	m     *UnionManager
	Rooms map[string]room.Room
}

func (u *Union) CreateRoom(service *service.UserService, session *remote.Session, req request.CreateRoomReq) *msError.Error {
	
}

func NewUnion(m *UnionManager) *Union {
	return &Union{
		Rooms: make(map[string]room.Room),
		m:     m,
	}
}
