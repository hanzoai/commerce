package collection

import (
	"hanzo.io/datastore"

	. "hanzo.io/models"
)

var kind = "collection"

func (c Collection) Kind() string {
	return kind
}

func (c *Collection) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *Collection) Defaults() {
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
	c.History = make([]Event, 0)
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Init(db)
	c.Defaults()
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
