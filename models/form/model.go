package form

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (f Form) Kind() string {
	return "form"
}

func (f *Form) Init(db *datastore.Datastore) {
	f.Model.Init(db, f)
}

func New(db *datastore.Datastore) *Form {
	f := new(Form)
	f.Init(db)
	return f
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
