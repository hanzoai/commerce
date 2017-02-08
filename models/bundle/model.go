package bundle

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (c Bundle) Kind() string {
	return "bundle"
}

func (c *Bundle) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func New(db *datastore.Datastore) *Bundle {
	b := new(Bundle)
	b.Init(db)
	return b
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
