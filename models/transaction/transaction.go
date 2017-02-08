package transaction

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
)

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
	Test     bool           `json:"test"`

	// Short text human readable description
	Notes string `json:"notes"`

	// For searching
	Tags string `json:"tags"`

	// Source Data
	// We store Kind even though it is encoded in id for easier reference
	SourceId   string `json:"sourceId"`
	SourceKind string `json:"sourceKind"`
}
