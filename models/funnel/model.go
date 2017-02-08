package funnel

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (f Funnel) Kind() string {
	return "funnel"
}

func (f *Funnel) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func New(db *datastore.Datastore) *Funnel {
	f := new(Funnel)
	f.Init(db)
	return f
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
