package route

import (
	"connector/handler"
	"framework/net"
)

func Register() net.LogicHandler {
	handlers := make(net.LogicHandler)
	entryHandler := handler.NewEntryHandler()
	handlers["entryHandler.entry"] = entryHandler.Entry
	return handlers
}
