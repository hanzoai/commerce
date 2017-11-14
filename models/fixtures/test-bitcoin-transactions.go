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
		if _, err := w.CreateAccount("Test input account", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
	}

	sender, ok := w.GetAccountByName("Test input account")
	if !ok {
		panic(errors.New("Sender Account Not Found."))
	}

	pw := wallet.New(db)
	pw.GetOrCreate("Id_=", "platform-wallet")

	// Find The Test Account
	receiver1, ok := pw.GetAccountByName("Test output account 1")
	if !ok {
		if _, err := pw.CreateAccount("Test output account 1", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
	}

	receiver2, ok := pw.GetAccountByName("Test output account 2")
	if !ok {
		if _, err := pw.CreateAccount("Test output account 2", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
	}

	log.Info("Accounts Found", ctx)
	if err := sender.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}
	if err := receiver1.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}
	if err := receiver2.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}

	tempScript := bitcoin.CreateScriptPubKey(sender.PublicKey)
	rawTransaction := bitcoin.CreateRawTransaction([]string{""}, []int{0}, []string{receiver1.PublicKey, receiver2.PublicKey}, []int{1000, 5000}, tempScript)
	finalSignature, err := bitcoin.GetRawTransactionSignature(rawTransaction, sender.PrivateKey)
	if err != nil {
		panic(err)
	}
	_ = bitcoin.CreateRawTransaction([]string{""}, []int{0}, []string{receiver1.PublicKey, receiver2.PublicKey}, []int{1000, 5000}, finalSignature)

	_, err = bitcoin.New(db.Context, config.Bitcoin.TestNetNodes[0], "", config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0], nil)
	if err != nil {
		panic(err)
	}

	//hash, err := client.SendTransaction(ethereum.Ropsten, account.PrivateKey, account.Address, w.Accounts[0].Address, big.NewInt(1000000000000000), big.NewInt(0), big.NewInt(0), []byte{})
	if err != nil {
		panic(err)
	}

	log.Info("Geth Node Response: %v", "", c)
})
