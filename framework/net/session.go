package net

import "sync"

type Session struct {
	sync.RWMutex
	Cid  string
	Uid  string
	data map[string]any
}

func NewSession(cid string) *Session {
	return &Session{
		Cid:  cid,
		data: make(map[string]any),
	}
}

func (s *Session) Put(k string, v any) {
	s.Lock()
	defer s.Unlock()
	s.data[k] = v
}

func (s *Session) Get(k string) (any, bool) {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.data[k]
	return v, ok
}

func (s *Session) SetData(uid string, data map[string]any) {
	s.Lock()
	defer s.Unlock()
	if s.Uid == uid {
		for k, v := range data {
			s.data[k] = v
		}
	}
}
