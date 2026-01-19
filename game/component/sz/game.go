package sz

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
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
		Winner:          make([]int, 0),
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
	case GameCompareNotify:
		g.OnGameCompare(user, session, req.Data.ChairID)
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
	// 在游戏中并且没输的玩家个数
	gameCount := 0
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairId(i) && !utils.Contains(g.gameData.Loser, i) {
			gameCount++
		}
	}
	if gameCount == 1 {
		// 游戏结束
		g.startResult(session)
	} else {
		// 2.推进座次
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

func (g *GameFrame) OnGameCompare(user *proto.RoomUser, session *remote.Session, chairID int) {
	// todo 1.先下分 跟注结束后 进行比牌

	// 2.比牌
	fromChairId := user.ChairID
	toChairId := chairID
	res := g.logic.CompareCards(g.gameData.HandCards[fromChairId], g.gameData.HandCards[toChairId])
	// 3.处理比牌结果，推送轮数、状态、比牌结果等信息
	if res == 0 {
		// 和牌：主动比牌者输
		res = -1
	}

	winner := -1
	loser := -1
	if res > 0 {
		winner = fromChairId
		loser = toChairId
	} else {
		winner = toChairId
		loser = fromChairId
	}
	g.ServerMessagePush(g.r.GetAllUid(), GameComparePushData(fromChairId, toChairId, winner, loser), session)
	g.gameData.UserStatusArray[winner] = Win
	g.gameData.UserStatusArray[loser] = Lose
	g.gameData.Winner = append(g.gameData.Winner, winner)
	g.gameData.Loser = append(g.gameData.Loser, loser)
	// todo 赢了后，继续和其他人比牌
	if winner == fromChairId {

	}
	g.endPourScore(session)
}

func (g *GameFrame) startResult(session *remote.Session) {
	// 推送游戏结果状态
	g.gameData.GameStatus = Result
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(g.gameData.GameStatus, 0), session)

	//
	if g.gameResult == nil {
		g.gameResult = new(GameResult)
	}
	g.gameResult.Winners = g.gameData.Winner
	g.gameResult.HandCards = g.gameData.HandCards
	g.gameResult.CurScores = g.gameData.CurScores
	g.gameResult.Losers = g.gameData.Loser
	winScores := make([]int, g.gameData.ChairCount)
	for i := range winScores {
		if g.gameData.PourScores[i] != nil {
			score := 0
			for _, s := range g.gameData.PourScores[i] {
				score += s
			}
			winScores[i] = -score

			for winner := range g.gameData.Winner {
				winScores[winner] += score / len(g.gameData.Winner)
			}
		}
	}
	g.gameResult.WinScores = winScores
	g.ServerMessagePush(g.r.GetAllUid(), GameResultPushData(g.gameResult), session)
	// 重置游戏 开始下一把
	g.resetGame(session)
	g.gameEnd(session)
}

func (g *GameFrame) resetGame(session *remote.Session) {
	gameData := &GameData{
		GameType:        GameType(g.gameRule.GameFrameType),
		BaseScore:       g.gameRule.BaseScore,
		ChairCount:      g.gameRule.MaxPlayerCount,
		PourScores:      make([][]int, g.gameRule.MaxPlayerCount),
		HandCards:       make([][]int, g.gameRule.MaxPlayerCount),
		LookCards:       make([]int, g.gameRule.MaxPlayerCount),
		CurScores:       make([]int, g.gameRule.MaxPlayerCount),
		UserStatusArray: make([]UserStatus, g.gameRule.MaxPlayerCount),
		UserTrustArray:  []bool{false, false, false, false, false, false, false, false, false, false},
		Loser:           make([]int, 0),
		Winner:          make([]int, 0),
		GameStatus:      GameStatus(None),
	}
	// 重置 gameData
	g.gameData = gameData
	// 推送状态
	g.sendGameStatus(g.gameData.GameStatus, 0, session)
	// 重置房间数据
	g.r.EndGame(session)
}

func (g *GameFrame) sendGameStatus(gameStatus GameStatus, tick int, session *remote.Session) {
	g.ServerMessagePush(g.r.GetAllUid(), GameStatusPushData(gameStatus, tick), session)
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	// 赢家当庄家
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.gameResult.WinScores[i] > 0 {
			g.gameData.BankerChairID = i
			g.gameData.CurChairID = g.gameData.BankerChairID
		}
	}
	time.AfterFunc(5*time.Second, func() {
		for uid, _ := range g.r.GetUsers() {
			g.r.UserReady(uid, session)
		}
	})
}
