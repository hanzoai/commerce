package wallet

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/util/rand"
)

func Fake(db *datastore.Datastore) (*Wallet, *Account, string) {
	w := New(db)
	password := rand.ShortPassword()
	a, _ := w.CreateAccount("Fake Account", blockchains.EthereumRopstenType, []byte(password))
	return w, a, password
}
