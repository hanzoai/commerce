package banktransferinstruction

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"

	. "github.com/hanzoai/commerce/types"
)

var kind = "bank-transfer-instruction"

// BankTransferInstruction represents bank account details issued to a customer
// for receiving inbound wire/ACH/SEPA transfers. Each instruction carries a
// unique payment reference that is used to reconcile incoming funds.
type BankTransferInstruction struct {
	mixin.Model

	CustomerId    string        `json:"customerId"`
	Currency      currency.Type `json:"currency"`
	Type          string        `json:"type"`                    // "ach" | "wire" | "sepa"
	Reference     string        `json:"reference"`               // unique payment reference
	BankName      string        `json:"bankName"`
	AccountHolder string        `json:"accountHolder,omitempty"`
	AccountNumber string        `json:"accountNumber"`           // last 4 only (masked)
	RoutingNumber string        `json:"routingNumber,omitempty"`
	IBAN          string        `json:"iban,omitempty"`
	BIC           string        `json:"bic,omitempty"`
	Status        string        `json:"status"` // "active" | "expired"

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (i BankTransferInstruction) Kind() string {
	return kind
}

func (i *BankTransferInstruction) Init(db *datastore.Datastore) {
	i.Model.Init(db, i)
}

func (i *BankTransferInstruction) Defaults() {
	i.Parent = i.Db.NewKey("synckey", "", 1, nil)
	if i.Status == "" {
		i.Status = "active"
	}
	if i.Currency == "" {
		i.Currency = "usd"
	}
}

func (i *BankTransferInstruction) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(i, ps); err != nil {
		return err
	}

	if len(i.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(i.Metadata_), &i.Metadata)
	}

	return err
}

func (i *BankTransferInstruction) Save() (ps []datastore.Property, err error) {
	i.Metadata_ = string(json.EncodeBytes(&i.Metadata))
	return datastore.SaveStruct(i)
}

func (i *BankTransferInstruction) Validator() *val.Validator {
	return nil
}

func New(db *datastore.Datastore) *BankTransferInstruction {
	i := new(BankTransferInstruction)
	i.Init(db)
	i.Defaults()
	return i
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
