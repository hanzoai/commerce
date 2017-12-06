package fixtures

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/log"
)

var CheckEthereumBalance = New("check-ethereum-balance", func(c *gin.Context) {
	db := datastore.New(c)
	ctx := db.Context

	w := wallet.New(db)
	w.Id_ = "test-customer-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "test-customer-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Test Customer Account", blockchains.EthereumRopstenType, []byte(config.Ethereum.TestPassword)); err != nil {
			panic(err)
		}
	}

	pw := wallet.New(db)
	pw.GetOrCreate("Id_=", "platform-wallet")

	// Find The Test Account
	account, ok := pw.GetAccountByName("Ethereum Ropsten Test Account")
	if !ok {
		panic(errors.New("Platform Account Not Found."))
	}

	log.Info("Account Found", ctx)
	if err := account.Decrypt([]byte(config.Ethereum.TestPassword)); err != nil {
		panic(err)
	}

	client := ethereum.New(db.Context, config.Ethereum.TestNetNodes[0])

	balance, err := client.GetBalance(account.Address)
	if err != nil {
		panic(err)
	}

	log.Info("Geth Node Response: %v", balance, c)
})
