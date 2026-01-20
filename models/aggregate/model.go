package aggregate

import "github.com/hanzoai/commerce/datastore"

var kind = "aggregate"

func (a Aggregate) Kind() string {
	return kind
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
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
