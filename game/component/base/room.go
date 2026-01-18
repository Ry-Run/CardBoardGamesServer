package base

import "game/component/proto"

type RoomFrame interface {
	GetUsers() map[string]*proto.RoomUser
	GetAllUid() []string
	GetId() string
}
