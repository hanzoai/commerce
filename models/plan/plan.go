package plan

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
)

type Interval string

const (
	Year  Interval = "year"
	Month          = "month"
)

type Plan struct {
	mixin.Model

	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Interval    Interval `json:"interval"`
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Validator() *val.Validator {
	return val.New(p)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
