package mj

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	mjp "game/component/mj/mp"
	"game/component/proto"
	"time"

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
		logic:    NewLogic(GameType(rule.GameFrameType), rule.Qidui),
	}
}

func initGameData(rule proto.GameRule) *GameData {
	g := &GameData{
		ChairCount:     rule.MaxPlayerCount,
		HandCards:      make([][]mjp.CardID, rule.MaxPlayerCount),
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
	handCards := make([][]mjp.CardID, g.gameData.ChairCount)
	for chairId, cards := range g.gameData.HandCards {
		if chairId == userChairId {
			handCards[chairId] = cards
		} else {
			handCards[chairId] = make([]mjp.CardID, len(cards))
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

// 开始游戏：1.（摇骰子阶段）游戏状态修改并推送；2.庄家推送；3.摇骰子推送；4.发牌推送；10.局数推进推送
func (g *GameFrame) StartGame(session *remote.Session, user *proto.RoomUser) {
	// 1.（摇骰子阶段）游戏状态修改并推送
	g.gameData.GameStarted = true
	g.gameData.GameStatus = Dices
	g.sendGameStatus(g.gameData.GameStatus, GameStatusTmDices, session)
	// 2.庄家推送
	if g.gameData.CurBureau == 0 {
		g.gameData.BankerChairID = 0
	} else {
		// todo 赢家是庄家
	}
	g.ServerMessagePush(g.r.GetAllUid(), GameBankerPushData(g.gameData.BankerChairID), session)
	// 3.摇骰子推送
	dice1 := utils.Rand(6) + 1
	dice2 := utils.Rand(6) + 1
	g.ServerMessagePush(g.r.GetAllUid(), GameDicesPushData(dice1, dice2), session)
	// 4.发牌推送
	g.sendHandCards(session)

	// 10.局数推进推送
	g.gameData.CurBureau++
	g.ServerMessagePush(g.r.GetAllUid(), GameBureauPushData(g.gameData.CurBureau), session)
}

func (g *GameFrame) GameMessageHandler(user *proto.RoomUser, session *remote.Session, msg []byte) {
	var req MessageReq
	json.Unmarshal(msg, &req)
	switch req.Type {
	case GameChatNotify:
		g.onGameChat(user, session, req.Data)
	case GameTurnOperateNotify:
		g.onGameTurnOperate(user, session, req.Data)
	}
}

func (g *GameFrame) sendGameStatus(gameStatus GameStatus, tick int, session *remote.Session) {
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(gameStatus, tick), session)
}

func (g *GameFrame) ServerMessagePush(users []string, data any, session *remote.Session) {
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push(users, data, "ServerMessagePush")
}

// 5.剩余牌数推送；6.开始游戏状态推送；7.拿牌推送；8.剩余牌数推送；
func (g *GameFrame) sendHandCards(session *remote.Session) {
	// 洗牌
	g.logic.washCards()
	// 发牌，每个 13 张
	for i := 0; i < g.gameData.ChairCount; i++ {
		g.gameData.HandCards[i] = g.logic.getCards(13)
	}
	// 推送手牌
	for uid, user := range g.r.GetUsers() {
		handCards := make([][]mjp.CardID, g.gameData.ChairCount)
		for i := range handCards {
			if i == user.ChairID {
				handCards[i] = g.gameData.HandCards[i]
			} else {
				handCards[i] = make([]mjp.CardID, len(g.gameData.HandCards[i]))
				for index := range handCards[i] {
					handCards[i][index] = 36 // 表示空牌
				}
			}
		}
		// 推送
		g.ServerMessagePush([]string{uid}, GameSendCardsPushData(handCards, user.ChairID), session)
	}
	// 5.剩余牌数推送
	g.sendRestCardsCount(session)
	// 间隔 1 秒执行
	time.AfterFunc(time.Second, func() {
		// 6.开始游戏状态推送；
		g.gameData.GameStatus = Playing
		g.sendGameStatus(g.gameData.GameStatus, GameStatusTmPlay, session)
		// 玩家操作的时间
		g.setTurn(g.gameData.BankerChairID, session)
	})
}

func (g *GameFrame) setTurn(chairID int, session *remote.Session) {
	// 7.拿牌推送；
	g.gameData.CurChairID = chairID
	// 牌不能大于 14
	if len(g.gameData.HandCards[chairID]) >= 14 {
		logs.Warn("玩家已经拿过牌了")
		return
	}
	// 摸一张牌
	card := g.logic.getCards(1)[0]
	g.gameData.HandCards[chairID] = append(g.gameData.HandCards[chairID], card)
	// 给所有玩家推送 这个玩家拿到了一张牌，当前用户是明牌，其他玩家看到暗牌
	operateArray := g.getMyOperateArray(chairID, card, session)
	for uid, user := range g.r.GetUsers() {
		if user.ChairID == chairID {
			g.ServerMessagePush([]string{uid}, GameTurnPushData(user.ChairID, card, OperateTime, operateArray), session)
			// 确保玩家重连还有记录
			g.gameData.OperateArrays[user.ChairID] = operateArray
			g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{
				ChairID: user.ChairID,
				Card:    card,
				Operate: Get,
			})
		} else {
			// 暗牌
			g.ServerMessagePush([]string{uid}, GameTurnPushData(user.ChairID, 36, OperateTime, operateArray), session)
		}
	}
	// 8.剩余牌数推送；
	g.sendRestCardsCount(session)
}

// 剩余牌数推送
func (g *GameFrame) sendRestCardsCount(session *remote.Session) {
	restCardsCount := g.logic.getRestCardsCount()
	g.ServerMessagePush(g.r.GetAllUid(), GameRestCardsCountPushData(restCardsCount), session)
}

// 用户当前可操作的行为：杠、碰、糊、弃牌等
func (g *GameFrame) getMyOperateArray(chairID int, card mjp.CardID, session *remote.Session) []OperateType {
	var operateArray = []OperateType{Qi}
	if g.logic.canHu(g.gameData.HandCards[chairID], -1) {
		operateArray = append(operateArray, HuZhi)
	}
	return operateArray
}

func (g *GameFrame) onGameChat(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.ServerMessagePush(g.r.GetAllUid(), GameChatPushData(user.ChairID, data.Type, data.Msg, data.RecipientID), session)
}

func (g *GameFrame) onGameTurnOperate(user *proto.RoomUser, session *remote.Session, data MessageData) {
	if data.Operate == Qi {
		// 1.向所有人推送 当前用户的操作
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		// 删除这个牌
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 1)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{
			ChairID: user.ChairID,
			Card:    data.Card,
			Operate: Qi,
		})
		// 用户不能再操作，用户可操作列表置为 nil
		g.gameData.OperateArrays[user.ChairID] = nil
		// 到下一个用户操作
		g.nextTurn(data.Card, session)
	}
}

func (g *GameFrame) delCards(cards []mjp.CardID, card mjp.CardID, times int) []mjp.CardID {
	for i, v := range cards {
		if v == card && times > 0 {
			cards = append(cards[:i], cards[i+1:]...)
			times--
		}
	}
	return cards
}

func (g *GameFrame) nextTurn(card mjp.CardID, session *remote.Session) {
	// todo 可以让下一个用户判断 card 可以做的操作：胡、碰、杠...
	// 简单的让下一个用户摸牌
	nextTurnID := (g.gameData.CurChairID + 1) % g.gameData.ChairCount
	g.setTurn(nextTurnID, session)
}
