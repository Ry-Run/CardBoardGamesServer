package remote

import (
	"common/logs"
	"encoding/json"
	"framework/protocol"
	"sync"
)

type Session struct {
	sync.RWMutex
	clt             Client
	msg             *Msg
	pushChan        chan *UserPushMsg
	data            map[string]any
	pushSessionChan chan map[string]any
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
		clt:             client,
		msg:             msg,
		pushChan:        make(chan *UserPushMsg, 1024),
		data:            make(map[string]any),
		pushSessionChan: make(chan map[string]any, 1024),
	}
	go session.pushChanReader()
	go session.pushSession()
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

func (s *Session) Put(key string, val any) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = val
	s.pushSessionChan <- s.data
}

func (s *Session) pushSession() {
	for {
		select {
		case session := <-s.pushSessionChan:
			msg := Msg{
				Dst:         s.msg.Src,
				Src:         s.msg.Dst,
				Cid:         s.msg.Cid,
				SessionData: session,
				Type:        SessionType,
			}
			data, _ := json.Marshal(msg)
			if err := s.clt.SendMsg(msg.Dst, data); err != nil {
				logs.Error("push session err: %v", err)
			}
		}
	}
}

func (s *Session) setData(data map[string]any) {
	s.Lock()
	defer s.Unlock()
	for k, v := range data {
		s.data[k] = v
	}
}

func (s *Session) Get(key string) (any, bool) {
	s.RLock()
	defer s.RUnlock()
	val, ok := s.data[key]
	return val, ok
}
