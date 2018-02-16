package transaction

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	Hold        Type = "hold"
	HoldRemoved Type = "hold-removed"
	Transfer    Type = "transfer"
	Deposit     Type = "deposit"
	Withdraw    Type = "withdraw"
)

type Transaction struct {
	mixin.Model

	DestinationId   string `json:"destinationId"`
	DestinationKind string `json:"destinationKind"`

	Currency currency.Type  `json:"currency"`
	Amount   currency.Cents `json:"amount"`
	Type     Type           `json:"type"`

	Test bool `json:"test,omitempty"`

	// Short text human readable description
	Notes string `json:"notes,omitempty"`

	// For searching
	Tags string `json:"tags,omitempty"`

	Event string `json:"event,omitempty"`

	// Source Data
	// We store Kind even though it is encoded in id for easier reference
	SourceId   string `json:"sourceId,omitempty"`
	SourceKind string `json:"sourceKind,omitempty"`

	// Deprecated
	UserId string `json:"-"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (t *Transaction) Load(c <-chan aeds.Property) error {
	var err error

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(t, c)); err != nil {
		return err
	}

	if t.UserId != "" {
		t.DestinationId = t.UserId
		t.DestinationKind = "user"
		t.UserId = ""
	}

	if len(t.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(t.Metadata_), &t.Metadata)
	}

	return err
}

func (t *Transaction) Save(c chan<- aeds.Property) error {
	// Serialize unsupported properties
	t.Metadata_ = string(json.EncodeBytes(&t.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(t, c))
}

func (t *Transaction) Validator() *val.Validator {
	return nil
}
