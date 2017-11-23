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

var SendTestBitcoinTransaction = New("send-test-bitcoin-transaction", func(c *gin.Context) {
	transactionId := "e2a49ed572d18bfb8dca73ab805866fa3cf01bd5aeb1a7da7707e48bb94a2749"
	//transactionId2 := "14f8d758bcd324a3e4c9a85c46a45e156a57bff160bc2ff70a090af6dc3b44dd"
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

	in := []bitcoin.Origin{
		bitcoin.Origin{TxId: transactionId, OutputIndex: 0},
		//bitcoin.Origin{TxId: transactionId2, OutputIndex: 0},
	}
	out := []bitcoin.Destination{bitcoin.Destination{Value: 100000, Address: receiver1.TestNetAddress}, bitcoin.Destination{Value: 500000, Address: receiver2.TestNetAddress}}
	senderAccount := bitcoin.Sender{
		PrivateKey:     sender.PrivateKey,
		PublicKey:      sender.PublicKey,
		Address:        sender.Address,
		TestNetAddress: sender.TestNetAddress,
	}

	client, err := bitcoin.NewRpcClient(db.Context, config.Bitcoin.TestNetNodes[0], config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0], true)
	if err != nil {
		panic(err)
	}
	log.Info("Created Bitcoin client.")

	rawTrx, err := bitcoin.CreateTransaction(client, in, out, senderAccount)
	if err != nil {
		panic(err)
	}

	log.Info("Raw transaction hex: %v", rawTrx)
	res, err := client.SendRawTransaction(rawTrx)
	if err != nil {
		panic(err)
	}

	log.Info("Btcd Node Response: %v", "", res)
})
