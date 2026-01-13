// websocket 消息消费模式：多生产者 -> 单消费者，所有连接把消息发送到一个读通道
// 优点：
//
//	1.集中处理协议
//	2.方便做路由/广播/鉴权/限流，按 cid 找连接、广播给所有人、按房间/用户路由
//	3.降低连接对象的复杂度，WsConnection 只负责网络读写，业务处理不塞进连接里
//
// 缺点：
//
//	1.单个 Manager 消费速度如果跟不上，cltReadChan 会被塞满（目前设置的 1024 buffer），然后连接的 readMsg() 往里写会阻塞，最后影响读网络包，可能导致超时/断线。
//
// 解决办法：如果业务处理很重，通常会在 Manager 里再开 worker pool（或者把 decode/handle 拆出去）避免单点瓶颈。
package net

import (
	"common/logs"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// websocket 需要的配置
var (
	websocketUpgrade = websocket.Upgrader{
		// 跨域配置
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type CheckOriginHandler func(r *http.Request) bool

type WsManager struct {
	sync.RWMutex
	websocketUpgrade   *websocket.Upgrader
	ServerId           string
	CheckOriginHandler CheckOriginHandler
	clts               map[string]Connection
	cltReadChan        chan *MsgPack // 共用一个 读通道
}

func (m *WsManager) Run(addr string) {
	go m.clientReadChanHandler()
	http.HandleFunc("/", m.serveWS)
	err := http.ListenAndServe(addr, nil)
	logs.Fatal("connector listen serve err:%v", err)
}

func (m *WsManager) serveWS(writer http.ResponseWriter, request *http.Request) {
	// websocket 基于 http
	if m.websocketUpgrade == nil {
		m.websocketUpgrade = &websocketUpgrade
	}
	// 1.客户端发起HTTP 请求，建立 websocket 连接
	// Upgrade 函数：如果请求头里没有 Connection: Upgrade / Upgrade: websocket（或其它不满足 websocket 握手要求），Upgrade() 就会返回 err，升级失败
	wsConn, err := m.websocketUpgrade.Upgrade(writer, request, nil)
	if err != nil {
		logs.Error("websocketUpgrade.Upgrade err:%v", err)
		return
	}
	clt := NewWsConnection(wsConn, m)
	m.addClt(clt)
	clt.Run()
}

func (m *WsManager) addClt(clt *WsConnection) {
	m.Lock()
	defer m.Unlock()
	m.clts[clt.Cid] = clt
}

func (m *WsManager) removeClt(cid string) {
	m.clts[cid].Close()
	m.Lock()
	delete(m.clts, cid)
	m.Unlock()
}

func (m *WsManager) clientReadChanHandler() {
	for {
		select {
		case body, ok := <-m.cltReadChan:
			if ok {
				// body 是 pomelo 协议，需要解析一下
				m.decodeClientPack(body)
			}
		}
	}
}

// 解析 pomelo 协议
func (m *WsManager) decodeClientPack(body *MsgPack) {
	logs.Info("receive msg:%v", string(body.body))
}

func (m *WsManager) Close() {
	m.Lock()
	defer m.Unlock()
	for cid, conn := range m.clts {
		conn.Close()
		delete(m.clts, cid)
	}
}

func NewWsManager() *WsManager {
	return &WsManager{
		cltReadChan: make(chan *MsgPack, 1024),
		clts:        make(map[string]Connection),
	}
}
