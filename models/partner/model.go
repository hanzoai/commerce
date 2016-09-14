package partner

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (a Partner) Kind() string {
	return "partner"
}

func (a *Partner) Init(db *datastore.Datastore) {
	a.Model.Init(db, a)
}

func New(db *datastore.Datastore) *Partner {
	r := new(Partner)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
