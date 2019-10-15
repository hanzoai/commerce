package paymentmethod

import (
	"hanzo.io/datastore"
)

var kind = "paymentmethod"

func (p PaymentMethod) Kind() string {
	return kind
}

func (p *PaymentMethod) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PaymentMethod) Defaults() {
}

func New(db *datastore.Datastore) *PaymentMethod {
	p := new(PaymentMethod)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
