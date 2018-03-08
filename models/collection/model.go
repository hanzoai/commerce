package collection

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (c Collection) Kind() string {
	return "collection"
}

func (c *Collection) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
