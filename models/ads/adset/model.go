package adset

import (
	"hanzo.io/datastore"

	. "hanzo.io/models/ads"
)

var kind = "adset"

func (a AdSet) Kind() string {
	return kind
}

func (a *AdSet) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *AdSet) Defaults() {
	a.Status = PendingStatus
}

func New(db *datastore.Datastore) *AdSet {
	a := new(AdSet)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
