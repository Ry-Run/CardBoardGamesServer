package dao

import (
	"context"
	"core/models/entity"
	"core/repo"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserDao struct {
	repo *repo.Manager
}

func (d *UserDao) FindUserByUid(ctx context.Context, uid string) (*entity.User, error) {
	table := d.repo.Mongo.Db.Collection("user")
	res := table.FindOne(ctx, bson.D{
		{"uid", uid},
	})
	user := new(entity.User)
	err := res.Decode(user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (d *UserDao) Save(ctx context.Context, user *entity.User) error {
	table := d.repo.Mongo.Db.Collection("user")
	_, err := table.InsertOne(ctx, user)
	return err
}

func (d *UserDao) UpdateUserAddressByUid(ctx context.Context, user *entity.User) error {
	table := d.repo.Mongo.Db.Collection("user")
	_, err := table.UpdateOne(ctx, bson.M{
		"uid": user.Uid,
	}, bson.M{
		"$set": bson.M{
			"address":  user.Address,
			"location": user.Location,
		},
	})
	return err
}

func (a AccountDao) SaveUser(ctx context.Context, e *entity.User) error {
	table := a.repo.Mongo.Db.Collection("account")
	_, err := table.InsertOne(ctx, e)
	if err != nil {
		return err
	}
	return nil
}

func NewUserDao(m *repo.Manager) *UserDao {
	return &UserDao{
		repo: m,
	}
}
