package room

import (
	"common/biz"
	"common/logs"
	"core/models/entity"
	"framework/msError"
	"framework/remote"
	"game/component/base"
	"game/component/mj"
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
	gameStarted   bool
}

// 1.推送房间号到客户端；2.推送游戏类型给客户端（进入游戏也要推送一次）；3.通知其他用户，此用户加入房间
func (r *Room) UserEntryRoom(session *remote.Session, user *entity.User) *msError.Error {
	r.RoomCreator = &proto.RoomCreator{
		Uid: user.Uid,
	}
	if r.UnionID == 1 {
		r.RoomCreator.CreatorType = proto.UserCreatorType
	} else {
		r.RoomCreator.CreatorType = proto.UnionCreatorType
	}
	// 最多 6 人参加: 0-5 号，按理前后端都有一个 MaxPlayerCount 配置
	chairID := r.genEmptyChairId(r.GameRule.MaxPlayerCount)
	if chairID == -1 {
		return biz.RoomPlayerCountFull
	}
	r.users[user.Uid] = proto.ToRoomUser(user, chairID)
	// 1.推送房间号到客户端
	r.updateUserInfoRoomPush(session, user.Uid)
	session.Put("roomId", r.Id)
	// 2.推送游戏类型给客户端
	r.SelfEntryRoomPush(session, user.Uid)
	// 3.通知其他用户，此用户加入房间
	r.OtherUserEntryRoomPushData(session, user.Uid)
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
	switch req.Type {
	case proto.GetRoomSceneInfoNotify:
		r.GetRoomSceneInfoPush(session)
	case proto.UserReadyNotify:
		r.userReady(session.GetUid(), session)
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
			"gameData":        r.GameFrame.GetGameData(session),
		},
	}
	session.Push([]string{session.GetUid()}, data, "ServerMessagePush")
}

func (r *Room) addKickScheduleEvent(session *remote.Session, uid string) {
	r.Lock()
	defer r.Unlock()
	t, ok := r.KickSchedules[uid]
	if ok {
		t.Stop()
		delete(r.KickSchedules, uid)
	}
	r.KickSchedules[uid] = time.AfterFunc(30*time.Second, func() {
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

// 1.push 用户的座次；2.修改用户状态；3.取消定时
func (r *Room) userReady(uid string, session *remote.Session) {
	// 修改状态
	user, ok := r.users[uid]
	if !ok {
		return
	}
	user.UserStatus = proto.Ready
	// 取消定时任务
	timer, ok := r.KickSchedules[uid]
	if ok {
		timer.Stop()
		delete(r.KickSchedules, uid)
	}
	// 给全部用户推送状态, push 用户座次
	otherUsers := r.otherUsers(uid)
	r.ServerMessagePush(otherUsers, proto.UserReadyPushData(user.ChairID), session)
	// 判断是否可以开始游戏
	if r.IsStartGame() {
		r.StartGame(session, user)
	}
}

func (r *Room) JoinRoom(session *remote.Session, user *entity.User) *msError.Error {
	return r.UserEntryRoom(session, user)
}

func (r *Room) OtherUserEntryRoomPushData(session *remote.Session, uid string) {
	user, ok := r.users[uid]
	if !ok {
		return
	}
	others := r.otherUsers(uid)
	r.ServerMessagePush(others, proto.OtherUserEntryRoomPushData(user), session)
}

func (r *Room) otherUsers(uid string) []string {
	others := make([]string, 0)
	for u, _ := range r.users {
		if u != uid {
			others = append(others, u)
		}
	}
	return others
}

func (r *Room) genEmptyChairId(seats int) int {
	r.Lock()
	defer r.Unlock()
	if len(r.users) == 0 {
		return 0
	}

	chairs := make([]bool, seats)
	for _, user := range r.users {
		if user.ChairID >= 0 && user.ChairID < seats {
			chairs[user.ChairID] = true
		}
	}
	for i, occupy := range chairs {
		if !occupy {
			return i
		}
	}
	return -1
}

func (r *Room) IsStartGame() bool {
	// 房间内准备人数 >= 最小开始游戏人数
	userReadyCount := 0
	for _, user := range r.users {
		if user.UserStatus == proto.Ready {
			userReadyCount++
		}
	}
	return len(r.users) == userReadyCount && userReadyCount >= r.GameRule.MinPlayerCount
}

func (r *Room) StartGame(session *remote.Session, user *proto.RoomUser) {
	if r.gameStarted {
		return
	}
	r.gameStarted = true
	for _, user := range r.users {
		user.UserStatus = proto.Playing
	}
	r.GameFrame.StartGame(session, user)
}

func (r *Room) GetUsers() map[string]*proto.RoomUser {
	return r.users
}

func (r *Room) GetAllUid() []string {
	users := make([]string, len(r.users))
	for uid, _ := range r.users {
		users = append(users, uid)
	}
	return users
}

func (r *Room) GameMessageHandler(session *remote.Session, msg []byte) {
	user, ok := r.users[session.GetUid()]
	if !ok {
		return
	}
	r.GameFrame.GameMessageHandler(user, session, msg)
}

func (r *Room) EndGame(session *remote.Session) {
	r.gameStarted = false
	for _, user := range r.users {
		user.UserStatus = proto.None
	}
}

func (r *Room) UserReady(uid string, session *remote.Session) {
	r.userReady(uid, session)
}

func (r *Room) GetId() string {
	return r.Id
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
	switch proto.GameType(rule.GameType) {
	case proto.PinSanZhang:
		r.GameFrame = sz.NewGameFrame(rule, r)
	case proto.HongZhong:
		r.GameFrame = mj.NewGameFrame(rule, r)
	}
	return r
}
