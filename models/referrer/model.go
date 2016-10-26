package referrer

import "crowdstart.com/datastore"

var kind = "referrer"

func (r Referrer) Kind() string {
	return kind
}

func (r *Referrer) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Referrer) Defaults() {
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
