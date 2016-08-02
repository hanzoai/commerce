package funnel

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (f Funnel) Kind() string {
	return "funnel"
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
	return f
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
