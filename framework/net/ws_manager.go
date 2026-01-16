// websocket 消息消费模式：多生产者 -> 单消费者，所有连接把消息发送到一个读通道
// 优点：
//
//	1.集中处理协议
//	2.方便做路由/广播/鉴权/限流，按 cid 找连接、广播给所有人、按房间/用户路由
//	3.降低连接对象的复杂度，WsConnection 只负责网络读写，业务处理不塞进连接里
//
// 缺点：
//
//	1.单个 Manager 消费速度如果跟不上，CltReadChan 会被塞满（目前设置的 1024 buffer），然后连接的 readMsg() 往里写会阻塞，最后影响读网络包，可能导致超时/断线。
//
// 解决办法：如果业务处理很重，通常会在 Manager 里再开 worker pool（或者把 decode/handle 拆出去）避免单点瓶颈。
package net

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"errors"
	"fmt"
	"framework/game"
	"framework/protocol"
	"framework/remote"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

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
	cltReadChan        chan *MsgPack                         // 客户端 WS 入站消息通道：各连接读取到的消息统一投递到这里，由后续逻辑消费
	handlers           map[protocol.PackageType]EventHandler // 客户端 Packet 处理器
	ConnectorHandlers  LogicHandler                          // 本地 connector 处理器
	RemoteReadChan     chan []byte                           // 远端入站消息通道：来自 NATS server/集群的消息写入此处，由本地逻辑读取处理
	RemoteClt          remote.Client
	RemotePushChan     chan *remote.Msg // 专门处理 push 到客户端的 Channel
}

type EventHandler func(packet *protocol.Packet, conn Connection) error

type HandlerFunc func(session *Session, body []byte) (any, error)
type LogicHandler map[string]HandlerFunc

func (m *WsManager) Run(addr string) {
	// 处理 WS 消息
	go m.clientReadChanHandler()
	// 处理 NATS server 消息
	go m.remoteReadChanHandler()
	// 专门一个处理 push 的消息，提高吞吐量
	go m.remotePushChanHandler()
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
		// 目标服务器 serverId
		dst, err := m.selectDst(serverType)
		if err != nil {
			logs.Error("remote send msg selectDst err: %v", err)
			return err
		}
		session := conn.GetSession()
		msg := &remote.Msg{
			Cid:         session.Cid,
			Uid:         session.Uid,
			Src:         m.ServerId,
			Dst:         dst,
			Router:      handlerMethod,
			Body:        message,
			SessionData: session.data,
		}
		data, _ := json.Marshal(msg)
		err = m.RemoteClt.SendMsg(dst, data)
		if err != nil {
			logs.Error("remote send msg err: %v", err)
			return err
		}
	}
	return nil
}

// 服务端会主动发起，所以服务端一般不用处理
func (m *WsManager) KickHandler(packet *protocol.Packet, conn Connection) error {
	logs.Info("receive kick message")
	return nil
}

func (m *WsManager) remoteReadChanHandler() {
	for {
		select {
		case body, ok := <-m.RemoteReadChan:
			if ok {
				logs.Info("sub nats msg: %v", string(body))
				var msg remote.Msg
				if err := json.Unmarshal(body, &msg); err != nil {
					logs.Error("nat remote message format err: %v", err)
					continue
				}
				// 0 normal 推送至客户端；1 session 更新本地 session 相关数据，不推送
				switch msg.Type {
				case 0:
					if msg.Body == nil {
						continue
					}
					switch msg.Body.Type {
					case protocol.Request, protocol.Response:
						// 给客户端回消息都是 protocol.Response 类型
						msg.Body.Type = protocol.Response
						m.Response(&msg)
					case protocol.Push:
						m.RemotePushChan <- &msg
					}
				case 1:

				}
			}
		}
	}
}

func (m *WsManager) selectDst(serverType string) (string, error) {
	serverConfigs, ok := game.Conf.ServersConf.TypeServer[serverType]
	if !ok {
		return "", errors.New("no server found")
	}
	// 随机选一个服务器，正常来说应该采用负载均衡算法：轮询、权重
	rand.New(rand.NewSource(time.Now().UnixNano()))
	index := rand.Intn(len(serverConfigs))
	return serverConfigs[index].ID, nil
}

func (m *WsManager) Response(r *remote.Msg) {
	conn, ok := m.clts[r.Cid]
	if !ok {
		logs.Error("%s client down，uid=%s", r.Cid, r.Uid)
		return
	}
	buf, err := protocol.MessageEncode(r.Body)
	if err != nil {
		logs.Error("response MessageEncode err: %v", err)
		return
	}
	res, err := protocol.Encode(protocol.Data, buf)
	if err != nil {
		logs.Error("response Encode err: %v", err)
		return
	}
	// 如果是推送的消息，可能会涉及到发送给多个 user
	if r.Body.Type == protocol.Push {
		for _, v := range m.clts {
			if utils.Contains(r.PushUser, v.GetSession().Uid) {
				v.SendMessage(res)
			}
		}
	} else {
		conn.SendMessage(res)
	}
}

func (m *WsManager) remotePushChanHandler() {
	for {
		select {
		case body, ok := <-m.RemotePushChan:
			if ok {
				if body.Body.Type == protocol.Push {
					m.Response(body)
				}
			}
		}
	}
}

func NewWsManager() *WsManager {
	return &WsManager{
		cltReadChan:    make(chan *MsgPack, 1024),
		clts:           make(map[string]Connection),
		handlers:       make(map[protocol.PackageType]EventHandler),
		RemoteReadChan: make(chan []byte, 1024),
		RemotePushChan: make(chan *remote.Msg, 1024),
	}
}
