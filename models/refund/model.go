package refund

import "github.com/hanzoai/commerce/datastore"

var kind = "refund"

func (r Refund) Kind() string {
	return kind
}

func (r *Refund) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *Refund) Defaults() {
	r.Parent = r.Db.NewKey("synckey", "", 1, nil)
	if r.Status == "" {
		r.Status = Pending
	}
}

func New(db *datastore.Datastore) *Refund {
	r := new(Refund)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
