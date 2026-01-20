package wallet

import "github.com/hanzoai/commerce/datastore"

var kind = "wallet"

func (w Wallet) Kind() string {
	return kind
}

func (w *Wallet) Init(db *datastore.Datastore) {
	w.Model.Init(db, w)
}

func (w *Wallet) Defaults() {
	w.Accounts = make([]Account, 0, 0)
}

func New(db *datastore.Datastore) *Wallet {
	w := new(Wallet)
	w.Init(db)
	w.Defaults()
	return w
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
