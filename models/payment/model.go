package payment

import (
	"crowdstart.com/datastore"

	. "crowdstart.com/models"
)

var kind = "payment"

func (p Payment) Kind() string {
	return kind
}

func (p *Payment) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Payment) Defaults() {
	p.Status = Unpaid
	p.FeeIds = make([]string, 0)
	p.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Payment {
	p := new(Payment)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
