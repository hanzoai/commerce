package models

type PaymentStatus string

const (
	PaymentDisputed   PaymentStatus = "disputed"
	PaymentFailed                   = "failed"
	PaymentFraudulent               = "fraudulent"
	PaymentPaid                     = "paid"
	PaymentRefunded                 = "refunded"
	PaymentUnpaid                   = "unpaid"
)

type PaymentType string

const (
	Stripe PaymentType = "stripe"
	Affirm             = "affirm"
	PayPal             = "paypal"
)

type Client struct {
	Ip        string
	UserAgent string
	Language  string
	Referer   string
}

type PaymentAccount struct {
	Name string `json:"name"`

	Type PaymentType `json:"type"`

	Buyer Buyer `json:"buyer"`

	// Optionally associated with a user
	UserId string `json:"userId"`

	Country string `json:"country"`

	Affirm struct {
		CheckoutToken string `json:"checkoutToken"`
	}

	Stripe struct {
		CustomerId string `json:"customerId"`
		ChargeId   string `json:"chargeId"`
		CardType   string `json:"cardType"`
		Last4      string `json:"last4"`
		Expiration struct {
			Month int `json:"month"`
			Year  int `json:"year"`
		}
	}

	Paypal struct {
		Email       string `json:"email"`
		SellerEmail string `json:"sellerEmail"`
		RedirectUrl string `json:"redirectUrl"`
		Ipn         string `json:"ipn"`

		// Preapproval expiration date (Unix timestamp in milliseconds).
		Ending int `json:"ending"`

		// Preapproval expiration date (ISO 8601 timestamp).
		EndingDate string `json:"endingDate"`
	}
}

type Payment struct {
	// Payment source information
	Account PaymentAccount `json:"account"`

	Currency CurrencyType `json:"currency"`

	CampaignId string `json:"campaignId"`

	// Id for Stripe/Affirm
	ChargeId string `json:"chargeId"`

	// Stripe only.
	BalanceTransaction string `json:"balanceTransaction"`

	// PayPal only.
	PayKey         string `json:"payKey"`
	PreapprovalKey string `json:"preapprovalKey"`

	// Affirm only.
	CaptureId     string `json:"captureId"`
	TransactionId string `json:"transactionId"`

	Amount         Cents `json:"amount"`
	AmountRefunded Cents `json:"amountRefunded"`

	Status PaymentStatus `json:"status"`

	// Client's browser, associated info
	Client Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Whether this was a transaction in production or a testing sandbox
	Live bool `json:"live"`
}
