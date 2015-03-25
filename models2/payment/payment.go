package payment

import (
	. "crowdstart.io/models2"
)

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

type Account struct {
	// Affirm
	CheckoutToken string `json:"checkoutToken,omitempty"`

	// PayPal
	Email       string `json:"email,omitempty"`
	SellerEmail string `json:"sellerEmail,omitempty"`
	RedirectUrl string `json:"redirectUrl,omitempty"`
	Ipn         string `json:"ipn,omitempty"`

	// Preapproval expiration date (Unix timestamp in milliseconds).
	Ending int `json:"ending,omitempty"`

	// Preapproval expiration date (ISO 8601 timestamp).
	EndingDate string `json:"endingDate,omitempty"`

	// Stripe
	Fingerprint string `json:"fingerprint,omitempty"`
	CustomerId  string `json:"customerId,omitempty"`
	ChargeId    string `json:"chargeId,omitempty"`
	CardId      string `json:"cardId,omitempty"`
	Brand       string `json:"brand,omitempty"`
	Type        string `json:"type,omitempty"`
	LastFour    string `json:"lastFour,omitempty"`
	Expiration  struct {
		Month int `json:"month,omitempty"`
		Year  int `json:"year,omitempty"`
	} `json:"expiration,omitempty"`
	Country  string `json:"country,omitempty"`
	CVCCheck string `json:"cvcCheck,omitempty"`
}

type Payment struct {
	Type Type `json:type"`

	// Optionally associated with a user
	UserId string `json:"userId,omitempty"`

	// Payment source information
	Account Account `json:"account"`

	// Immutable buyer data from time of payment
	Buyer Buyer `json:"buyer"`

	Currency CurrencyType `json:"currency"`

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

	Amount         Cents `json:"amount"`
	AmountRefunded Cents `json:"amountRefunded"`

	Status Status `json:"status"`

	// Client's browser, associated info
	Client Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Whether this was a transaction in production or a testing sandbox
	Live bool `json:"live"`
}
