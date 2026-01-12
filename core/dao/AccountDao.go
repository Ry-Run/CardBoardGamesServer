package dao

import (
	"context"
	"core/models/entity"
	"core/repo"
)

type AccountDao struct {
	repo *repo.Manager
}

func (a AccountDao) Save(ctx context.Context, e *entity.Account) error {
	table := a.repo.Mongo.Db.Collection("account")
	_, err := table.InsertOne(ctx, e)
	if err != nil {
		return err
	}
	return nil
}

func NewAccountDao(m *repo.Manager) *AccountDao {
	return &AccountDao{
		repo: m,
	}
}
