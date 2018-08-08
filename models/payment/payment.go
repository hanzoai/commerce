package payment

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/fee"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"

	. "hanzo.io/types"
)

type Status string

const (
	Cancelled  Status = "cancelled"
	Credit     Status = "credit"
	Disputed   Status = "disputed"
	Failed     Status = "failed"
	Fraudulent Status = "fraudulent"
	Paid       Status = "paid"
	Refunded   Status = "refunded"
	Unpaid     Status = "unpaid"
)

type Payment struct {
	mixin.Model

	// Deprecated
	Type accounts.Type `json:"type"`

	// Order this payment is associated with
	OrderId string `json:"orderId,omitempty"`

	// User this payment is associated with
	UserId string `json:"userId,omitempty"`

	// Payment source information
	Account accounts.Account `json:"account"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	Currency currency.Type `json:"currency"`

	CampaignId string `json:"campaignId,omitempty"`

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded"`
	Fee            currency.Cents `json:"fee"`
	FeeIds         []string       `json:"fees" datastore:",noindex"`

	AmountTransferred   currency.Cents `json:"-"`
	CurrencyTransferred currency.Type  `json:"-"`

	Description string `json:"description,omitempty"`
	Status      Status `json:"status"`

	// Client's browser, associated info
	Client client.Client `json:"client,omitempty"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (p *Payment) GetFees() ([]*fee.Fee, error) {
	fees := make([]*fee.Fee, 0)
	if err := fee.Query(p.Db).Filter("PaymentId=", p.Id()).GetModels(&fees); err != nil {
		return nil, err
	}
	return fees, nil
}

func (p *Payment) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized
	p.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Payment) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	// Save properties
	return datastore.SaveStruct(p)
}
