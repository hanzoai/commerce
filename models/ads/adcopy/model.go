package adcopy

import "hanzo.io/datastore"

var kind = "adccopy"

func (a AdCopy) Kind() string {
	return kind
}

func (a *AdCopy) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *AdCopy) Defaults() {
}

func New(db *datastore.Datastore) *AdCopy {
	a := new(AdCopy)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
