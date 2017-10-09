package adconfig

import (
	"hanzo.io/datastore"
)

var kind = "adconfig"

func (a AdConfig) Kind() string {
	return kind
}

func (a *AdConfig) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *AdConfig) Defaults() {
}

func New(db *datastore.Datastore) *AdConfig {
	a := new(AdConfig)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
