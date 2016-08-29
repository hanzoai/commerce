package fee

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (f Fee) Kind() string {
	return "fee"
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

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
