package balancetransaction

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"

	. "github.com/hanzoai/commerce/types"
)

var kind = "balance-transaction"

// BalanceTransaction records a single change to a customer's balance.
// Positive amount = credit (adds to balance), negative = debit.
type BalanceTransaction struct {
	mixin.Model

	CustomerId string        `json:"customerId"`
	Amount     int64         `json:"amount"` // positive = credit, negative = debit
	Currency   currency.Type `json:"currency"`

	// "adjustment" | "credit_note" | "invoice_payment" | "deposit" | "bank_transfer" | "refund"
	Type string `json:"type"`

	Description   string `json:"description,omitempty"`
	InvoiceId     string `json:"invoiceId,omitempty"`
	CreditNoteId  string `json:"creditNoteId,omitempty"`
	SourceRef     string `json:"sourceRef,omitempty"` // external reference
	EndingBalance int64  `json:"endingBalance"`       // balance after this transaction

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (bt BalanceTransaction) Kind() string {
	return kind
}

func (bt *BalanceTransaction) Init(db *datastore.Datastore) {
	bt.Model.Init(db, bt)
}

func (bt *BalanceTransaction) Defaults() {
	bt.Parent = bt.Db.NewKey("synckey", "", 1, nil)
	if bt.Currency == "" {
		bt.Currency = "usd"
	}
}

func (bt *BalanceTransaction) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(bt, ps); err != nil {
		return err
	}

	if len(bt.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(bt.Metadata_), &bt.Metadata)
	}

	return err
}

func (bt *BalanceTransaction) Save() (ps []datastore.Property, err error) {
	bt.Metadata_ = string(json.EncodeBytes(&bt.Metadata))
	return datastore.SaveStruct(bt)
}

func (bt *BalanceTransaction) Validator() *val.Validator {
	return nil
}

func New(db *datastore.Datastore) *BalanceTransaction {
	bt := new(BalanceTransaction)
	bt.Init(db)
	bt.Defaults()
	return bt
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
