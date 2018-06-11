package invoice

import (
	"strconv"

	"github.com/stripe/stripe-go"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
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

	// Invoice source information
	Account Account `json:"account"`

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

func (p Invoice) ToCard() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = p.Buyer.Name()
	card.Number = p.Account.Number
	card.CVC = p.Account.CVC
	card.Month = strconv.Itoa(p.Account.Month)
	card.Year = strconv.Itoa(p.Account.Year)
	card.Address1 = p.Buyer.Address.Line1
	card.Address2 = p.Buyer.Address.Line2
	card.City = p.Buyer.Address.City
	card.State = p.Buyer.Address.State
	card.Zip = p.Buyer.Address.PostalCode
	card.Country = p.Buyer.Address.Country
	return &card
}

func (p *Invoice) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	p.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(p, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Invoice) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(p, c))
}

func (p *Invoice) Validator() *val.Validator {
	return val.New(p)
}

func New(db *datastore.Datastore) *Invoice {
	p := new(Invoice)
	p.Init()
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
