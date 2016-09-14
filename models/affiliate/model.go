package affiliate

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (a Affiliate) Kind() string {
	return "affiliate"
}

func (a *Affiliate) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func (a *Affiliate) Defaults() {
	a.Schedule.Period = 30
	a.Schedule.Rolling = false
}

func New(db *datastore.Datastore) *Affiliate {
	a := new(Affiliate)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
