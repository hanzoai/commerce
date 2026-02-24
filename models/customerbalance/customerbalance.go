package customerbalance

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/val"
)

var kind = "customer-balance"

// CustomerBalance tracks a customer's stored-value balance per currency.
// Positive balance = credit available to settle invoices.
type CustomerBalance struct {
	mixin.BaseModel

	CustomerId string        `json:"customerId"`
	Currency   currency.Type `json:"currency"`
	Balance    int64         `json:"balance"` // cents, positive = credit
}

func (cb CustomerBalance) Kind() string {
	return kind
}

func (cb *CustomerBalance) Init(db *datastore.Datastore) {
	cb.BaseModel.Init(db, cb)
}

func (cb *CustomerBalance) Defaults() {
	cb.Parent = cb.Db.NewKey("synckey", "", 1, nil)
	if cb.Currency == "" {
		cb.Currency = "usd"
	}
}

func (cb *CustomerBalance) Load(ps []datastore.Property) (err error) {
	return datastore.LoadStruct(cb, ps)
}

func (cb *CustomerBalance) Save() (ps []datastore.Property, err error) {
	return datastore.SaveStruct(cb)
}

func (cb *CustomerBalance) Validator() *val.Validator {
	return nil
}

func New(db *datastore.Datastore) *CustomerBalance {
	cb := new(CustomerBalance)
	cb.Init(db)
	cb.Defaults()
	return cb
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
