package net

type Connection interface {
	Close()
}

type MsgPack struct {
	Cid  string
	body []byte
}
