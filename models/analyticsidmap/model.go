package analyticsidmap

import (
	"hanzo.io/datastore"
)

var kind = "analyticsidmap"

func (e AnalyticsIdMap) Kind() string {
	return kind
}

func (e *AnalyticsIdMap) Init(db *datastore.Datastore) {
	e.Model.Init(db, e)
}

func (e *AnalyticsIdMap) Defaults() {
}

func New(db *datastore.Datastore) *AnalyticsIdMap {
	e := new(AnalyticsIdMap)
	e.Init(db)
	e.Defaults()
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
