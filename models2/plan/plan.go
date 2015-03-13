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
	*mixin.Model `datastore:"-"`

	Name        string
	Description string
	Price       int
	Interval    Interval
}

func New(db *datastore.Datastore) *Plan {
	c := new(Plan)
	c.Model = mixin.NewModel(db, c)
	return c
}

func (c Plan) Kind() string {
	return "plan2"
}
