package remote

type Session struct {
	client Client
	msg    *Msg
}

func NewSession(client Client, msg *Msg) *Session {
	return &Session{
		client: client,
		msg:    msg,
	}
}

func (s *Session) GetUid() string {
	return s.msg.Uid
}
