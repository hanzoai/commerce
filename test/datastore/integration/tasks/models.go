package tasks

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
)

// Model 1
type Model struct {
	mixin.Model
	Count int
}

func (m Model) Kind() string {
	return "user"
}

func (m Model) Document() mixin.Document {
	return nil
}

func (m *Model) Validator() *val.Validator {
	return val.New()
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

func (m Model2) Document() mixin.Document {
	return nil
}

func (m *Model2) Validator() *val.Validator {
	return val.New()
}

func NewModel2(db *datastore.Datastore) *Model2 {
	m := new(Model2)
	m.Model = mixin.Model{Db: db, Entity: m}
	return m
}
