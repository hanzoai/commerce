package models

import "time"

type PaymentStatus string

const (
	PaymentDisputed   PaymentStatus = "disputed"
	PaymentFailed                   = "failed"
	PaymentFraudulent               = "fraudulent"
	PaymentPaid                     = "paid"
	PaymentRefunded                 = "refunded"
	PaymentUnpaid                   = "unpaid"
)

type PaymentGateway string

const (
	Stripe PaymentGateway = "stripe"
	Affirm                = "affirm"
	PayPal                = "paypal"
)

type Client struct {
	Ip        string
	UserAgent string
	Language  string
	Referer   string
}

type PaymentAccount struct {
	Name string
	Type PaymentGateway

	Country string

	Affirm struct {
		CheckoutToken string
	}

	Stripe struct {
		CardId     string
		CardType   string
		Last4      string
		Expiration struct {
			Month int
			Year  int
		}
	}

	Paypal struct {
		Email       string
		SellerEmail string
		RedirectUrl string
		Ipn         string

		// Preapproval expiration date (Unix timestamp in milliseconds).
		Ending int

		// Preapproval expiration date (ISO 8601 timestamp).
		EndingDate string
	}
}

type Payment struct {
	CampaignId string

	// Id for Stripe/Affirm
	ChargeId string

	// Stripe only.
	BalanceTransaction string

	// PayPal only.
	PayKey         string
	PreapprovalKey string

	// Affirm only.
	CaptureId     string
	TransactionId string

	Gateway PaymentGateway

	Amount         Cents
	AmountRefunded Cents

	CreatedAt time.Time

	Status PaymentStatus

	// Client's browser, associated info
	Client Client

	// Whether this was a transaction in production or a testing sandbox
	Live bool
}
