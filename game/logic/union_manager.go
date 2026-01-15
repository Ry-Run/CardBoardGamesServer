package logic

import "sync"

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
