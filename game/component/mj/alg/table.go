package alg

import (
	"strconv"
)

// 字典表
// A、B、C 叫非字牌 1-9 都一样，所以生成一份即可。D 生成一种（没有连子）
// 穷举所有胡牌可能 存入 Table0， D TableFeng0
// 有1个鬼 将Table0中的数据替换一个存入 Table1
// 有2个鬼 将Table0中的数据替换一个存入 Table2
// 有3个鬼 将Table0中的数据替换一个存入 Table3
// 有4个鬼 将Table0中的数据替换一个存入 Table4
// 有5个鬼 将Table0中的数据替换一个存入 Table5
// 有6个鬼 将Table0中的数据替换一个存入 Table6
// 有7个鬼 将Table0中的数据替换一个存入 Table7
type Table struct {
	keyDic        map[string]bool         // Table0 非字牌(不是feng) ABC 无鬼 字典
	keyGuiDic     map[int]map[string]bool // 非字牌 ABC 有鬼 字典
	keyFengDic    map[string]bool         // 字牌 D 无鬼 字典
	keyFengGuiDic map[int]map[string]bool // 字牌 D 有鬼 字典
}

func NewTable() *Table {
	t := &Table{
		keyDic:        make(map[string]bool),
		keyGuiDic:     make(map[int]map[string]bool),
		keyFengDic:    make(map[string]bool),
		keyFengGuiDic: make(map[int]map[string]bool),
	}
	t.gen()
	return t
}

func (t *Table) gen() {
	cards := []int{0, 0, 0, 0, 0, 0, 0, 0, 0}
	level := 0
	t.genTableNoGui(cards, level, false, false)
	t.genTableGui(false)
	// feng
	t.genTableNoGui(cards, level, false, true)
	t.genTableGui(true)
	t.save()
}

// 生成的表是：将可选的，即 N*连子+M*刻子+1*将（可选）
func (t *Table) genTableNoGui(cards []int, level int, jiang bool, feng bool) {
	// 14 张牌，最多递归 5 次，3+3+3+3+2
	// len(cards) == 9
	for i := 0; i < 9; i++ {
		// feng 东南西北中发白 7*4 张牌  春夏秋冬梅兰竹菊（没用的牌）
		if feng && i > 6 {
			continue
		}
		// 1.先将 cards 中牌数量计算出来，后续判断用
		totalCardCount := t.calTotalCardCount(cards)
		// AAA刻子 避免超过 14 个牌。   每张牌只能有四个，每次放三张牌，所以 card[i] 这个位置最多有 1 个
		if totalCardCount <= 11 && cards[i] <= 1 {
			cards[i] += 3
			// cards 转成 string
			key := t.genKey(cards)
			if feng {
				t.keyFengDic[key] = true
			} else {
				t.keyDic[key] = true
			}
			// 14 张牌，最多递归 5 次
			if level < 5 {
				t.genTableNoGui(cards, level+1, jiang, feng)
			}
			cards[i] -= 3
		}

		// ABC连子 feng 是字牌 不能放连子
		if !feng && totalCardCount <= 11 && i < 7 && cards[i] <= 3 && cards[i+1] <= 3 && cards[i+2] <= 3 {
			cards[i] += 1
			cards[i+1] += 1
			cards[i+2] += 1
			// cards 转成 string
			key := t.genKey(cards)
			t.keyDic[key] = true
			if level < 5 {
				t.genTableNoGui(cards, level+1, jiang, feng)
			}
			cards[i] -= 1
			cards[i+1] -= 1
			cards[i+2] -= 1
		}
		// AA将
		if !jiang && totalCardCount <= 12 && cards[i] <= 2 {
			cards[i] += 2
			// cards 转成 string
			key := t.genKey(cards)
			if feng {
				t.keyFengDic[key] = true
			} else {
				t.keyDic[key] = true
			}
			if level < 5 {
				t.genTableNoGui(cards, level+1, true, feng)
			}
			cards[i] -= 2
		}
	}
}

func (t *Table) genTableGui(feng bool) {
	dic := t.keyDic
	if feng {
		dic = t.keyFengDic
	}
	// 每个数字分别使用gui牌替换
	for k := range dic {
		cards := t.toNumberArray(k)
		t.genGui(cards, 1, feng)
	}
}

func (t *Table) genGui(cards []int, guiCount int, feng bool) {
	for i := 0; i < 9; i++ {
		// 没有这个牌
		if cards[i] == 0 {
			continue
		}
		// 替换掉这个牌
		cards[i]--
		if !t.tryAdd(cards, guiCount, feng) {
			cards[i]++
			continue
		}
		if guiCount < 8 {
			t.genGui(cards, guiCount+1, feng)
		}
		// 恢复
		cards[i]++
	}
}

func (t *Table) calTotalCardCount(cards []int) int {
	count := 0
	for _, card := range cards {
		count += card
	}
	return count
}

// cards 转成 string
func (t *Table) genKey(cards []int) string {
	key := ""
	dic := []string{"0", "1", "2", "3", "4"}
	for _, v := range cards {
		key += dic[v]
	}
	return key
}

func (t *Table) save() {
	//fmt.Println(t.keyDic)
}

func (t *Table) toNumberArray(k string) []int {
	cards := make([]int, len(k))
	for i := 0; i < len(k); i++ {
		cards[i], _ = strconv.Atoi(k[i : i+1])
	}
	return cards
}

func (t *Table) tryAdd(cards []int, count int, feng bool) bool {
	for i := 0; i < 9; i++ {
		if cards[i] < 0 || cards[i] > 4 {
			return false
		}
	}

	key := t.genKey(cards)
	dic := t.keyGuiDic
	if feng {
		dic = t.keyFengGuiDic
	}
	if dic[count] == nil {
		dic[count] = make(map[string]bool)
	}
	if ok := dic[count][key]; ok {
		return false
	}
	dic[count][key] = true
	return true
}
