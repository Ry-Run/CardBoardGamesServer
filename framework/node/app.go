package node

import (
	"common/logs"
	"encoding/json"
	"framework/remote"
)

// 处理游戏实际逻辑的服务
type App struct {
	remoteClt remote.Client
	readChan  chan []byte
	writeChan chan *remote.Msg
	handlers  LogicHandler
}

func Default() *App {
	return &App{
		readChan:  make(chan []byte, 1024),
		writeChan: make(chan *remote.Msg, 1024),
		handlers:  make(LogicHandler),
	}
}

func (a *App) Run(serverId string) error {
	a.remoteClt = remote.NewNatsClient(serverId, a.readChan)
	err := a.remoteClt.Run()
	if err != nil {
		return err
	}
	go a.readChanMsg()
	go a.writeChanMsg()
	return nil
}

// 收到其他 nat client 发来的消息
func (a *App) readChanMsg() {
	for {
		select {
		case data := <-a.readChan:
			var req remote.Msg
			json.Unmarshal(data, &req)
			// 根据 路由消息，分发给对应的 handler 处理
			router := req.Router
			session := remote.NewSession(a.remoteClt, &req)
			session.setData(req.SessionData)
			handlerFunc, ok := a.handlers[router]
			if !ok || handlerFunc == nil {
				continue
			}

			body := req.Body
			handlerResp := handlerFunc(session, body.Data)

			var respBytes []byte
			if handlerResp != nil {
				respBytes, _ = json.Marshal(handlerResp)
			}
			body.Data = respBytes

			resp := &remote.Msg{
				Cid:  req.Cid,
				Uid:  req.Uid,
				Src:  req.Dst,
				Dst:  req.Src,
				Body: body,
			}

			a.writeChan <- resp
		}
	}
}

func (a *App) writeChanMsg() {
	for {
		select {
		case resp, ok := <-a.writeChan:
			if ok {
				respBytes, _ := json.Marshal(resp)
				err := a.remoteClt.SendMsg(resp.Dst, respBytes)
				if err != nil {
					logs.Error("app remote send msg err: %v", err)
				}
			}
		}
	}
}

func (a *App) Close() {
	if a.remoteClt != nil {
		a.remoteClt.Close()
	}
}

func (a *App) RegisterHandler(handler LogicHandler) {
	a.handlers = handler
}
