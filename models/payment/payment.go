package payment

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"

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

type AffirmAccount struct {
	CaptureId     string `json:"captureId,omitempty"`
	TransactionId string `json:"transactionId,omitempty"`
	CheckoutToken string `json:"checkoutToken,omitempty"`
}

type PayPalAccount struct {
	Email       string `json:"email,omitempty"`
	SellerEmail string `json:"sellerEmail,omitempty"`
	RedirectUrl string `json:"redirectUrl,omitempty"`
	Ipn         string `json:"ipn,omitempty"`

	PayKey         string `json:"payKey,omitempty"`
	PreapprovalKey string `json:"preapprovalKey,omitempty"`

	// Preapproval expiration date (Unix timestamp in milliseconds).
	Ending int `json:"ending,omitempty"`

	// Preapproval expiration date (ISO 8601 timestamp).
	EndingDate string `json:"endingDate,omitempty"`
}

type StripeAccount struct {
	// Very important to never store these!
	Number string `json:"number,omitempty" datastore:"-"`
	CVC    string `json:"cvc,omitempty" datastore:"-"`

	BalanceTransactionId string `json:"balanceTransactionId,omitempty"`
	CardId               string `json:"cardId,omitempty"`
	ChargeId             string `json:"chargeId,omitempty"`
	CustomerId           string `json:"customerId,omitempty"`

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
	if len(sa.LastFour) == 4 && sa.LastFour != acct.LastFour {
		return false
	}
	return true
}

// Sort of a union type of all possible payment accounts, used everywhere for convenience
type Account struct {
	AffirmAccount
	PayPalAccount
	StripeAccount

	Error string `json:"error,omitempty"`
}

type Payment struct {
	mixin.Model

	Type Type `json:"type"`

	// Order this is associated with
	OrderId string `json:"orderId,omitempty"`

	// Payment source information
	Account Account `json:"account"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

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

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`
}

func (p *Payment) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	p.Defaults()

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

func (p *Payment) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(p, c))
}
