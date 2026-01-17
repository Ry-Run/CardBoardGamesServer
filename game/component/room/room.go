package room

import (
	"core/models/entity"
	"framework/msError"
	"framework/remote"
	"game/component/proto"
	"game/component/sz"
	"game/models/request"
)

type Room struct {
	Id          string
	UnionID     int64
	GameRule    proto.GameRule
	users       map[string]*proto.RoomUser
	RoomCreator *proto.RoomCreator
	GameFrame   GameFrame
}

// 1.推送房间号到客户端；2.推送游戏类型给客户端（进入游戏也要推送一次）
func (r *Room) UserEntryRoom(session *remote.Session, user *entity.User) *msError.Error {
	r.RoomCreator = &proto.RoomCreator{
		Uid: user.Uid,
	}
	if r.UnionID == 1 {
		r.RoomCreator.CreatorType = proto.UserCreatorType
	} else {
		r.RoomCreator.CreatorType = proto.UnionCreatorType
	}
	r.users[user.Uid] = proto.ToRoomUser(user)
	// 1.推送房间号到客户端
	r.updateUserInfoRoomPush(session, user.Uid)
	session.Put("roomId", r.Id)
	// 2.推送游戏类型给客户端
	r.SelfEntryRoomPush(session, user.Uid)
	return nil
}

func (r *Room) updateUserInfoRoomPush(session *remote.Session, uid string) {
	pushMsg := map[string]any{
		"roomId":     r.Id,
		"pushRouter": "UpdateUserInfoPush",
	}
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push([]string{uid}, pushMsg, "ServerMessagePush")
}

func (r *Room) SelfEntryRoomPush(session *remote.Session, uid string) {
	pushMsg := map[string]any{
		"gameType":   r.GameRule.GameType,
		"pushRouter": "SelfEntryRoomPush",
	}
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push([]string{uid}, pushMsg, "ServerMessagePush")
}

func (r *Room) RoomMessageHandler(session *remote.Session, req request.RoomMessageReq) {
	if req.Type == proto.GetRoomSceneInfoNotify {
		r.GetRoomSceneInfoPush(session)
	}
}

func (r *Room) GetRoomSceneInfoPush(session *remote.Session) {
	roomUserInfoArr := make([]*proto.RoomUser, len(r.users))
	for _, v := range r.users {
		roomUserInfoArr = append(roomUserInfoArr, v)
	}

	data := map[string]any{
		"type":       proto.GetRoomSceneInfoPush,
		"pushRouter": "RoomMessagePush",
		"data": map[string]any{
			"roomID":          r.Id,
			"roomCreatorInfo": r.RoomCreator,
			"gameRule":        r.GameRule,
			"roomUserInfoArr": roomUserInfoArr,
			"gameData":        r.GameFrame.GetGameData(),
		},
	}
	session.Push([]string{session.GetUid()}, data, "ServerMessagePush")
}

func NewRoom(id string, unionID int64, rule proto.GameRule) *Room {
	r := &Room{
		Id:       id,
		UnionID:  unionID,
		GameRule: rule,
		users:    make(map[string]*proto.RoomUser),
	}
	if rule.GameType == int(proto.PinSanZhang) {
		r.GameFrame = sz.NewGameFrame(rule, r)
	}
	return r
}
