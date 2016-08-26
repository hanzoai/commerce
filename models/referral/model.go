package referral

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (r Referral) Kind() string {
	return "referral"
}

func (r *Referral) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
