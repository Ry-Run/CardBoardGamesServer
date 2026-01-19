package mj

import (
	"framework/remote"
	"game/component/base"
	"game/component/proto"

	"github.com/jinzhu/copier"
)

type GameFrame struct {
	r          base.RoomFrame
	gameRule   proto.GameRule
	gameData   *GameData
	logic      *Logic
	gameResult *GameResult
}

func NewGameFrame(rule proto.GameRule, room base.RoomFrame) *GameFrame {
	return &GameFrame{
		r:        room,
		gameRule: rule,
		gameData: initGameData(rule),
		logic:    NewLogic(),
	}
}

func initGameData(rule proto.GameRule) *GameData {
	g := &GameData{
		ChairCount:     rule.MaxPlayerCount,
		HandCards:      make([][]int, rule.MaxPlayerCount),
		GameStatus:     GameStatusNone,
		OperateRecord:  make([]OperateRecord, 0),
		OperateArrays:  make([][]OperateType, rule.MaxPlayerCount),
		CurChairID:     -1,
		RestCardsCount: 9*4*3 + 4, // 剩余牌数，万筒条 + 4个红中
	}
	// 红中 8 个
	if rule.GameFrameType == HongZhong8 {
		g.RestCardsCount = 9*3*4 + 8
	}
	return g
}

// 返回游戏数据、游戏场景，隐藏其他玩家的牌
func (g *GameFrame) GetGameData(session *remote.Session) any {
	userChairId := g.r.GetUsers()[session.GetUid()].ChairID
	var gameData GameData
	copier.CopyWithOption(gameData, g.gameData, copier.Option{DeepCopy: true, IgnoreEmpty: true})
	handCards := make([][]int, g.gameData.ChairCount)
	for chairId, cards := range g.gameData.HandCards {
		if chairId == userChairId {
			handCards[chairId] = cards
		} else {
			handCards[chairId] = make([]int, len(cards))
			// 每张牌置为 36 表示空牌
			for i := range handCards[chairId] {
				handCards[chairId][i] = 36
			}
		}
	}
	gameData.HandCards = handCards
	if g.gameData.GameStatus == GameStatusNone {
		gameData.RestCardsCount = 9*3*4 + 4
		if g.gameRule.GameFrameType == HongZhong8 {
			gameData.RestCardsCount = 9*3*4 + 8
		}
	}
	return gameData
}

func (g *GameFrame) StartGame(session *remote.Session, user *proto.RoomUser) {

}

func (g *GameFrame) GameMessageHandler(user *proto.RoomUser, session *remote.Session, msg []byte) {

}
