package discount

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (d Discount) Kind() string {
	return "discount"
}

func (d *Discount) Init(db *datastore.Datastore) {
	d.Model.Init(db, d)
}

func (d *Discount) Defaults() {
	d.Enabled = true
	d.Rules = make([]Rule, 0)
}

func New(db *datastore.Datastore) *Discount {
	d := new(Discount)
	d.Init(db)
	return d
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
