package route

import (
	"core/repo"
	"framework/node"
	"game/handler"
	"game/logic"
)

func Register(r *repo.Manager) node.LogicHandler {
	handlers := make(node.LogicHandler)
	manager := logic.NewUnionManager()
	unionHandler := handler.NewUnionHandler(r, manager)
	handlers["unionHandler.createRoom"] = unionHandler.CreateRoom
	handlers["unionHandler.joinRoom"] = unionHandler.JoinRoom
	gameHandler := handler.NewGameHandler(r, manager)
	handlers["gameHandler.roomMessageNotify"] = gameHandler.RoomMessageNotify
	handlers["gameHandler.gameMessageNotify"] = gameHandler.GameMessageNotify
	return handlers
}
