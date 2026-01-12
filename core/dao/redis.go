package dao

import (
	"context"
	"core/repo"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const Prefix = "MSQP"
const AccountIdRedisKey = "AccountId"
const AccountIdBegin = 10000

type RedisDao struct {
	repo *repo.Manager
}

// 自增
func (r RedisDao) NextAccountId() (string, error) {
	// 自增给一个前缀
	return r.Increment(Prefix + ":" + AccountIdRedisKey)
}

func (r RedisDao) Increment(key string) (string, error) {
	ctx := context.TODO()
	// 判断是单机模式还是集群
	var c redis.Cmdable
	if r.repo.Redis.Clt != nil {
		c = r.repo.Redis.Clt
	} else {
		c = r.repo.Redis.ClusterClt
	}

	// 判断此 key 是否存在，不存在 set，否则自增
	// Result() 返回 0 表示不存在
	result, err := c.Exists(ctx, key).Result()
	exists := result
	if exists == 0 {
		// 不存在
		err = c.Set(ctx, key, AccountIdBegin, 0).Err()
		if err != nil {
			return "", err
		}
	}
	id, err := c.Incr(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

func NewRedisDao(m *repo.Manager) *RedisDao {
	return &RedisDao{
		repo: m,
	}
}
