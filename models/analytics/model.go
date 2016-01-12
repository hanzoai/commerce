package analytics

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (e AnalyticsEvent) Kind() string {
	return "event"
}

func (e *AnalyticsEvent) Init(db *datastore.Datastore) {
	e.Model = mixin.Model{Db: db, Entity: e}
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

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
