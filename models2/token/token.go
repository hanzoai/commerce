package token

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/rand"
)

type Token struct {
	mixin.Model

	ShortId string `json:"shortId"`
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

func (t *Token) Generate() string {
	t.ShortId = rand.ShortId()
	return t.ShortId
}
