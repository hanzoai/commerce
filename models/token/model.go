package token

import "github.com/hanzoai/commerce/datastore"

var kind = "token"

func (t Token) Kind() string {
	return kind
}

func (t *Token) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *Token) Defaults() {
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
