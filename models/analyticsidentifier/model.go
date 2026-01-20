package analyticsidentifier

import (
	"github.com/hanzoai/commerce/datastore"
)

var kind = "analyticsidentifier"

func (e AnalyticsIdentifier) Kind() string {
	return kind
}

func (e *AnalyticsIdentifier) Init(db *datastore.Datastore) {
	e.Model.Init(db, e)
}

func (e *AnalyticsIdentifier) Defaults() {
}

func New(db *datastore.Datastore) *AnalyticsIdentifier {
	e := new(AnalyticsIdentifier)
	e.Init(db)
	e.Defaults()
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
