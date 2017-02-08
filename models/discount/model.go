package discount

import "hanzo.io/datastore"

var kind = "discount"

func (d Discount) Kind() string {
	return kind
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
	d.Defaults()
	return d
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
