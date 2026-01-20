package variant

import "github.com/hanzoai/commerce/datastore"

var kind = "variant"

func (v Variant) Kind() string {
	return kind
}

func (v *Variant) Init(db *datastore.Datastore) {
	v.Model.Init(db, v)
}

func (v *Variant) Defaults() {
	v.Options = make([]Option, 0)
}

func New(db *datastore.Datastore) *Variant {
	v := new(Variant)
	v.Init(db)
	v.Defaults()
	return v
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
