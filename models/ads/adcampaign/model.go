package adcampaign

import (
	"hanzo.io/datastore"
	. "hanzo.io/models/ads"
)

var kind = "adcampaign"

func (a AdCampaign) Kind() string {
	return kind
}

func (a *AdCampaign) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *AdCampaign) Defaults() {
	a.Status = PendingStatus
}

func New(db *datastore.Datastore) *AdCampaign {
	a := new(AdCampaign)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
