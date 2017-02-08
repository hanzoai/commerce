package analytics

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (e AnalyticsEvent) Kind() string {
	return "event"
}

func (e *AnalyticsEvent) Init(db *datastore.Datastore) {
	e.Model.Init(db, e)
}

func New(db *datastore.Datastore) *AnalyticsEvent {
	e := new(AnalyticsEvent)
	e.Init(db)
	return e
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
