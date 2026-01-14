package connector

import (
	"common/logs"
	"fmt"
	"framework/game"
	"framework/net"
)

type Connector struct {
	isRunning bool
	wsManager *net.WsManager
	handlers  net.LogicHandler
}

func Default() *Connector {
	return &Connector{
		handlers: make(net.LogicHandler),
	}
}

// 启动 websocket 和 nats
func (c *Connector) Run(serverId string) {
	if !c.isRunning {
		c.wsManager = net.NewWsManager()
		c.wsManager.ConnectorHandlers = c.handlers
		c.wsManager.ServerId = serverId
		c.Serve(serverId)
	}
}

func (c *Connector) Close() {
	if c.isRunning {
		// 关闭 websocket 和 nats
		c.wsManager.Close()
	}
}

// 启动 websocket 和 nats
func (c *Connector) Serve(serverId string) {
	logs.Info("run connector serverID=%v", serverId)
	connectorConfig := game.Conf.GetConnector(serverId)
	if connectorConfig == nil {
		panic("no connector config found")
	}
	addr := fmt.Sprintf("%s:%d", connectorConfig.Host, connectorConfig.ClientPort)
	c.isRunning = true
	c.wsManager.Run(addr)
}

func (c *Connector) RegisterHandler(handlers net.LogicHandler) {
	c.handlers = handlers
}
