package remote

import "framework/protocol"

type Msg struct {
	Cid         string
	Body        *protocol.Message
	Src         string // 发送 msg 的 serverId
	Dst         string // 接收 msg 的 serverId
	Router      string
	Uid         string
	SessionData map[string]any
	Type        int // 0 normal 1 session
	PushUser    []string
}

const SessionType = 1
