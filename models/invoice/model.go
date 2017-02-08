package invoice

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (i Invoice) Kind() string {
	return "payment"
}

func (i *Invoice) Init(db *datastore.Datastore) {
	i.Model.Init(db, i)
}

func New(db *datastore.Datastore) *Invoice {
	i := new(Invoice)
	i.Init(db)
	return i
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
