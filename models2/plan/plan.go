package plan

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
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
	return "plan2"
}
