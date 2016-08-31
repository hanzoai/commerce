package bundle

import (
	"crowdstart.com/datastore"

	. "crowdstart.com/models"
)

func (b Bundle) Kind() string {
	return "bundle"
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
	return b
}
