package transaction

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	Deposit  Type = "deposit"
	Transfer      = "transfer"
	Withdraw      = "withdraw"
)

type Transaction struct {
	mixin.Model

	UserId          string         `json:"userId"`
	DestinationId   string         `json:"destinationId"`
	DestinationKind string         `json:"destinationKind"`
	Type            Type           `json:"type"`
	Currency        currency.Type  `json:"currency"`
	Amount          currency.Cents `json:"amount"`
	Test            bool           `json:"test"`

	// Short text human readable description
	Notes string `json:"notes"`

	// For searching
	Tags string `json:"tags"`

	Event string `json:"event"`

	// Source Data
	// We store Kind even though it is encoded in id for easier reference
	SourceId   string `json:"sourceId"`
	SourceKind string `json:"sourceKind"`
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
