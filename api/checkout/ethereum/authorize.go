package ethereum

import (
	"errors"
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

var PlatformWalletNotFound = errors.New("Platform Wallet Not Found.")
var PlatformAccountNotFound = errors.New("Platform Account Not Found.")

// This creates the wallet for
func Authorize(org *organization.Organization, ord *order.Order, usr *user.User) error {
	ctx := org.Db.Context

	w, err := ord.GetOrCreateWallet(ord.Db)
	if err != nil {
		return err
	}

	ord.WalletPassphrase = rand.SecretKey()

	if ord.Test {
		var account wallet.Account
		pw := wallet.New(org.Db)
		if ok, err := pw.Query().Filter("Id_=", "platform-wallet").Get(); !ok {
			if err != nil {
				return err
			}
			return PlatformWalletNotFound
		}

		// Find The Test Account
		for _, a := range pw.Accounts {
			log.Info("Account %s ?= %s", a.Name, "Ethereum Ropsten Test Account", ctx)
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

		if account.PrivateKey == "" {
			return PlatformAccountNotFound
		}

		log.Info("Ethereum Test Mode", ctx)
		if _, err = w.CreateAccount("Receiver Account", blockchains.EthereumRopstenType, []byte(ord.WalletPassphrase)); err != nil {
			return err
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
	} else {
		log.Info("Ethereum Production Mode", ctx)
		if _, err = w.CreateAccount("Receiver Account", blockchains.EthereumType, []byte(ord.WalletPassphrase)); err != nil {
			return err
		}
	}
	return err
}
