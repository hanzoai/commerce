package ethereum

import (
	"errors"
	// "math/big"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	// "hanzo.io/thirdparty/bitcoin"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

var PlatformWalletNotFound = errors.New("Platform Wallet Not Found.")
var PlatformAccountNotFound = errors.New("Platform Account Not Found.")
var PlatformAccountDecryptionFailed = errors.New("Platform Account Decryption Failed.")

// This creates the wallet for
func Authorize(org *organization.Organization, ord *order.Order, usr *user.User) error {
	ctx := org.Db.Context

	w, err := ord.GetOrCreateWallet(ord.Db)
	if err != nil {
		return err
	}

	ord.WalletPassphrase = rand.SecretKey()

	if ord.Test {
		pw := wallet.New(org.Db)
		if ok, err := pw.Query().Filter("Id_=", "platform-wallet").Get(); !ok {
			if err != nil {
				return err
			}
			return PlatformWalletNotFound
		}

		// Find The Test Account
		account, ok := pw.GetAccountByName("Bitcoin Test Account")
		if !ok {
			return PlatformAccountNotFound
		}

		log.Info("Account Found", ctx)
		if err := account.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
			return err
		}

		if account.PrivateKey == "" {
			return PlatformAccountDecryptionFailed
		}

		log.Info("Bitcoin Test Mode", ctx)
		if _, err = w.CreateAccount("Receiver Account", blockchains.BitcoinTestnetType, []byte(ord.WalletPassphrase)); err != nil {
			return err
		}

		// client := bitcoin.NewRPCClient(org.Db.Context, config.Bitcoin.TestNetNodes[0], config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0], true)

		// in := []bitcoin.Origin{
		// 	bitcoin.Origin{TxId: transactionId, OutputIndex: 0},
		// 	// bitcoin.Origin{TxId: transactionId2, OutputIndex: 0},
		// }

		// if _, err := client.SendTransaction(
		// 	bitcoin.TestNet,
		// 	account.PrivateKey,
		// 	account.Address,
		// 	w.Accounts[0].Address,
		// 	ord.Currency.ToMinimalUnits(ord.Total),
		// 	big.NewInt(0),
		// 	big.NewInt(0),
		// 	[]byte{},
		// ); err != nil {
		// 	return err
		// }
	} else {
		log.Info("Bitcoin Production Mode", ctx)
		if _, err = w.CreateAccount("Receiver Account", blockchains.EthereumType, []byte(ord.WalletPassphrase)); err != nil {
			return err
		}
	}
	return err
}
