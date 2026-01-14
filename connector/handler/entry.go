package handler

import (
	"common/logs"
	"framework/net"
)

type EntryHandler struct {
}

func (h EntryHandler) Entry(session *net.Session, body []byte) (any, error) {
	logs.Info("entry request params: %v", string(body))
	return nil, nil
}

func NewEntryHandler() *EntryHandler {
	return &EntryHandler{}
}
