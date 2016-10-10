package referral

import "crowdstart.com/datastore"

var kind = "referral"

func (r Referral) Kind() string {
	return kind
}

func (r *Referral) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
