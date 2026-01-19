package sz

import (
	"common/utils"
	"sort"
	"sync"
)

type Logic struct {
	sync.RWMutex
	cards []int // 赢三张有 52 张牌
}

func NewLogic() *Logic {
	return &Logic{
		cards: make([]int, 0),
	}
}

// washCards 洗牌
// 1-13 => A,2,3,4,5,6,7,8,9,10,J,Q,K => 0x01 ~ 0x0d
// 不同花色 方块、梅花、红桃、黑桃 => 0x0n, 0x1n, 0x2n, 0x3n
//
// 即：高 4 位花色索引，低 4 位牌面点数索引
//
//	{
//			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d,
//			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d,
//			0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d,
//			0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d,
//	}
//
// 顺序是：方块、梅花、红桃、黑桃
func (l *Logic) washCards() {
	l.cards = []int{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d,
		0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d,
		0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d,
	}
	for i, v := range l.cards {
		random := utils.Rand(len(l.cards))
		l.cards[i] = l.cards[random]
		l.cards[random] = v
	}
}

// getCards 获取三张手牌
func (l *Logic) getCards() []int {
	cards := make([]int, 3)
	l.RLock()
	defer l.RUnlock()
	for i := 0; i < 3; i++ {
		if len(cards) == 0 {
			break
		}
		card := l.cards[len(l.cards)-1]
		l.cards = l.cards[:len(l.cards)-1]
		cards[i] = card
	}
	return cards
}

// 比牌：0 合牌，大于 0 胜，小于 0 败
func (l *Logic) CompareCards(from []int, to []int) int {
	// 获取牌类型
	fromType := l.getCardsType(from)
	toType := l.getCardsType(to)
	if fromType != toType {
		return int(fromType - toType)
	}
	// 类型相等 比较牌面大小
	if fromType == DuiZi {
		duiFrom, danFrom := l.getDuizi(from)
		duiTo, danTo := l.getDuizi(from)
		if duiFrom != duiTo {
			return duiFrom - duiTo
		}
		return danFrom - danTo
	}

	sortFrom := l.orderCardValues(from)
	sortTo := l.orderCardValues(to)
	if sortFrom[2] != sortTo[2] {
		return sortFrom[2] - sortTo[2]
	}
	if sortFrom[1] != sortTo[1] {
		return sortFrom[1] - sortTo[1]
	}
	if sortFrom[0] != sortTo[0] {
		return sortFrom[0] - sortTo[0]
	}
	return 0
}

func (l *Logic) getCardsType(cards []int) CardsType {
	// 1.豹子 牌面值相等
	one := l.getCardNumber(cards[0])
	two := l.getCardNumber(cards[1])
	three := l.getCardNumber(cards[2])
	if one == two && two == three {
		return BaoZi
	}

	// 2.金花 花色相同，还需要判断顺金
	jinhua := false
	oneColor := l.getCardsColor(cards[0])
	twoColor := l.getCardsColor(cards[1])
	threeColor := l.getCardsColor(cards[2])
	if oneColor == twoColor && twoColor == threeColor {
		jinhua = true
	}

	// 3.判断顺子 先排序 有两个特殊的顺子：A23  QKA
	shunzi := false
	sorts := l.orderCardValues(cards)
	oneS := sorts[0]
	twoS := sorts[1]
	threeS := sorts[2]
	if one+1 == twoS && twoS+1 == threeS {
		shunzi = true
	}
	// 排完序所以 A23 => [2,3,14]
	if oneS == 2 && twoS == 3 && threeS == 14 {
		shunzi = true
	}
	if jinhua && shunzi {
		return ShunJin
	}
	if jinhua {
		return JinHua
	}
	if shunzi {
		return ShunZi
	}

	// 4.对子
	if oneS == twoS || twoS == threeS {
		return DuiZi
	}

	// 5.单牌
	return DanZhang
}

func (l *Logic) orderCardValues(cards []int) []int {
	sorts := make([]int, len(cards))
	for i, card := range cards {
		// 转成 2-14 的牌面值
		sorts[i] = l.getCardValue(card)
	}
	sort.Ints(sorts)
	return sorts
}

// 获取牌面值 1-13，A 是 1
func (l *Logic) getCardNumber(card int) int {
	// 0x0f => 0000 1111
	// 0000 1111 & card 即只保留低四位
	return card & 0x0f
}

// 获取牌面值，但是转成 2-14
func (l *Logic) getCardValue(card int) int {
	// 转成 1-13 的值
	number := l.getCardNumber(card)
	// 如果是 A
	if number == 1 {
		number += 13
	}
	return number
}

// 获取牌的花色
// 1-13 => A,2,3,4,5,6,7,8,9,10,J,Q,K => 0x01 ~ 0x0d
// 不同花色 方块、梅花、红桃、黑桃 => 0x0n, 0x1n, 0x2n, 0x3n
//
//	{
//			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d,
//			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d,
//			0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d,
//			0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d,
//	}
//
// 前 13 张牌是 方块，顺序是：方块、梅花、红桃、黑桃。
func (l *Logic) getCardsColor(card int) string {
	colors := []string{"方块", "梅花", "红桃", "黑桃"}
	// 花色取高 4 位 除以 0x10=16 即右移 4位。十进制数除以10，即右移一位 120/10 => 12
	if card >= 0x01 && card <= 0x3d {
		return colors[card/0x10]
	}
	return ""
}

// 获取对子重复的一个，和单牌
func (l *Logic) getDuizi(from []int) (int, int) {
	// AAB || BAA
	values := l.orderCardValues(from)
	if values[0] == values[1] {
		return values[0], values[2]
	}
	return values[1], values[0]
}
