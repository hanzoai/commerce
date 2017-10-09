package adheadline

import "hanzo.io/datastore"

var kind = "adheadline"

func (a AdHeadline) Kind() string {
	return kind
}

func (a *AdHeadline) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *AdHeadline) Defaults() {
}

func New(db *datastore.Datastore) *AdHeadline {
	a := new(AdHeadline)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
