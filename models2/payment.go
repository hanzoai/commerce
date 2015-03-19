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
	Ip        string `json:"ip,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Language  string `json:"language,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

type PaymentAccount struct {
	Name string `json:"name,omitempty"`

	Type PaymentType `json:"type"`

	Buyer Buyer `json:"buyer"`

	// Optionally associated with a user
	UserId string `json:"userId,omitempty"`

	Country string `json:"country,omitempty"`

	Affirm struct {
		CheckoutToken string `json:"checkoutToken,omitempty"`
	} `json:"affirm,omitempty"`

	Stripe struct {
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
	} `json:"stripe,omitempty"`

	Paypal struct {
		Email       string `json:"email,omitempty"`
		SellerEmail string `json:"sellerEmail,omitempty"`
		RedirectUrl string `json:"redirectUrl,omitempty"`
		Ipn         string `json:"ipn,omitempty"`

		// Preapproval expiration date (Unix timestamp in milliseconds).
		Ending int `json:"ending,omitempty"`

		// Preapproval expiration date (ISO 8601 timestamp).
		EndingDate string `json:"endingDate,omitempty"`
	} `json:"paypal,omitempty"`
}

type Payment struct {
	// Payment source information
	Account PaymentAccount `json:"account"`

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

	Status PaymentStatus `json:"status"`

	// Client's browser, associated info
	Client Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Whether this was a transaction in production or a testing sandbox
	Live bool `json:"live"`
}
