package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/wallet"
)

// This wallet stores special platform level Addresses
var PlatformWallet = New("platform-wallet", func(c *gin.Context) *wallet.Wallet {
	db := datastore.New(c)

	w := wallet.New(db)
	w.Id_ = "platform-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "platform-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Ethereum Ropsten Test Account", wallet.Ethereum, []byte(config.Ethereum.TestPassword)); err != nil {
			panic(err)
		}

		if _, err := w.CreateAccount("Ethereum Deposit Account", wallet.Ethereum, []byte(config.Ethereum.DepositPassword)); err != nil {
			panic(err)
		}
	}

	return w
})
