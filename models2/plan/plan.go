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

	Name        string
	Description string
	Price       int
	Interval    Interval
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p Plan) Kind() string {
	return "plan2"
}
