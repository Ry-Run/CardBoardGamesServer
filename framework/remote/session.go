package remote

import (
	"common/logs"
	"encoding/json"
	"framework/protocol"
)

type Session struct {
	clt      Client
	msg      *Msg
	pushChan chan *UserPushMsg
}

type PushMsg struct {
	data   []byte
	router string
}

type UserPushMsg struct {
	PushMsg PushMsg  `json:"pushMsg"`
	Users   []string `json:"users"`
}

func NewSession(client Client, msg *Msg) *Session {
	session := Session{
		clt:      client,
		msg:      msg,
		pushChan: make(chan *UserPushMsg, 1024),
	}
	go session.pushChanReader()
	return &session
}

func (s *Session) GetUid() string {
	return s.msg.Uid
}

func (s *Session) Push(users []string, data any, router string) {
	msg, _ := json.Marshal(data)
	pushMsg := &UserPushMsg{
		PushMsg: PushMsg{
			data:   msg,
			router: router,
		},
		Users: users,
	}
	s.pushChan <- pushMsg
}

func (s *Session) pushChanReader() {
	for {
		select {
		case data := <-s.pushChan:
			pushMsg := protocol.Message{
				Type:  protocol.Push,
				ID:    s.msg.Body.ID,
				Route: data.PushMsg.router,
				Data:  data.PushMsg.data,
			}
			msg := Msg{
				Dst:      s.msg.Src,
				Src:      s.msg.Dst,
				Body:     &pushMsg,
				Cid:      s.msg.Cid,
				Uid:      s.GetUid(),
				PushUser: data.Users,
			}
			res, _ := json.Marshal(msg)
			logs.Info("push message dst: %v", msg.Dst)
			if err := s.clt.SendMsg(msg.Dst, res); err != nil {
				logs.Error("push message err: %v, msg: %v", err, msg)
			}
		}
	}
}
