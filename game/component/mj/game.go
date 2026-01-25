package mj

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/mj/mp"
	"game/component/proto"
	"sync"
	"time"

	"github.com/jinzhu/copier"
)

type GameFrame struct {
	sync.RWMutex
	r          base.RoomFrame
	gameRule   proto.GameRule
	gameData   *GameData
	logic      *Logic
	gameResult *GameResult
	testCards  []mp.CardID
}

func NewGameFrame(rule proto.GameRule, room base.RoomFrame) *GameFrame {
	return &GameFrame{
		r:         room,
		gameRule:  rule,
		gameData:  initGameData(rule),
		logic:     NewLogic(GameType(rule.GameFrameType), rule.Qidui),
		testCards: make([]mp.CardID, rule.MaxPlayerCount),
	}
}

func initGameData(rule proto.GameRule) *GameData {
	g := &GameData{
		ChairCount:     rule.MaxPlayerCount,
		HandCards:      make([][]mp.CardID, rule.MaxPlayerCount),
		GameStatus:     GameStatusNone,
		OperateRecord:  make([]OperateRecord, 0),
		OperateArrays:  make([][]OperateType, rule.MaxPlayerCount),
		CurChairID:     -1,
		RestCardsCount: 9*4*3 + 4, // 剩余牌数，万筒条 + 4个红中
		Result:         nil,
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
	handCards := make([][]mp.CardID, g.gameData.ChairCount)
	for chairId, cards := range g.gameData.HandCards {
		if chairId == userChairId {
			handCards[chairId] = cards
		} else {
			handCards[chairId] = make([]mp.CardID, len(cards))
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
	case GameGetCardNotify:
		g.onGameGetCard(user, session, req.Data)
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
		handCards := make([][]mp.CardID, g.gameData.ChairCount)
		for i := range handCards {
			if i == user.ChairID {
				handCards[i] = g.gameData.HandCards[i]
			} else {
				handCards[i] = make([]mp.CardID, len(g.gameData.HandCards[i]))
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
	// 拿 test 牌
	card := g.testCards[chairID]
	if card > 0 && card < 36 {
		// 从牌堆拿指定的牌
		card = g.logic.getCard(card)
		g.testCards[chairID] = 0
	}
	if card <= 0 || card >= 36 {
		// 摸一张牌
		cards := g.logic.getCards(1)
		if cards == nil || len(cards) == 0 {
			return
		}
		card = cards[0]
	}
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
func (g *GameFrame) getMyOperateArray(chairID int, card mp.CardID, session *remote.Session) []OperateType {
	var operateArray = []OperateType{Qi}
	if g.logic.canHu(g.gameData.HandCards[chairID], -1) {
		operateArray = append(operateArray, HuZhi)
	}
	cardCount := 0
	for _, c := range g.gameData.HandCards[chairID] {
		if c == card {
			cardCount++
		}
	}
	if cardCount == 4 {
		// 自摸杠
		operateArray = append(operateArray, GangZhi)
	}
	// 已经碰了，然后又摸到一张，就可以补杠
	for _, record := range g.gameData.OperateRecord {
		if record.ChairID == chairID && record.Operate == Peng && record.Card == card {
			operateArray = append(operateArray, GangBu)
		}
	}

	return operateArray
}

func (g *GameFrame) onGameChat(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.ServerMessagePush(g.r.GetAllUid(), GameChatPushData(user.ChairID, data.Type, data.Msg, data.RecipientID), session)
}

func (g *GameFrame) onGameTurnOperate(user *proto.RoomUser, session *remote.Session, data MessageData) {
	switch data.Operate {
	case Qi:
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
	case Peng:
		// 碰一张牌，出一张牌
		// 1.当前用户的操作是否成功 告诉所有人
		// 前端发送 0 代表用户出的上一张牌
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				// 没记录，出错
				logs.Error("用户碰操作，但是没有上一个用户操作")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		// 碰的牌加进来，相当于摸了一张牌 => 14 张，所以接着要打出一张
		// 添加弃牌操作
		g.gameData.OperateArrays[user.ChairID] = []OperateType{Qi}
		// 碰相当于损失 两张牌。当用户重新加入房间时，会重新加载数据，碰掉的牌不能在手牌
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 2)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		// 2.让用户出牌
		g.gameData.CurChairID = user.ChairID
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnPushData(user.ChairID, 0, OperateTime, g.gameData.OperateArrays[user.ChairID]), session)
	case GangChi:
		// 杠一张牌，出一张牌
		// 1.当前用户的操作是否成功 告诉所有人
		// 前端发送 0 代表用户出的上一张牌
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				// 没记录，出错
				logs.Error("用户杠操作，但是没有上一个用户操作")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		// 碰的牌加进来，相当于摸了一张牌 => 14 张，所以接着要打出一张
		// 添加弃牌操作
		g.gameData.OperateArrays[user.ChairID] = []OperateType{Qi}
		// 碰相当于损失 3 张牌。当用户重新加入房间时，会重新加载数据，碰掉的牌不能在手牌
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 3)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		// 2.让用户出牌
		g.gameData.CurChairID = user.ChairID
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnPushData(user.ChairID, 0, OperateTime, g.gameData.OperateArrays[user.ChairID]), session)
	case HuChi:
		// 发送用户的操作
		// 前端发送 0 代表用户出的上一张牌
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				// 没记录，出错
				logs.Error("用户吃胡操作，但是没有上一个用户操作")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		g.gameData.OperateArrays[user.ChairID] = nil
		// 2.让用户出牌
		g.gameData.CurChairID = user.ChairID
		// 结束
		g.gameEnd(session)
	case HuZhi:
		// 一定是用户先摸牌
		// 发送用户的操作
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		g.gameData.OperateArrays[user.ChairID] = nil
		// 2.让用户出牌
		g.gameData.CurChairID = user.ChairID
		// 结束
		g.gameEnd(session)
	case GangZhi:
		// 自摸杠
		// 1.当前用户的操作是否成功 告诉所有人， 前端传的 data.Card 会是一个 nil，相当于暗杠
		// 获取 card
		data.Card = g.gameData.HandCards[user.ChairID][len(g.gameData.HandCards[user.ChairID])-1]
		// 推送玩家的操作，其他用户不知道暗杠的什么牌，只有当前用户能看到自己的自摸杠
		for uid, _ := range g.r.GetUsers() {
			if uid == user.UserInfo.Uid {
				g.ServerMessagePush([]string{uid}, GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
			} else {
				// card = 0
				g.ServerMessagePush([]string{uid}, GameTurnOperatePushData(user.ChairID, 0, data.Operate, true), session)
			}
		}
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 4)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		// 不需要再弃牌，摸牌，继续操作
		g.setTurn(user.ChairID, session)
	case Guo:
		g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
		// todo 如果牌 14，先弃牌再做其他的
		// 继续操作
		g.setTurn(user.ChairID, session)
	case GangBu:
		// 1.自摸补杠
		if user.ChairID == g.gameData.CurChairID {
			card := g.gameData.HandCards[user.ChairID][len(g.gameData.HandCards[user.ChairID])-1]
			for uid, _ := range g.r.GetUsers() {
				if uid == user.UserInfo.Uid {
					g.ServerMessagePush([]string{uid}, GameTurnOperatePushData(user.ChairID, card, data.Operate, true), session)
				} else {
					// card = 0
					g.ServerMessagePush([]string{uid}, GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
				}
			}
			g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], card, 1)
			g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: card, Operate: data.Operate})
			// 继续操作
			g.setTurn(user.ChairID, session)
		} else {
			// 2.吃牌补杠
			if data.Card == 0 {
				length := len(g.gameData.OperateRecord)
				if length == 0 {
					// 没记录，出错
					logs.Error("用户杠操作，但是没有上一个用户操作")
				} else {
					data.Card = g.gameData.OperateRecord[length-1].Card
				}
			}
			g.ServerMessagePush(g.r.GetAllUid(), GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
			g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{ChairID: user.ChairID, Card: data.Card, Operate: data.Operate})
			// 继续操作
			g.setTurn(user.ChairID, session)
		}
	default:
		logs.Warn("非法操作")
	}
}

func (g *GameFrame) delCards(cards []mp.CardID, card mp.CardID, times int) []mp.CardID {
	g.Lock()
	defer g.Unlock()
	newCards := make([]mp.CardID, 0)
	for _, v := range cards {
		if v == card && times > 0 {
			// 删除一次
			times--
			continue
		}
		newCards = append(newCards, v) // 保留
	}
	return newCards
}

func (g *GameFrame) nextTurn(lastCard mp.CardID, session *remote.Session) {
	// 在下一个用户摸牌之前，判断其他玩家在这个 card 上可以做的操作：胡、碰、杠...
	hasOtherOp := false
	if lastCard > 0 && lastCard < 36 {
		for i := 0; i < g.gameData.ChairCount; i++ {
			// 跳过当前玩家
			if i == g.gameData.CurChairID {
				continue
			}
			operateArray := g.logic.getOperateArray(g.gameData.HandCards[i], lastCard)
			// 已经碰了，然后当前出牌的玩家又打出一张牌，就可以补杠
			for _, record := range g.gameData.OperateRecord {
				if record.ChairID == i && record.Operate == Peng && record.Card == lastCard {
					operateArray = append(operateArray, GangBu)
				}
			}
			if len(operateArray) > 0 {
				// 用户可以做一些操作
				hasOtherOp = true
				// 通知用户可以操作，因为不用摸牌，这里通知到所有用户（但是其他用户就知道此用户有哪些操作了），但是只有 chairID=i 有对应操作
				g.ServerMessagePush(g.r.GetAllUid(), GameTurnPushData(i, lastCard, OperateTime, operateArray), session)
				g.gameData.OperateArrays[i] = operateArray
			}
		}
	}
	if !hasOtherOp {
		// 简单的让下一个用户摸牌
		nextTurnID := (g.gameData.CurChairID + 1) % g.gameData.ChairCount
		g.setTurn(nextTurnID, session)
	}
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	g.gameData.GameStatus = Result
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(g.gameData.GameStatus, 0), session)
	scores := make([]int, g.gameData.ChairCount)
	// 结算推送
	for i := 0; i < g.gameData.ChairCount; i++ {

	}
	length := len(g.gameData.OperateRecord)
	if length <= 0 {
		logs.Error("没有操作记录，不能结束游戏")
		return
	}
	lastOp := g.gameData.OperateRecord[length-1]
	// 还有一种情况是：牌摸完了
	count := g.logic.getRestCardsCount()
	if lastOp.Operate != HuChi && lastOp.Operate != HuZhi && count > 0 {
		logs.Error("没有糊牌，并且还有剩余的牌没摸，不能结束")
		return
	}
	result := GameResult{
		Scores:          scores,
		HandCards:       g.gameData.HandCards,
		RestCards:       g.logic.getRestCards(),
		WinChairIDArray: []int{lastOp.ChairID},
		HuType:          lastOp.Operate,
		// 需要一个默认值，防止前端报错。目前模式下用不到这两个
		MyMaCards:     []MyMaCard{},
		FangGangArray: []int{},
	}
	g.gameData.Result = &result
	g.ServerMessagePush(g.r.GetAllUid(), GameResultPushData(result), session)

	time.AfterFunc(3*time.Second, func() {
		g.r.EndGame(session)
		g.resetGame(session)
	})
	// 倒计时 30 秒，如果用户未准备，自动准备或踢出房间
}

// 重置游戏数据
func (g *GameFrame) resetGame(session *remote.Session) {
	g.gameData = initGameData(g.gameRule)
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(g.gameData.GameStatus, 0), session)
	// 推送剩余牌数
	g.ServerMessagePush(g.r.GetAllUid(), GameRestCardsCountPushData(g.logic.getRestCardsCount()), session)
}

func (g *GameFrame) onGameGetCard(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.testCards[user.ChairID] = data.Card
}
