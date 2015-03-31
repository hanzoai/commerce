package payment

import (
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/util/gob"
	"crowdstart.io/util/log"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Status string

const (
	Disputed   Status = "disputed"
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

type Client struct {
	Ip        string `json:"ip,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Language  string `json:"language,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

type AffirmAccount struct {
	CheckoutToken string `json:"checkoutToken,omitempty"`
}

type PayPalAccount struct {
	Email       string `json:"email,omitempty"`
	SellerEmail string `json:"sellerEmail,omitempty"`
	RedirectUrl string `json:"redirectUrl,omitempty"`
	Ipn         string `json:"ipn,omitempty"`

	// Preapproval expiration date (Unix timestamp in milliseconds).
	Ending int `json:"ending,omitempty"`

	// Preapproval expiration date (ISO 8601 timestamp).
	EndingDate string `json:"endingDate,omitempty"`
}

type StripeAccount struct {
	// Very important to never store these!
	Number string `json:"number" datastore:"-"`
	CVC    string `json:"cvc" datastore:"-"`

	CardId     string `json:"cardId,omitempty"`
	ChargeId   string `json:"chargeId,omitempty"`
	CustomerId string `json:"customerId,omitempty"`

	Fingerprint string `json:"fingerprint,omitempty"`
	Funding     string `json:"funding,omitempty"`
	Brand       string `json:"brand,omitempty"`
	LastFour    string `json:"lastFour,omitempty"`
	Month       int    `json:"month,string,omitempty"`
	Year        int    `json:"year,string,omitempty"`
	Country     string `json:"country,omitempty"`

	CVCCheck string `json:"cvcCheck,omitempty"`
}

func (sa StripeAccount) CardMatches(acct Account) bool {
	log.Debug("Checking for match")
	log.Debug("Old card: %v", sa)
	log.Debug("New card: %v", acct)

	if sa.Month != acct.Month {
		return false
	}
	if sa.Year != acct.Year {
		return false
	}
	if len(acct.Number) > 4 && sa.LastFour != acct.Number[len(acct.Number)-4:] {
		return false
	}
	return true
}

// Sort of a union type of all possible payment accounts, used everywhere for convenience
type Account struct {
	AffirmAccount
	PayPalAccount
	StripeAccount
}

type Payment struct {
	mixin.Model

	Type Type `json:type"`

	// Order this is associated with
	OrderId string `json:"orderId,omitempty"`

	// Payment source information
	Account Account `json:"account"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	Currency currency.Type `json:"currency"`

	CampaignId string `json:"campaignId"`

	// Id for Stripe/Affirm
	ChargeId string `json:"chargeId,omitempty"`

	// Stripe only.
	BalanceTransaction string `json:"balanceTransaction,omitempty"`

	// PayPal only.
	PayKey         string `json:"payKey,omitempty"`
	PreapprovalKey string `json:"preapprovalKey,omitempty"`

	// Affirm only.
	CaptureId     string `json:"captureId,omitempty"`
	TransactionId string `json:"transactionId,omitempty"`

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded"`

	Status Status `json:"status"`

	// Client's browser, associated info
	Client Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`

	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ []byte   `json:"-"`
}

func (p Payment) Kind() string {
	return "payment"
}

func (p *Payment) Init() {

}

func (p *Payment) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	p.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(p, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = gob.Decode(p.Metadata_, &p.Metadata)
	}

	return err
}

func (p *Payment) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	p.Metadata_, err = gob.Encode(&p.Metadata)

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(p, c))
}

func (p *Payment) Validator() *val.Validator {
	return val.New(p)
}

func New(db *datastore.Datastore) *Payment {
	p := new(Payment)
	p.Init()
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
