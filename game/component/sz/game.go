package sz

import (
	"game/component/base"
	"game/component/proto"
)

type GameFrame struct {
	r        base.RoomFrame
	gameRule proto.GameRule
	gameData *GameData
}

func NewGameFrame(rule proto.GameRule, room base.RoomFrame) *GameFrame {
	return &GameFrame{
		r:        room,
		gameRule: rule,
		gameData: initGameData(rule),
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
