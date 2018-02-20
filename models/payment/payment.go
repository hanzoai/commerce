package payment

import (
	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/fee"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"

	. "hanzo.io/models"
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

type Type string

const (
	Affirm   Type = "affirm"
	Balance  Type = "balance"
	Ethereum Type = "ethereum"
	Bitcoin  Type = "bitcoin"
	Null     Type = "null"
	PayPal   Type = "paypal"
	Stripe   Type = "stripe"
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
	Name   string `json:"name,omitempty" datastore:"-"`
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

type BitcoinTransaction struct {
	BitcoinTransactionTxId string           `json:"bitcoinTransactionTxId,omitempty"`
	BitcoinChainType       blockchains.Type `json:"bitcoinChainType,omitempty"`
	BitcoinAmount          currency.Cents   `json:"bitcoinAmount,omitempty"`

	// Denotes whether or not a successful transfer occurred
	BitcoinTransferred bool `json:"bitcoinTransfered"`

	BitcoinFinalTransactionTxId string         `json:"finalbitcoinTransactionTxId,omitempty"`
	BitcoinFinalTransactionCost currency.Cents `json:"finalbitcoinTransactionCost,omitempty"`
	BitcoinFinalAddress         string         `json:"finalbitcoinAddress,omitempty"`
	BitcoinFinalAmount          currency.Cents `json:"finalbitcoinAmount,omitempty"`
}

type EthereumTransaction struct {
	EthereumTransactionHash string                `json:"ethereumTransactionHash,omitempty"`
	EthereumChainType       blockchains.Type      `json:"ethereumChainType,omitempty"`
	EthereumAmount          blockchains.BigNumber `json:"ethereumAmount,omitempty"`

	// Denotes whether or not a successful transfer occurred
	EthereumTransferred bool `json:"ethereumTransfered"`

	EthereumFinalTransactionHash string                `json:"finalEthereumTransactionHash,omitempty"`
	EthereumFinalTransactionCost blockchains.BigNumber `json:"finalEthereumTransactionCost,omitempty"`
	EthereumFinalAddress         string                `json:"finalEthereumAddress,omitempty"`
	EthereumFinalAmount          blockchains.BigNumber `json:"finalEthereumAmount,omitempty"`
}

// Sort of a union type of all possible payment accounts, used everywhere for convenience
type Account struct {
	AffirmAccount
	PayPalAccount
	StripeAccount
	BitcoinTransaction
	EthereumTransaction

	Error string `json:"error,omitempty"`
}

type Payment struct {
	mixin.Model

	Type Type `json:"type"`

	// Order this payment is associated with
	OrderId string `json:"orderId,omitempty"`

	// User this payment is associated with
	UserId string `json:"userId,omitempty"`

	// Payment source information
	Account Account `json:"account"`

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

func (p *Payment) Load(ps datastore.PropertyList) (err error) {
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

func (p *Payment) Save() (ps datastore.PropertyList, err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	// Save properties
	return datastore.SaveStruct(p)
}
