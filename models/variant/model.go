package variant

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (v Variant) Kind() string {
	return "variant"
}

func (v *Variant) Init(db *datastore.Datastore) {
	v.Model = mixin.Model{Db: db, Entity: v}
}

func (v *Variant) Defaults() {
	v.Options = make([]Option, 0)
}

func New(db *datastore.Datastore) *Variant {
	return new(Variant).New(db).(*Variant)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
