package sz

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/proto"

	"github.com/jinzhu/copier"
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

// 返回游戏数据，用户已看牌，则返回牌。并且隐藏其他用户的牌
func (g *GameFrame) GetGameData(session *remote.Session) any {
	user := g.r.GetUsers()[session.GetUid()]
	// 深 copy
	var gameData GameData
	copier.CopyWithOption(&gameData, g.gameData, copier.Option{DeepCopy: true})
	for i := 0; i < g.gameData.ChairCount; i++ {
		// 如果用户发过牌
		if g.gameData.HandCards[i] != nil {
			// 隐藏掉牌
			gameData.HandCards[i] = make([]int, 3)
		} else {
			// 用户没有牌
			gameData.HandCards[i] = nil
		}
	}

	// 用户已经看牌了
	if g.gameData.LookCards[user.ChairID] == 1 {
		gameData.HandCards[user.ChairID] = g.gameData.HandCards[user.ChairID]
	}
	return gameData
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
		g.ServerMessagePush(uids, GamePourScorePushData(u.ChairID, g.gameData.CurScore, g.gameData.CurScore, 1, 0), session)
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

func (g *GameFrame) GameMessageHandler(user *proto.RoomUser, session *remote.Session, msg []byte) {
	// 1.解析参数
	var req MessageReq
	json.Unmarshal(msg, &req)
	// 2.根据不同的类型，触发不同的操作
	switch req.Type {
	case GameLookNotify:
		g.OnGameLook(user, session, req.Data.CuoPai)
	case GamePourScoreNotify:
		g.OnGamePourScore(user, session, req.Data.Score, req.Data.Type)
	}
}

// 看牌：给当前用户推送自己的牌，给其他用户推送此用户已看牌
func (g *GameFrame) OnGameLook(user *proto.RoomUser, session *remote.Session, cuopai bool) {
	// 当前游戏状态不是下分 || 当前可操作的玩家不是发送请求的玩家
	if g.gameData.GameStatus != PourScore || g.gameData.CurChairID != user.ChairID {
		logs.Warn("ID: %v room, sanzhang game look err：GameStatus=%v, chairId=%v", g.r.GetId(), g.gameData.GameStatus, user.ChairID)
		return
	}
	// 用户可能已经输了，不能再进行下分操作
	if !g.IsPlayingChairId(user.ChairID) {
		logs.Warn("ID: %v room, sanzhang game look err：user uid=%v not playing", g.r.GetId(), user.UserInfo.Uid)
		return
	}
	// 设置玩家状态为已看牌
	g.gameData.UserStatusArray[user.ChairID] = Look
	g.gameData.LookCards[user.ChairID] = 1 // 1.看牌
	// 推送消息
	for uid, ru := range g.r.GetUsers() {
		if uid == user.UserInfo.Uid {
			// 当前操作的用户
			g.ServerMessagePush([]string{uid}, GameLookPushData(g.gameData.CurChairID, g.gameData.HandCards[ru.ChairID], cuopai), session)
		} else {
			// 其他用户
			g.ServerMessagePush([]string{uid}, GameLookPushData(g.gameData.CurChairID, nil, cuopai), session)
		}
	}
}

// 处理下分 1.保存用户下的分数，推送下分数据到其他用户 2.结束下分，座次移动到下一位，推送游戏状态，推送操作的座次
func (g *GameFrame) OnGamePourScore(user *proto.RoomUser, session *remote.Session, score int, t int) {
	//1.保存用户下的分数，推送下分数据到其他用户
	// 当前游戏状态不是下分 || 当前可操作的玩家不是发送请求的玩家
	if g.gameData.GameStatus != PourScore || g.gameData.CurChairID != user.ChairID {
		logs.Warn("ID: %v room, sanzhang game pour score err：GameStatus=%v, chairId=%v", g.r.GetId(), g.gameData.GameStatus, user.ChairID)
		return
	}
	// 用户可能已经输了，不能再进行下分操作
	if !g.IsPlayingChairId(user.ChairID) {
		logs.Warn("ID: %v room, sanzhang game pour score err：user uid=%v not playing", g.r.GetId(), user.UserInfo.Uid)
		return
	}
	if score < 0 {
		logs.Warn("ID: %v room, sanzhang game pour score err：score < 0", g.r.GetId())
		return
	}
	if g.gameData.PourScores[user.ChairID] == nil {
		g.gameData.PourScores[user.ChairID] = make([]int, 0)
	}
	// 用户的下分记录
	g.gameData.PourScores[user.ChairID] = append(g.gameData.PourScores[user.ChairID], score)
	// 所有人的分数
	scores := 0
	for _, pourScore := range g.gameData.PourScores {
		if pourScore != nil {
			for _, s := range pourScore {
				scores += s
			}
		}
	}
	// 当前座次的总分
	chairScore := 0
	for _, s := range g.gameData.PourScores[user.ChairID] {
		chairScore += s
	}
	g.ServerMessagePush(g.r.GetAllUid(), GamePourScorePushData(g.gameData.CurChairID, score, chairScore, scores, t), session)

	// 2.结束下分，座次移动到下一位，推送游戏状态，推送操作的座次
	g.endPourScore(session)
}

// 结束下分，座次推进，推送游戏状态，推送操作的座次
func (g *GameFrame) endPourScore(session *remote.Session) {
	// 1.推送轮次 todo 轮数大于规则的限制 结束游戏 进行结算
	round := g.getCurRound()
	g.ServerMessagePush(g.r.GetAllUid(), GameRoundPushData(round), session)
	// 推进座次
	for i := 0; i < g.gameData.ChairCount; i++ {
		g.gameData.CurChairID++
		g.gameData.CurChairID = g.gameData.CurChairID % g.gameData.ChairCount
		if g.IsPlayingChairId(g.gameData.CurChairID) {
			break
		}
	}
	// 推送游戏状态
	g.gameData.GameStatus = PourScore
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(g.gameData.GameStatus, 30), session)
	// 推送操作
	// ChairID 是当前可做操作的玩家的 chairId
	g.ServerMessagePush(g.r.GetAllUid(), GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), session)
}

// 获取当前轮次
func (g *GameFrame) getCurRound() int {
	cur := g.gameData.CurChairID
	for i := 0; i < g.gameData.ChairCount; i++ {
		cur++
		cur = cur % g.gameData.ChairCount
		if g.IsPlayingChairId(cur) {
			return len(g.gameData.PourScores[cur])
		}
	}
	return len(g.gameData.PourScores[g.gameData.CurChairID])
}
