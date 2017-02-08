package coupon

import "hanzo.io/datastore"

var kind = "redemption"

func (r Redemption) Kind() string {
	return kind
}

func (r *Redemption) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Redemption) Defaults() {
}

func New(db *datastore.Datastore) *Redemption {
	r := new(Redemption)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
