package logic

import (
	"core/models/entity"
	"core/service"
	"framework/msError"
	"framework/remote"
	"game/component/room"
	"game/models/request"
	"sync"
)

type Union struct {
	sync.RWMutex
	id    int64
	m     *UnionManager
	Rooms map[string]*room.Room
}

// 1.创建房间；2.推送房间号到客户端；3.进入游戏时，将游戏类型推送到客户端
func (u *Union) CreateRoom(service *service.UserService, session *remote.Session, req request.CreateRoomReq, user *entity.User) *msError.Error {
	// 1.创建房间
	roomId := u.m.CreateRoomId()
	newRoom := room.NewRoom(roomId, req.UnionID, req.GameRule, u)
	u.Rooms[roomId] = newRoom
	// 2.推送房间号到客户端
	return newRoom.UserEntryRoom(session, user)
}

func (u *Union) DismissRoom(roomId string) {
	u.Lock()
	defer u.Unlock()
	delete(u.Rooms, roomId)
}

func NewUnion(m *UnionManager) *Union {
	return &Union{
		Rooms: make(map[string]*room.Room),
		m:     m,
	}
}
