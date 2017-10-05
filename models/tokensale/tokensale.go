package tokensale

import (
	"hanzo.io/models/mixin"
	// "hanzo.io/models/payment"
	// "hanzo.io/models/types/pricing"
	"hanzo.io/models/wallet"
)

type TokenSale struct {
	mixin.Model

	// Auditor Wallet
	wallet.WalletHolder `json:"-"`

	Name        string `json:"name"`
	TotalTokens int    `json:"totalTokens"`

	// // Fee structure for this tokensale
	// Fees pricing.Fees `json:"fees" datastore:",noindex"`

	// // Partner fees (private, should be up to partner to disclose)
	// Partners []pricing.Partner `json:"-" datastore:",noindex"`

	// Slug string `json:"slug"`

	// SupportedPaymentTypes []payment.Type `json:"supportedPaymentTypes`

	// Passphrase for the wallet accounts the order controls, never send to the client
	WalletPassphrase string `json:"-"`
}

// func (ts TokenSale) Pricing() (*pricing.Fees, []pricing.Partner) {
// 	// Ensure our id is set on fees used
// 	fees := ts.Fees
// 	fees.Id = ts.Id()
// 	return &fees, ts.Partners
// }
