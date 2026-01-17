package logic

import (
	"fmt"
	"game/component/room"
	"math/rand"
	"sync"
	"time"
)

type UnionManager struct {
	sync.RWMutex
	unions map[int64]*Union
}

func NewUnionManager() *UnionManager {
	return &UnionManager{
		unions: make(map[int64]*Union),
	}
}

func (u *UnionManager) GetUnion(unionId int64) *Union {
	u.Lock()
	defer u.Unlock()
	union, ok := u.unions[unionId]
	if ok {
		return union
	}

	union = NewUnion(u)
	u.unions[unionId] = union
	return union
}

func (u *UnionManager) CreateRoomId() string {
	// 随机数的方式创建
	roomId := u.genRoomId()
	for _, union := range u.unions {
		if _, ok := union.Rooms[roomId]; ok {
			return u.CreateRoomId()
		}
	}
	return roomId
}

// 生成 6 位房间号
func (u *UnionManager) genRoomId() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	minVal, maxVal := int64(100000), int64(999999)
	roomIdInt := minVal + rand.Int63n(maxVal-minVal+1)
	return fmt.Sprintf("%d", roomIdInt)
}

func (u *UnionManager) GetRoomById(roomId string) *room.Room {
	for _, union := range u.unions {
		if r, ok := union.Rooms[roomId]; ok {
			return r
		}
	}
	return nil
}
