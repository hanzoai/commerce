package wallet

import (
	"hanzo.io/datastore"
	"hanzo.io/util/rand"
)

func Fake(db *datastore.Datastore) (*Wallet, Account, string) {
	w := New(db)
	password := rand.ShortPassword()
	a, _ := w.CreateAccount(Ethereum, []byte(password))
	return w, a, password
}
