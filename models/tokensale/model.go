package tokensale

import (
	"hanzo.io/datastore"
)

var kind = "tokensale"

func (ts TokenSale) Kind() string {
	return kind
}

func (ts *TokenSale) Init(db *datastore.Datastore) {
	ts.Model.Init(db, ts)
}

func (ts *TokenSale) Defaults() {
}

func New(db *datastore.Datastore) *TokenSale {
	ts := new(TokenSale)
	ts.Init(db)
	ts.Defaults()
	return ts
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
