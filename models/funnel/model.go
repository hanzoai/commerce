package funnel

import "hanzo.io/datastore"

var kind = "funnel"

func (f Funnel) Kind() string {
	return kind
}

func (f *Funnel) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func (f *Funnel) Defaults() {
	f.Events = make([][]string, 0)
}

func New(db *datastore.Datastore) *Funnel {
	f := new(Funnel)
	f.Init(db)
	f.Defaults()
	return f
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
