package referral

import "github.com/hanzoai/commerce/datastore"

var kind = "referral"

func (r Referral) Kind() string {
	return kind
}

func (r *Referral) Init(db *datastore.Datastore) {
	r.BaseModel.Init(db, r)
}

func (r *Referral) Defaults() {
}

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
