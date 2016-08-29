package payment

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (p Payment) Kind() string {
	return "payment"
}

func (p *Payment) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Payment) Defaults() {
	p.Status = Unpaid
	p.Fees = make([]*fee.Fee, 0)
	p.FeeIds = make([]string, 0)
	p.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Payment {
	p := new(Payment)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
