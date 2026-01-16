package room

import (
	"core/models/entity"
	"framework/msError"
	"framework/remote"
	"game/component/proto"
)

type Room struct {
	Id       string
	UnionID  int64
	GameRule proto.GameRule
	users    []string
}

// 1.推送房间号到客户端；2.推送游戏类型给客户端（进入游戏也要推送一次）
func (r Room) UserEntryRoom(session *remote.Session, user *entity.User) *msError.Error {
	// 1.推送房间号到客户端
	r.updateUserInfoRoomPush(session, user.Uid)
	// 2.推送游戏类型给客户端
	r.SelfEntryRoomPush(session, user.Uid)
	return nil
}

func (r Room) updateUserInfoRoomPush(session *remote.Session, uid string) {
	pushMsg := map[string]any{
		"roomId":     r.Id,
		"pushRouter": "UpdateUserInfoPush",
	}
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push([]string{uid}, pushMsg, "ServerMessagePush")
}

func (r Room) SelfEntryRoomPush(session *remote.Session, uid string) {
	pushMsg := map[string]any{
		"gameType":   r.GameRule.GameType,
		"pushRouter": "SelfEntryRoomPush",
	}
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push([]string{uid}, pushMsg, "ServerMessagePush")
}

func NewRoom(id string, unionID int64, rule proto.GameRule) *Room {
	return &Room{
		Id:       id,
		UnionID:  unionID,
		GameRule: rule,
		users:    []string{},
	}
}
