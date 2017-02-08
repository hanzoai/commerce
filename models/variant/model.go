package variant

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (v Variant) Kind() string {
	return "variant"
}

func (v *Variant) Init(db *datastore.Datastore) {
	v.Model.Init(db, v)
}

func New(db *datastore.Datastore) *Variant {
	v := new(Variant)
	v.Init(db)
	return v
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
