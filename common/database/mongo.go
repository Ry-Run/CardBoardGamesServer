package database

import (
	"common/config"
	"common/logs"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoManager struct {
	Clt *mongo.Client
	Db  *mongo.Database
}

func NewMongo() *MongoManager {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Conf.Database.MongoConf.Timeout)*time.Second)
	defer cancel()
	cltOptions := options.Client().ApplyURI(config.Conf.Database.MongoConf.Url)
	cltOptions.SetAuth(options.Credential{
		Username: config.Conf.Database.MongoConf.UserName,
		Password: config.Conf.Database.MongoConf.Password,
	})
	cltOptions.SetMinPoolSize(uint64(config.Conf.Database.MongoConf.MinPoolSize))
	cltOptions.SetMaxPoolSize(uint64(config.Conf.Database.MongoConf.MaxPoolSize))
	client, err := mongo.Connect(cltOptions)
	if err != nil {
		logs.Fatal("mongo connect err:%v", err)
		return nil
	}
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		logs.Fatal("mongo ping err:%v", err)
		return nil
	}
	database := client.Database(config.Conf.Database.MongoConf.Db)
	return &MongoManager{
		Clt: client,
		Db:  database,
	}
}

func (m *MongoManager) Close() {
	err := m.Clt.Disconnect(context.TODO())
	if err != nil {
		logs.Fatal("mongo Disconnect err:%v", err)
		return
	}
}
