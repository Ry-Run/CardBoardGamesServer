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
	"encoding/json"
	"errors"
	"fmt"
	"framework/game"
	"framework/protocol"
	"net/http"
	"strings"
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
	cltReadChan        chan *MsgPack                         // 共用一个 读通道
	handlers           map[protocol.PackageType]EventHandler // 客户端 Packet 处理器
	ConnectorHandlers  LogicHandler                          // 本地 connector 处理器
}

type EventHandler func(packet *protocol.Packet, conn Connection) error

type HandlerFunc func(session *Session, body []byte) (any, error)
type LogicHandler map[string]HandlerFunc

func (m *WsManager) Run(addr string) {
	go m.clientReadChanHandler()
	http.HandleFunc("/", m.serveWS)
	// 设置不同的消息处理器
	m.setupEventHandler()
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
	// 解析 pomelo Packet 包
	packet, err := protocol.Decode(body.body)
	if err != nil {
		logs.Error("decode Packet err: %v", err)
		return
	}
	if err := m.routeEvent(packet, body.Cid); err != nil {
		logs.Error("routeEvent err: %v", err)
		return
	}
}

func (m *WsManager) Close() {
	m.Lock()
	defer m.Unlock()
	for cid, conn := range m.clts {
		conn.Close()
		delete(m.clts, cid)
	}
}

func (m *WsManager) routeEvent(packet *protocol.Packet, cid string) error {
	conn, ok := m.clts[cid]
	if !ok {
		return errors.New("connection has broken")
	}
	handler, ok := m.handlers[packet.Type]
	if !ok {
		return errors.New("packet type not found")
	}
	return handler(packet, conn)
}

func (m *WsManager) setupEventHandler() {
	m.handlers[protocol.Handshake] = m.HandshakeHandler
	m.handlers[protocol.HandshakeAck] = m.HandshakeAckHandler
	m.handlers[protocol.Heartbeat] = m.HeartbeatHandler
	m.handlers[protocol.Data] = m.MessageHandler
	m.handlers[protocol.Kick] = m.KickHandler
}

func (m *WsManager) HandshakeHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive handshake type: %v", packet.Type)
	resp := protocol.HandshakeResponse{
		Code: 200,
		Sys: protocol.Sys{
			Heartbeat: 3,
		},
	}
	data, _ := json.Marshal(resp)
	buf, err := protocol.Encode(packet.Type, data)
	if err != nil {
		logs.Error("encode packet err: %v", err)
		return err
	}
	return conn.SendMessage(buf)
}

// 握手确认，客户端响应收到 Handshake，服务端一般不用处理
func (m *WsManager) HandshakeAckHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive handshake ack")
	return nil
}

func (m *WsManager) HeartbeatHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive heartbeat type: %v", packet.Type)
	var resp []byte
	data, _ := json.Marshal(resp)
	buf, err := protocol.Encode(packet.Type, data)
	if err != nil {
		logs.Error("encode packet err: %v", err)
		return err
	}
	return conn.SendMessage(buf)
}

func (m *WsManager) MessageHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive message: %+v", packet.Body)
	message := packet.MessageBody()
	// routeStr 形如：connector.entryHandler.entry
	routeStr := message.Route
	routes := strings.Split(routeStr, ".")
	if len(routes) != 3 {
		return errors.New("route unsupported")
	}
	serverType := routes[0]
	handlerMethod := fmt.Sprintf("%s.%s", routes[1], routes[2])
	connectorConfig := game.Conf.GetConnectorByServerType(serverType)
	if connectorConfig != nil {
		// 本地 connector 服务器处理
		handler, ok := m.ConnectorHandlers[handlerMethod]
		if !ok {
			return errors.New("connector handler unsupported")
		}
		data, err := handler(conn.GetSession(), message.Data)
		if err != nil {
			return err
		}
		marshal, _ := json.Marshal(data)
		message.Type = protocol.Response
		message.Data = marshal
		body, err := protocol.MessageEncode(message)
		if err != nil {
			return err
		}
		resp, err := protocol.Encode(packet.Type, body)
		if err != nil {
			return err
		}
		return conn.SendMessage(resp)
	} else {
		// nat handle
	}
	return nil
}

// 服务端会主动发起，所以服务端一般不用处理
func (m *WsManager) KickHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive kick message")
	return nil
}

func NewWsManager() *WsManager {
	return &WsManager{
		cltReadChan: make(chan *MsgPack, 1024),
		clts:        make(map[string]Connection),
		handlers:    make(map[protocol.PackageType]EventHandler),
	}
}
