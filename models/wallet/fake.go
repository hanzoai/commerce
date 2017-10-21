package wallet

import (
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/util/rand"
)

func Fake(db *datastore.Datastore) (*Wallet, Account, string) {
	w := New(db)
	password := rand.ShortPassword()
	a, _ := w.CreateAccount("Fake Account", blockchains.EthereumRopstenType, []byte(password))
	return w, a, password
}
