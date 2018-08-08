package invoice

import (

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Status string

const (
	Cancelled  Status = "cancelled"
	Credit            = "credit"
	Disputed          = "disputed"
	Failed            = "failed"
	Fraudulent        = "fraudulent"
	Paid              = "paid"
	Refunded          = "refunded"
	Unpaid            = "unpaid"
)

type Type string

const (
	Stripe Type = "stripe"
	Affirm      = "affirm"
	PayPal      = "paypal"
)

type Invoice struct {
	mixin.Model

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	Type Type `json:"type"`

	// Order this is associated with
	OrderId string `json:"orderId,omitempty"`

	Currency currency.Type `json:"currency"`

	CampaignId string `json:"campaignId"`

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded"`
	Fee            currency.Cents `json:"fee"`

	AmountTransferred   currency.Cents `json:"-"`
	CurrencyTransferred currency.Type  `json:"-"`

	Description string `json:"description"`
	Status      Status `json:"status"`

	// Client's browser, associated info
	Client client.Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`
}

func (p Invoice) Kind() string {
	return "payment"
}

func (p *Invoice) Init() {
	p.Status = Unpaid
	p.Metadata = make(Map)
}


func (p *Invoice) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized
	p.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(p, ps)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Invoice) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	if err != nil {
		return nil, err
	}

	// Save properties
	return datastore.SaveStruct(p)
}

func (p *Invoice) Validator() *val.Validator {
	return val.New()
}

func New(db *datastore.Datastore) *Invoice {
	p := new(Invoice)
	p.Init()
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

