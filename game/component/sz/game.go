package sz

import (
	"common/utils"
	"framework/remote"
	"game/component/base"
	"game/component/proto"
)

type GameFrame struct {
	r        base.RoomFrame
	gameRule proto.GameRule
	gameData *GameData
	logic    *Logic
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
	return &GameData{
		GameType:        GameType(rule.GameFrameType),
		BaseScore:       rule.BaseScore,
		ChairCount:      rule.MaxPlayerCount,
		PourScores:      make([][]int, rule.MaxPlayerCount),
		HandCards:       make([][]int, rule.MaxPlayerCount),
		LookCards:       make([]int, rule.MaxPlayerCount),
		CurScores:       make([]int, rule.MaxPlayerCount),
		UserStatusArray: make([]UserStatus, rule.MaxPlayerCount),
		UserTrustArray:  []bool{false, false, false, false, false, false, false, false, false, false},
		Loser:           make([]int, 0),
	}
}

func (g *GameFrame) GetGameData() any {
	return g.gameData
}

func (g *GameFrame) ServerMessagePush(users []string, data any, session *remote.Session) {
	// 通过 nats 发送消息到 connector，connector 推送消息到客户端
	session.Push(users, data, "ServerMessagePush")
}

func (g *GameFrame) StartGame(session *remote.Session, user *proto.RoomUser) {
	// 1.用户信息的变更推送（金币的变化）
	uids := g.r.GetAllUid()
	g.ServerMessagePush(uids, UpdateUserInfoPushGold(user.UserInfo.Gold), session)
	// 2.庄家推送
	if g.gameData.CurBureau == 0 {
		g.gameData.BankerChairID = utils.Rand(len(uids))
	}
	// 设置庄家为当前操作的座次
	g.gameData.CurChairID = g.gameData.BankerChairID
	g.ServerMessagePush(uids, GameBankerPushData(g.gameData.BankerChairID), session)
	// 3.局数推送
	g.gameData.CurBureau++
	g.ServerMessagePush(uids, GameBureauPushData(g.gameData.CurBureau), session)
	// 4.游戏状态推送 分两步：(1) 推送发牌，(2) 下分推送：需要用户操作
	// 只推送状态为发牌 SendCards
	g.gameData.GameStatus = SendCards
	g.ServerMessagePush(uids, GameStatusPushData(g.gameData.GameStatus, 0), session)
	// 5.发牌推送
	g.sendCards(session)
	// 6.下分推送
	// 先推送下分状态
	g.gameData.GameStatus = PourScore
	g.ServerMessagePush(uids, GameStatusPushData(g.gameData.GameStatus, 0), session)
	// 再推送下分数据
	g.gameData.CurScore = g.gameRule.BaseScore * g.gameRule.AddScores[0]
	for _, u := range g.r.GetUsers() {
		g.ServerMessagePush(uids, GamePourScorePushData(u.ChairID, g.gameData.CurScore, g.gameData.CurScore, 1), session)
	}
	// 7.轮数推送
	g.gameData.Round = 1
	g.ServerMessagePush(uids, GameRoundPushData(g.gameData.Round), session)
	// 8.操作推送
	// ChairID 是当前可做操作的玩家的 chairId
	// 游戏开始时第一个可操作的座次是庄家位位置
	g.ServerMessagePush(uids, GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), session)
}

// 发牌
func (g *GameFrame) sendCards(session *remote.Session) {
	// 1.洗牌
	g.logic.washCards()
	// 2.发牌
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairId(i) {
			g.gameData.HandCards[i] = g.logic.getCards()
		}
	}
	// 3.推送手牌 如果没有看牌的话，就返回暗牌
	hands := make([][]int, g.gameData.ChairCount)
	for chair, cards := range g.gameData.HandCards {
		if cards != nil {
			// 暗牌
			hands[chair] = []int{0, 0, 0}
		}
	}
	g.ServerMessagePush(g.r.GetAllUid(), GameSendCardsPushData(hands), session)
}

func (g *GameFrame) IsPlayingChairId(chairId int) bool {
	for _, v := range g.r.GetUsers() {
		if v.UserStatus == proto.Playing && v.ChairID == chairId {
			return true
		}
	}
	return false
}
