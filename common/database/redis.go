package database

import (
	"common/config"
	"common/logs"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisManager struct {
	Clt        *redis.Client        // 单机
	ClusterClt *redis.ClusterClient // 集群
	// redis.NewUniversalClient(...) 可以统一单机、集群模式
}

func NewRedis() *RedisManager {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Conf.Database.MongoConf.Timeout)*time.Second)
	defer cancel()

	var clt *redis.Client
	var clusterClt *redis.ClusterClient
	if len(config.Conf.Database.RedisConf.ClusterAddrs) == 0 {
		// 单节点
		clt = redis.NewClient(&redis.Options{
			Addr:         config.Conf.Database.RedisConf.Addr,
			PoolSize:     config.Conf.Database.RedisConf.PoolSize,
			MinIdleConns: config.Conf.Database.RedisConf.MinIdleConns,
			Password:     config.Conf.Database.RedisConf.Password,
		})
	} else {
		clusterClt = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Conf.Database.RedisConf.ClusterAddrs,
			PoolSize:     config.Conf.Database.RedisConf.PoolSize,
			MinIdleConns: config.Conf.Database.RedisConf.MinIdleConns,
			Password:     config.Conf.Database.RedisConf.Password,
		})
	}

	if clusterClt != nil {
		if err := clusterClt.Ping(ctx); err != nil {
			logs.Fatal("redis cluster connect err:%v", err)
			return nil
		}
	}

	if clt != nil {
		if err := clt.Ping(ctx); err != nil {
			logs.Fatal("redis single connect err:%v", err)
			return nil
		}
	}

	return &RedisManager{
		Clt:        clt,
		ClusterClt: clusterClt,
	}
}

func (r *RedisManager) Close() {
	if r.ClusterClt != nil {
		if err := r.ClusterClt.Close(); err != nil {
			logs.Error("redis cluster close err:%v", err)
		}
	}

	if r.Clt != nil {
		if err := r.Clt.Close(); err != nil {
			logs.Error("redis single close err:%v", err)
		}
	}
}

func (r *RedisManager) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	if r.ClusterClt != nil {
		return r.ClusterClt.Set(ctx, key, value, expiration).Err()
	}

	if r.Clt != nil {
		return r.Clt.Set(ctx, key, value, expiration).Err()
	}

	return nil
}
