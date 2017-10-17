package tokensale

import (
	"hanzo.io/models/wallet"
	"hanzo.io/util/rand"
)

// Hooks
func (ts *TokenSale) BeforeCreate() error {
	ts.WalletPassphrase = rand.SecretKey()
	w, err := ts.GetOrCreateWallet(ts.Db)
	if err != nil {
		return err
	}

	_, err = w.CreateAccount("default", wallet.Ethereum, []byte(ts.WalletPassphrase))
	return err
}
