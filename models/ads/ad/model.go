package ad

import "hanzo.io/datastore"

var kind = "ad"

func (a Ad) Kind() string {
	return kind
}

func (a *Ad) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Ad) Defaults() {
}

func New(db *datastore.Datastore) *Ad {
	a := new(Ad)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
