package payment

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "payment"

func (p Payment) Kind() string {
	return kind
}

func (p *Payment) Init(db *datastore.Datastore) {
	p.BaseModel.Init(db, p)
}

func (p *Payment) Defaults() {
	p.Status = Unpaid
	p.FeeIds = make([]string, 0)
	p.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Payment {
	p := new(Payment)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
