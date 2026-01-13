package net

type Connection interface {
	Close()
	SendMessage(buf []byte) error
}

type MsgPack struct {
	Cid  string
	body []byte
}
