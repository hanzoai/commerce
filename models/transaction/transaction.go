package transaction

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	Deposit  Type = "deposit"
	Withdraw      = "withdraw"
)

type Transaction struct {
	mixin.Model

	UserId   string         `json:"userId"`
	Type     Type           `json:"type"`
	Currency currency.Type  `json:"currency"`
	Amount   currency.Cents `json:"amount"`
}

func New(db *datastore.Datastore) *Transaction {
	t := new(Transaction)
	t.Model = mixin.Model{Db: db, Entity: t}
	return t
}

func (r Transaction) Kind() string {
	return "transaction"
}

func (t *Transaction) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(t, c)); err != nil {
		return err
	}

	return err
}

func (t *Transaction) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(t, c))
}

func (t *Transaction) Validator() *val.Validator {
	return nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
