package transaction

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[Transaction]("transaction") }

type Type string

const (
	Hold        Type = "hold"
	HoldRemoved Type = "hold-removed"
	Transfer    Type = "transfer"
	Deposit     Type = "deposit"
	Withdraw    Type = "withdraw"
)

type Transaction struct {
	mixin.Model[Transaction]

	DestinationId   string `json:"destinationId"`
	DestinationKind string `json:"destinationKind"`

	Currency currency.Type  `json:"currency"`
	Amount   currency.Cents `json:"amount"`
	Type     Type           `json:"type"`

	Test bool `json:"test"`

	// Short text human readable description
	Notes string `json:"notes,omitempty"`

	// For searching
	Tags string `json:"tags,omitempty"`

	Event string `json:"event,omitempty"`

	// Source Data
	// We store Kind even though it is encoded in id for easier reference
	SourceId   string `json:"sourceId,omitempty"`
	SourceKind string `json:"sourceKind,omitempty"`

	// ExpiresAt marks when a deposit credit expires. Zero value means no expiry.
	// Expired deposits are excluded from balance calculations.
	ExpiresAt time.Time `json:"expiresAt,omitempty"`

	// Deprecated
	UserId string `json:"-"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (t *Transaction) Load(ps []datastore.Property) (err error) {
	// Load supported properties
	if err = datastore.LoadStruct(t, ps); err != nil {
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

func (t *Transaction) Save() (ps []datastore.Property, err error) {
	// Serialize unsupported properties
	t.Metadata_ = string(json.EncodeBytes(&t.Metadata))

	// Save properties
	return datastore.SaveStruct(t)
}

func (t *Transaction) Validator() *val.Validator {
	return nil
}

func New(db *datastore.Datastore) *Transaction {
	t := new(Transaction)
	t.Init(db)
	t.Parent = db.NewKey("synckey", "", 1, nil)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("transaction")
}
