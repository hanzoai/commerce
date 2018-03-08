package fee

import "hanzo.io/datastore"

var kind = "fee"

func (f Fee) Kind() string {
	return kind
}

func (f *Fee) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func (f *Fee) Defaults() {
	f.Status = Pending
}

func New(db *datastore.Datastore) *Fee {
	f := new(Fee)
	f.Init(db)
	f.Defaults()
	return f
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
