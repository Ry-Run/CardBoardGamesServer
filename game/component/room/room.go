package room

import (
	"common/logs"
	"core/models/entity"
	"framework/msError"
	"framework/remote"
	"game/component/base"
	"game/component/proto"
	"game/component/sz"
	"game/models/request"
	"sync"
	"time"
)

type Room struct {
	sync.RWMutex
	Id            string
	UnionID       int64
	GameRule      proto.GameRule
	users         map[string]*proto.RoomUser
	RoomCreator   *proto.RoomCreator
	GameFrame     GameFrame
	KickSchedules map[string]*time.Timer
	union         base.UnionBase
	dismissed     bool
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
	go r.addKickScheduleEvent(session, user.Uid)
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

func (r *Room) addKickScheduleEvent(session *remote.Session, uid string) {
	r.KickSchedules[uid] = time.AfterFunc(10*time.Second, func() {
		logs.Info("kick 执行，用户 %v 长时间未准备", uid)
		// 取消定时任务，目前 time.AfterFunc 就是一次性的，也可以不停止
		timer, ok := r.KickSchedules[uid]
		if ok {
			timer.Stop()
		}
		delete(r.KickSchedules, uid)
		// 判断用户是否该踢出
		user, ok := r.users[uid]
		if ok && user.UserStatus < proto.Ready {
			r.kickUser(user, session)
			// 判断是否可以解散房间
			if len(r.users) == 0 {
				r.dismissRoom()
			}
		}
	})
}

func (r *Room) ServerMessagePush(users []string, data any, session *remote.Session) {
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push(users, data, "ServerMessagePush")
}

func (r *Room) kickUser(user *proto.RoomUser, session *remote.Session) {
	kickUid := user.UserInfo.Uid
	// 给被踢用户推送：roomId 为空的消息
	r.ServerMessagePush([]string{kickUid}, proto.UpdateUserInfoPush(""), session)
	// 通知房间其他用户
	users := make([]string, len(r.users))
	for uid, _ := range r.users {
		users = append(users, uid)
	}
	r.ServerMessagePush(users, proto.UserLeaveRoomPushData(user), session)
	delete(r.users, kickUid)
}

// 解散房间，将 union 存储的房间信息，删除掉
func (r *Room) dismissRoom() {
	r.Lock()
	defer r.Unlock()
	// 避免重复解散
	if r.dismissed {
		return
	}
	r.dismissed = true
	r.cancelAllScheduler()
	r.union.DismissRoom(r.Id)
}

// 取消所有定时任务
func (r *Room) cancelAllScheduler() {
	for uid, timer := range r.KickSchedules {
		timer.Stop()
		delete(r.KickSchedules, uid)
	}
}

func NewRoom(id string, unionID int64, rule proto.GameRule, u base.UnionBase) *Room {
	r := &Room{
		Id:            id,
		UnionID:       unionID,
		GameRule:      rule,
		users:         make(map[string]*proto.RoomUser),
		KickSchedules: make(map[string]*time.Timer),
		union:         u,
	}
	if rule.GameType == int(proto.PinSanZhang) {
		r.GameFrame = sz.NewGameFrame(rule, r)
	}
	return r
}
