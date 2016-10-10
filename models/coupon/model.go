package coupon

import "crowdstart.com/datastore"

var kind = "coupon"

func (c Coupon) Kind() string {
	return kind
}

func (c *Coupon) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *Coupon) Defaults() {
	c.Enabled = true
	// c.Buyers = make([]string, 0)
}

func New(db *datastore.Datastore) *Coupon {
	c := new(Coupon)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
