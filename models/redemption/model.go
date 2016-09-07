package coupon

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (r Redemption) Kind() string {
	return "redemption"
}

func (r *Redemption) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Redemption) Defaults() {
}

func New(db *datastore.Datastore) *Redemption {
	r := new(Redemption)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
