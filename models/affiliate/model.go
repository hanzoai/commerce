package affiliate

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/schedule"
)

var kind = "affiliate"

func (a Affiliate) Kind() string {
	return kind
}

func (a *Affiliate) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Affiliate) Defaults() {
	a.Schedule.Period = 30
	a.Schedule.Type = schedule.DailyRolling
}

func New(db *datastore.Datastore) *Affiliate {
	a := new(Affiliate)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
