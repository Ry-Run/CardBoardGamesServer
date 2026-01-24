package alg

import (
	"game/component/mj/mp"
)

type HuLogic struct {
	table *Table
}

func NewHuLogic() *HuLogic {
	return &HuLogic{
		table: NewTable(),
	}
}

type HuData struct {
	cards    []int
	feng     bool
	guiCount int  // 鬼牌数量
	jiang    bool // 是否用将
}

// cards 玩家当前的手牌；guiList 鬼牌可以代替任意一种牌（目前是红中）；card 玩家拿牌/吃牌
// A 万(1-9 * 4)，B 筒(1-9 * 4)，C 条(1-9 * 4)，D 风 (东南西北 白板 发财等，1-9 * 4)
// A+B+C+D(红中) = 36+36+36+红中数量
// 胡牌逻辑：N*连子+M*刻子（比对子多一个: 2,2,2）+1*将（对子: 3,3）(N 和 M 不能同时为 0)
// 注意：将可以跨门使用，例如：万的将给条使用
// 例如 14 张牌：1A 2A 3A(连子) 4A 4A 4A(刻子) 6A 6A 6A(刻子) 2B 3B 4B(连子) 5C 5C(将)
// 算法：
// 1.编码，例如 1-9A => 000000000(9位) 每个位置代表牌有几个 1A 2A 3A 4A 4A 4A 6A 6A 6A (1万、2万、3万分别一个，4万、6万分别3个) => 111303000
// 2.生成胡牌信息：上面的 111303000 编码满足公式：1*连子+1刻子+1将，所以胡了。穷举所有可能的糊牌排列，转换成编码
// 1A2A5A5A => 110020000 如果 0 鬼，可以胡 3A。如果 1 鬼（但是无将） 胡3A、5A。有将 直接胡
// 鬼0-7 都有 n 种可能，8个鬼直接胡，因此0-7需要生成 8 张表 直接存入内存中
// 将可选（有的玩法，将是可选的，不是必须一个）！！！
func (h *HuLogic) CheckHu(cards mp.CardIDs, guiList []mp.CardID, card mp.CardID) bool {
	// 牌有效，并且不能超过 14 张牌的限制
	if card > 0 && card < 36 && len(cards) < 14 {
		cards = append(cards, card)
	}
	return h.isHu(cards, guiList)
}

func (h *HuLogic) isHu(cards mp.CardIDs, guiList []mp.CardID) bool {
	// 统计 A B C D 数量，转成编码
	cardList := [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	guiCount := 0
	for _, card := range cards {
		// 统计鬼牌数量
		if IndexOf(guiList, card) != -1 {
			guiCount++
		} else {
			intCard := int(card)
			// 属于 A B C D 哪个种牌
			i := intCard / 10
			j := intCard%10 - 1
			cardList[i][j]++
		}
	}
	data := HuData{
		guiCount: guiCount,
		jiang:    false,
	}
	for i, ints := range cardList {
		feng := i == 3
		data.cards = ints
		data.feng = feng
		if !h.CheckCards(&data, 0) {
			return false
		}
	}
	// 没有哪一门占用了将，然后 鬼牌先代替 连子和刻子，最后还剩两张鬼牌，这两个鬼牌就当做将
	if !data.jiang && data.guiCount%3 == 2 {
		return true
	}
	// 某一门占用了将，然后 鬼牌代替连子和刻子没有多余的话，就代表组成的牌面可以胡
	if data.jiang && data.guiCount%3 == 0 {
		return true
	}
	return false
}

// findTable 检查牌型是否能拆成：N*连子 + M*刻子 + 1*将
func (h *HuLogic) CheckCards(data *HuData, guiCount int) bool {
	totalCardCount := h.table.calTotalCardCount(data.cards)
	if totalCardCount == 0 {
		return true
	}
	// 编码
	k := h.table.genKey(data.cards)
	// 检查排牌型，是不是：N*连子+M*刻子+1*将（可选）
	if !h.findTable(k, guiCount, data.feng) {
		if guiCount < data.guiCount {
			return h.CheckCards(data, guiCount+1)
		} else {
			return false
		}
	} else {
		// 将规则（整手只能有一个将）
		// 本门就是当前为 万/筒/条/feng
		// 对于单门而言，若 (本门张数 + 本门使用的鬼) % 3 == 2，表示该门的某种拆分需要“将”(一对)。
		if (totalCardCount+guiCount)%3 == 2 {
			if !data.jiang {
				// 全局还没用过将：允许本门占用将，因为将可以跨门使用（万使用条的将...）
				data.jiang = true
			} else if guiCount < data.guiCount {
				// 全局将已被占用：尝试在本门多用 1 张鬼，让本门变成 %3==0（全面子，不再需要将）
				return h.CheckCards(data, guiCount+1)
			} else {
				// 鬼已无法再增加：本门仍必须要将，但整手将已占用 => 不可胡
				return false
			}
		}
		data.guiCount = data.guiCount - guiCount
	}
	return true
}

// findTable 检查「单门牌型」是否能拆成：N*连子 + M*刻子 +（可选）1*将。
// 注意：这里只判断这一门的可拆分性；整手“将只能有一个”的全局约束在 CheckCards/isHu 中处理。
func (h *HuLogic) findTable(k string, guiCount int, feng bool) bool {
	ok := false
	if !feng && guiCount == 0 {
		_, ok = h.table.keyDic[k]
	} else if !feng && guiCount > 0 {
		_, ok = h.table.keyGuiDic[guiCount][k]
	} else if feng && guiCount == 0 {
		_, ok = h.table.keyFengDic[k]
	} else if feng && guiCount > 0 {
		_, ok = h.table.keyFengGuiDic[guiCount][k]
	} else {
		ok = false
	}
	return ok
}

func IndexOf[T mp.CardID](list []T, v T) int {
	for i, val := range list {
		if val == v {
			return i
		}
	}
	return -1
}
