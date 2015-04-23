package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
)

// Model 1
type Model struct {
	mixin.Model
	Count int
}

func (m Model) Kind() string {
	return "user"
}

func (m *Model) Validator() *val.Validator {
	return val.New(m)
}

func NewModel(db *datastore.Datastore) *Model {
	m := new(Model)
	m.Model = mixin.Model{Db: db, Entity: m}
	return m
}

// Model 2
type Model2 struct {
	mixin.Model
	Count int
}

func (m Model2) Kind() string {
	return "order"
}

func (m *Model2) Validator() *val.Validator {
	return val.New(m)
}

func NewModel2(db *datastore.Datastore) *Model2 {
	m := new(Model2)
	m.Model = mixin.Model{Db: db, Entity: m}
	return m
}
