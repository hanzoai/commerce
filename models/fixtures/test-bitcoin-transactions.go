package fixtures

import (
	"errors"
	//"math/big"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/util/log"
)

var SendTestBitcoinTransaction = New("send-test-ethereum-transaction", func(c *gin.Context) {
	db := datastore.New(c)
	ctx := db.Context

	w := wallet.New(db)
	w.Id_ = "test-bitcoin-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "test-bitcoin-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Test Customer Account", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
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
	if err := account.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}

	_, err := bitcoin.New(db.Context, config.Bitcoin.TestNetNodes[0], "", config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0], nil)
	if err != nil {
		panic(err)
	}
	//hash, err := client.SendTransaction(ethereum.Ropsten, account.PrivateKey, account.Address, w.Accounts[0].Address, big.NewInt(1000000000000000), big.NewInt(0), big.NewInt(0), []byte{})
	if err != nil {
		panic(err)
	}

	log.Info("Geth Node Response: %v", "", c)
})
