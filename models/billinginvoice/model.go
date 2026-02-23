package billinginvoice

import "github.com/hanzoai/commerce/datastore"

var kind = "billing-invoice"

func (inv BillingInvoice) Kind() string {
	return kind
}

func (inv *BillingInvoice) Init(db *datastore.Datastore) {
	inv.Model.Init(db, inv)
}

func (inv *BillingInvoice) Defaults() {
	inv.Parent = inv.Db.NewKey("synckey", "", 1, nil)
	if inv.Status == "" {
		inv.Status = Draft
	}
	if inv.Currency == "" {
		inv.Currency = "usd"
	}
}

func New(db *datastore.Datastore) *BillingInvoice {
	inv := new(BillingInvoice)
	inv.Init(db)
	inv.Defaults()
	return inv
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
