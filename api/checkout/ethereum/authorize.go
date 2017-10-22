package ethereum

import (
	"math/big"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

// This creates the wallet for
func Authorize(org *organization.Organization, ord *order.Order, usr *user.User) error {
	ctx := org.Db.Context

	w, err := ord.GetOrCreateWallet(ord.Db)
	if err != nil {
		return err
	}

	ord.WalletPassphrase = rand.SecretKey()

	_, err = w.CreateAccount("Receiver Account", blockchains.EthereumType, []byte(ord.WalletPassphrase))

	if ord.Test {
		var account wallet.Account
		pw := wallet.New(org.Db)
		if _, err := w.Query().Filter("Id_=", "platform-wallet").Get(); err != nil {
			return err
		}

		// Find The Test Account
		for _, a := range pw.Accounts {
			if a.Name != "Ethereum Ropsten Test Account" {
				continue
			}
			log.Info("Account Found", ctx)
			if err := a.Decrypt([]byte(config.Ethereum.TestPassword)); err != nil {
				panic(err)
			}
			account = a
			break
		}

		client := ethereum.New(org.Db.Context, config.Ethereum.TestNetNodes[0])

		if _, err := client.SendTransaction(
			ethereum.Ropsten,
			account.PrivateKey,
			account.Address,
			w.Accounts[0].Address,
			ord.Currency.ToMinimalUnits(ord.Total),
			big.NewInt(0),
			big.NewInt(0),
			[]byte{},
		); err != nil {
			return err
		}
	}
	return err
}
