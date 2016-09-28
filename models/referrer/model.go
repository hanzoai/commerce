package referrer

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (r Referrer) Kind() string {
	return "referrer"
}

func (r *Referrer) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
