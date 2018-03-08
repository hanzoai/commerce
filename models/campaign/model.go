package campaign

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (c Campaign) Kind() string {
	return "campaign"
}

func (c *Campaign) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
