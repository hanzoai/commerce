package test

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
)

type Model struct {
	mixin.BaseModel
	Name string
}

func (m Model) Kind() string {
	return "test-model"
}

func (m Model) Init(db *datastore.Datastore) {
	m.BaseModel = mixin.BaseModel{Db: db, Entity: m}
}

func (m Model) Document() mixin.Document {
	return nil
}
