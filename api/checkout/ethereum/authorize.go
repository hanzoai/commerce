package ethereum

import (
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/rand"
)

// This creates the wallet for
func Authorize(org *organization.Organization, ord *order.Order, usr *user.User) error {
	w, err := ord.GetOrCreateWallet(ord.Db)
	if err != nil {
		return err
	}

	ord.WalletPassphrase = rand.SecretKey()

	_, err = w.CreateAccount("Receiver Account", wallet.Ethereum, []byte(ord.WalletPassphrase))

	// id := rand.Int64()

	// if ord.Test {
	// 	ord.
	// }
	return err
}
