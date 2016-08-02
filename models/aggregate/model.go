package aggregate

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (a Aggregate) Kind() string {
	return "aggregate"
}

func (a *Aggregate) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Aggregate) Defaults() {
	a.VectorValue = make([]int64, 0)
}

func New(db *datastore.Datastore) *Aggregate {
	a := new(Aggregate)
	a.Init(db)
	return a
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
