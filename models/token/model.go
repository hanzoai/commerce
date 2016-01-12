package token

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (t Token) Kind() string {
	return "token"
}

func (t *Token) Init(db *datastore.Datastore) {
	t.Model = mixin.Model{Db: db, Entity: t}
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
