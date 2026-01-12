package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 一般来说一个账户可以对应多个角色
// 这里简单做成一个角色即可
type Account struct {
	Id           primitive.ObjectID `bson:"_id,omitempty"`
	Uid          string             `bson:"uid"`
	Account      string             `bson:"account" `
	Password     string             `bson:"password"`
	PhoneAccount string             `bson:"phoneAccount"`
	WxAccount    string             `bson:"wxAccount"`
	CreateTime   time.Time          `bson:"createTime"`
}
