package analytics

import (
	"hanzo.io/datastore"

	. "hanzo.io/models"
)

var kind = "event"

func (e AnalyticsEvent) Kind() string {
	return kind
}

func (e *AnalyticsEvent) Init(db *datastore.Datastore) {
	e.Model.Init(db, e)
}

func (e *AnalyticsEvent) Defaults() {
	e.Data = make(Map)
}

func New(db *datastore.Datastore) *AnalyticsEvent {
	e := new(AnalyticsEvent)
	e.Init(db)
	e.Defaults()
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
