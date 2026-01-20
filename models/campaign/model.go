package campaign

import "github.com/hanzoai/commerce/datastore"

var kind = "campaign"

func (c Campaign) Kind() string {
	return kind
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
	c.Defaults()
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
