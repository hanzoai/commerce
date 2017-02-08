package bundle

import (
	"hanzo.io/datastore"

	. "hanzo.io/models"
)

var kind = "bundle"

func (b Bundle) Kind() string {
	return kind
}

func (b *Bundle) Init(db *datastore.Datastore) {
	b.Model.Init(db, b)
}

func (b *Bundle) Defaults() {
	b.Media = make([]Media, 0)
	b.ProductIds = make([]string, 0)
	b.VariantIds = make([]string, 0)
}

func New(db *datastore.Datastore) *Bundle {
	b := new(Bundle)
	b.Init(db)
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
