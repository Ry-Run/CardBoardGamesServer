package net

import (
	"common/logs"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var cidBase uint64 = 10000
var (
	maxMessageSize int64 = 1024
	pongWait             = 10 * time.Second
	writeWait            = 10 * time.Second
	pingWait             = pongWait * 9 / 10 // 一定比 pong 少
)

type WsConnection struct {
	Cid       string
	Conn      *websocket.Conn
	wsManager *WsManager
	ReadChan  chan *MsgPack
	WriteChan chan []byte
}

func (c *WsConnection) Run() {
	go c.readMsg()
	go c.writeMsg()
	// websocket ping pong 心跳机制
	c.Conn.SetPongHandler(c.PongHandler)
}

func (c *WsConnection) readMsg() {
	defer func() {
		c.wsManager.removeClt(c.Cid)
	}()
	// 读取消息最大的 msg 大小
	c.Conn.SetReadLimit(maxMessageSize)
	// 设置读超时截止时间，期间没读到数据就会断开连接
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logs.Error("SetReadDeadline err: %v", err)
		return
	}
	for {
		// 3.持续读客户端的消息
		messageType, msg, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		// 客户端发来的是 二进制消息
		if messageType == websocket.BinaryMessage {
			if c.ReadChan != nil {
				c.ReadChan <- &MsgPack{
					Cid:  c.Cid,
					body: msg,
				}
			}
		} else {
			logs.Error("unsupported msg type: %v", messageType)
		}
	}
}

func (c *WsConnection) writeMsg() {
	ticker := time.NewTimer(pingWait)
	defer ticker.Stop()
	for {
		select {
		case message, ok := <-c.WriteChan:
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, nil); err != nil {
					logs.Error("connection closed, err: %v", err)
				}
				return
			}
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				logs.Error("client[%v] write message err: %v", c.Cid, err)
			}
		case <-ticker.C:
			// SetWriteDeadline：从 now + writeWait 这个截止时间之前，这个 WriteMessage(Ping) 必须完成；
			// 如果底层网络卡住（对端不收、TCP 缓冲满、网络异常），导致写一直阻塞，超过 deadline 后写就会返回超时错误。
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logs.Error("client[%v] ping err: %v", c.Cid, err)
			}
			// 2.隔 n 秒就发送一个 ping 消息给客户端
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logs.Error("client[%v] ping err: %v", c.Cid, err)
			}
			logs.Info("ping...")
		}
	}
}

func (c *WsConnection) Close() {
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
}

// Ping/Pong/Close 是控制帧，gorilla 内部拦截处理所有消息，ReadMessage() 只会读到 Text/Binary 数据帧
// 控制帧由 ReadMessage() -> NextReader() -> advanceFrame(注释 7.) 里面处理了，而数据帧则正常返回
func (c *WsConnection) PongHandler(data string) error {
	logs.Info("pong...")
	// 重置读超时截止时间
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	return nil
}

func NewWsConnection(conn *websocket.Conn, wsManager *WsManager) *WsConnection {
	cid := fmt.Sprintf("%s-%s-%d", uuid.New().String(), wsManager.ServerId, atomic.AddUint64(&cidBase, 1))
	return &WsConnection{
		Conn:      conn,
		wsManager: wsManager,
		Cid:       cid,
		WriteChan: make(chan []byte, 1024),
		ReadChan:  wsManager.cltReadChan,
	}
}
