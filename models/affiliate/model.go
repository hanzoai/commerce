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
	a.FlatFee = 30
	a.PlatformFee = .30
}

func New(db *datastore.Datastore) *Affiliate {
	r := new(Affiliate)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
