package campaign

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (c Campaign) Kind() string {
	return "campaign"
}

func (c *Campaign) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *Campaign) Defaults() {
	c.Links = make([]string, 0)
	c.ProductIds = make([]string, 0)
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
