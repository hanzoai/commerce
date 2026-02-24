package oauthtoken

import (
	"github.com/hanzoai/commerce/datastore"
)

var kind = "oauthtoken"

func (t Token) Kind() string {
	return kind
}

func (t *Token) Init(db *datastore.Datastore) {
	t.BaseModel.Init(db, t)
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
