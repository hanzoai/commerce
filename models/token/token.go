package token

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
)

type Token struct {
	mixin.Model

	Email   string    `json:"email"`
	UserId  string    `json:"userId"`
	Used    bool      `json:"used"`
	Expires time.Time `json:"expires"`
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Model = mixin.Model{Db: db, Entity: t}
	return t
}

func (t Token) Kind() string {
	return "token"
}

func (t *Token) Validator() *val.Validator {
	return val.New()
}

func (t Token) Expired() bool {
	if t.Used || time.Now().After(t.Expires) {
		return true
	}
	return false
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
