package remote

import (
	"common/logs"
	"errors"
	"framework/game"

	"github.com/nats-io/nats.go"
)

type NatsClient struct {
	serverId string
	conn     *nats.Conn
	readChan chan []byte
}

func NewNatsClient(serverId string, readChan chan []byte) *NatsClient {
	return &NatsClient{
		serverId: serverId,
		readChan: readChan,
	}
}

func (n *NatsClient) Run() error {
	var err error
	n.conn, err = nats.Connect(game.Conf.ServersConf.Nats.Url)
	if err != nil {
		logs.Error("connect nats server failed, err: %v", err)
		return err
	}
	go n.sub()
	return nil
}

func (n *NatsClient) SendMsg(dst string, data []byte) error {
	if n.conn == nil {
		return errors.New("conn is nil")
	}
	return n.conn.Publish(dst, data)
}

func (n *NatsClient) Close() error {
	if n.conn != nil {
		n.conn.Close()
	}
	return nil
}

func (n *NatsClient) sub() {
	// 在 serverId 上订阅
	_, err := n.conn.Subscribe(n.serverId, func(msg *nats.Msg) {
		// 收到的其他 nat client 传递过来的消息
		n.readChan <- msg.Data
	})
	if err != nil {
		logs.Error("nat sub err: %v", err)
	}
}
