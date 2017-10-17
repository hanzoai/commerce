package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/wallet"
)

var TestWallet = New("test-wallet", func(c *gin.Context) *wallet.Wallet {
	db := datastore.New(c)

	w := wallet.New(db)
	w.Id_ = "test-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "test-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Ropsten Test Account", wallet.Ethereum, []byte(config.Ethereum.TestPrivateKey)); err != nil {
			panic(err)
		}
	}

	return w
})
