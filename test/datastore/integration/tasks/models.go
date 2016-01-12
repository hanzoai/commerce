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

func (m *Model) Init(db *datastore.Datastore) {
	m.Model = mixin.Model{Db: db, Entity: m}
}

func (m Model) Document() mixin.Document {
	return nil
}

func (m *Model) Validator() *val.Validator {
	return val.New()
}

func NewModel(db *datastore.Datastore) *Model {
	return new(Model).New(db).(*Model)
}

// Model 2
type Model2 struct {
	mixin.Model
	Count int
}

func (m Model2) Kind() string {
	return "order"
}

func (m *Model2) Init(db *datastore.Datastore) {
	m.Model = mixin.Model{Db: db, Entity: m}
}

func (m Model2) Document() mixin.Document {
	return nil
}

func (m *Model2) Validator() *val.Validator {
	return val.New()
}

func NewModel2(db *datastore.Datastore) *Model2 {
	return new(Model2).New(db).(*Model2)
}
