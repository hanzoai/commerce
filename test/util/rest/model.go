package test

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
)

type Model struct {
	mixin.Model
	Name string
}

func (m Model) Kind() string {
	return "test-model"
}

func (m Model) Init(db *datastore.Datastore) {
	m.Model = mixin.Model{Db: db, Entity: m}
}

func (m Model) Document() mixin.Document {
	return nil
}
