package bundle

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (c Bundle) Kind() string {
	return "bundle"
}

func (c *Bundle) Init(db *datastore.Datastore) {
	c.Model = mixin.Model{Db: db, Entity: c}
}

func (c *Bundle) Defaults() {
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
}

func New(db *datastore.Datastore) *Bundle {
	b := new(Bundle)
	b.Init(db)
	b.Defaults()
	return b
}
