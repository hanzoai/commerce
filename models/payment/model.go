package payment

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (p Payment) Kind() string {
	return "payment"
}

func (p *Payment) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func New(db *datastore.Datastore) *Payment {
	p := new(Payment)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
