package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/log"
)

var SendTestEthereumTransaction = New("send-test-ethereum-transaction", func(c *gin.Context) {
	db := datastore.New(c)

	w := wallet.New(db)
	w.Id_ = "test-customer-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "test-customer-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Test Customer Account", wallet.Ethereum, []byte(config.Ethereum.TestPassword)); err != nil {
			panic(err)
		}
	}

	aI := -1
	pw := wallet.New(db)
	pw.MustGetById("platform-wallet")
	for i, a := range pw.Accounts {
		if a.Name != "Ethereum Ropsten Test Account" {
			continue
		}
		aI = i
		if err := a.Decrypt([]byte(config.Ethereum.TestPassword)); err != nil {
			panic(err)
		}
	}

	client := ethereum.New(db.Context, config.Ethereum.TestNetNodes[0])

	signedTx, err := ethereum.NewSignedTransaction(ethereum.MainNet, w.Accounts[aI].PrivateKey, pw.Accounts[0].Address, 1000000, 0, 0, []byte{})
	if err != nil {
		panic(err)
	}

	jrr, err := client.SendRawTransaction(signedTx)
	if err != nil {
		panic(err)
	}

	log.Info("Geth Node Response: %v", jrr, c)
})
