package tokensale

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/payment"
	"hanzo.io/models/wallet"
)

type TokenSale struct {
	mixin.Model

	// Auditor Wallet
	wallet.WalletHolder `json:"-"`

	Name string `json:"name"`
	// Slug string `json:"slug"`

	SupportedPaymentTypes []payment.Type `json:"supportedPaymentTypes`

	// Passphrase for the wallet accounts the order controls, never send to the client
	WalletPassphrase string `json:"-"`
}
