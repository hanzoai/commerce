package referralprogram

import (
	"crowdstart.com/datastore"
)

var kind = "referralprogram"

func (r ReferralProgram) Kind() string {
	return kind
}

func (r *ReferralProgram) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *ReferralProgram) Defaults() {
}

func New(db *datastore.Datastore) *ReferralProgram {
	r := new(ReferralProgram)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
