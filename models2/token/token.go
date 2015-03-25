package token

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
)

type Token struct {
	mixin.Model

	Email   string `json:"email"`
	UserId  string `json:"userId"`
	Used    bool   `json:"used"`
	Expired bool   `json:"expired"`
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Model = mixin.Model{Db: db, Entity: t}
	return t
}

func (t Token) Kind() string {
	return "token2"
}

func (t *Token) Validator() *val.Validator {
	return val.New(t)
}
