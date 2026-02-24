package cryptobalance

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/types"
)

var kind = "crypto-balance"

func (cb CryptoBalance) Kind() string {
	return kind
}

func (cb *CryptoBalance) Init(db *datastore.Datastore) {
	cb.Model.Init(db, cb)
}

func (cb *CryptoBalance) Defaults() {
	cb.Parent = cb.Db.NewKey("synckey", "", 1, nil)
	if cb.Balance == "" {
		cb.Balance = "0"
	}
	if cb.Reserved == "" {
		cb.Reserved = "0"
	}
	if cb.Metadata == nil {
		cb.Metadata = make(Map)
	}
}

func New(db *datastore.Datastore) *CryptoBalance {
	cb := new(CryptoBalance)
	cb.Init(db)
	cb.Defaults()
	return cb
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
