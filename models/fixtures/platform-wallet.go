package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
)

// This wallet stores special platform level Addresses
var PlatformWallet = New("platform-wallet", func(c *context.Context) *wallet.Wallet {
	BlockchainNamespace(c)

	db := datastore.New(c)

	w := wallet.New(db)
	w.Id_ = "platform-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "platform-wallet")

	if _, ok := w.GetAccountByName("Ethereum Ropsten Test Account"); !ok {
		if _, err := w.CreateAccount("Ethereum Ropsten Test Account", blockchains.EthereumRopstenType, []byte(config.Ethereum.TestPassword)); err != nil {
			panic(err)
		}
	}

	if _, ok := w.GetAccountByName("Ethereum Deposit Account"); !ok {
		if _, err := w.CreateAccount("Ethereum Deposit Account", blockchains.EthereumType, []byte(config.Ethereum.DepositPassword)); err != nil {
			panic(err)
		}
	}

	if _, ok := w.GetAccountByName("Bitcoin Test Account"); !ok {
		if _, err := w.CreateAccount("Bitcoin Test Account", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
	}

	if _, ok := w.GetAccountByName("Bitcoin Deposit Account"); !ok {
		if _, err := w.CreateAccount("Bitcoin Deposit Account", blockchains.BitcoinType, []byte(config.Bitcoin.DepositPassword)); err != nil {
			panic(err)
		}
	}

	return w
})
