package fixtures

import (
	"errors"
	//"math/big"

	"bytes"
	"encoding/hex"
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/util/log"
)

var SendTestBitcoinTransaction = New("send-test-bitcoin-transaction", func(c *gin.Context) {
	db := datastore.New(c)
	ctx := db.Context

	w := wallet.New(db)
	w.Id_ = "test-btc-wallet"
	w.UseStringKey = true
	w.GetOrCreate("Id_=", "test-btc-wallet")

	if len(w.Accounts) == 0 {
		if _, err := w.CreateAccount("Test input account", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
		if _, err := w.CreateAccount("Test output account 1", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
		if _, err := w.CreateAccount("Test output account 2", blockchains.BitcoinTestnetType, []byte(config.Bitcoin.TestPassword)); err != nil {
			panic(err)
		}
	}

	sender, ok := w.GetAccountByName("Test input account")
	if !ok {
		panic(errors.New("Sender Account Not Found."))
	}
	receiver1, ok := w.GetAccountByName("Test output account 1")
	if !ok {
		panic(errors.New("Sender Account Not Found."))
	}
	receiver2, ok := w.GetAccountByName("Test output account 2")
	if !ok {
		panic(errors.New("Sender Account Not Found."))
	}

	log.Info("Accounts Found", ctx)
	log.Info("Sender Address", sender.TestNetAddress)
	log.Info("Receiver 1 Address", receiver1.TestNetAddress)
	log.Info("Receiver 2 Address", receiver2.TestNetAddress)
	if err := sender.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}
	if err := receiver1.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}
	if err := receiver2.Decrypt([]byte(config.Bitcoin.TestPassword)); err != nil {
		panic(err)
	}

	tempScript := bitcoin.CreateScriptPubKey(sender.TestNetAddress)
	log.Info("Created temporary script key.")
	rawTransaction := bitcoin.CreateRawTransaction([]string{"5b60d0684a8201ddac20f713782a1f03682b508e90d99d0887b4114ad4ccfd2c"}, []int{0}, []string{receiver1.TestNetAddress}, []int{1000}, tempScript)
	log.Info("Created initial raw transaction.")
	hashCodeType, err := hex.DecodeString("01000000")
	if err != nil {
		log.Fatal(err)
	}

	var rawTransactionBuffer bytes.Buffer
	rawTransactionBuffer.Write(rawTransaction)
	rawTransactionBuffer.Write(hashCodeType)
	rawTransactionWithHashCodeType := rawTransactionBuffer.Bytes()
	finalSignature, err := bitcoin.GetRawTransactionSignature(rawTransactionWithHashCodeType, sender.PrivateKey)
	if err != nil {
		panic(err)
	}
	log.Info("Created final signature.")
	rawTrx := bitcoin.CreateRawTransaction([]string{"5b60d0684a8201ddac20f713782a1f03682b508e90d99d0887b4114ad4ccfd2c"}, []int{0}, []string{receiver1.TestNetAddress}, []int{1000}, finalSignature)
	log.Info("Created final transaction.")

	client, err := bitcoin.New(db.Context, config.Bitcoin.TestNetNodes[0], config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0])
	if err != nil {
		panic(err)
	}
	log.Info("Created Bitcoin client.")

	res, err := client.SendRawTransaction(rawTrx)
	if err != nil {
		panic(err)
	}

	log.Info("Btcd Node Response: %v", "", res)
})
