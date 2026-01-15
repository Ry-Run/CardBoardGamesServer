package node

import "framework/remote"

type HandlerFunc func(session *remote.Session, msg []byte) error
type LogicHandler map[string]HandlerFunc
