package token

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/rand"
)

type Token struct {
	*mixin.Model `datastore:"-"`

	ShortId string
	Email   string
	UserId  string
	Used    bool
	Expired bool
}

func New(db *datastore.Datastore) *Token {
	t := new(Token)
	t.Model = mixin.NewModel(db, t)
	// t.Model.StringKey_ = true
	return t
}

func (t Token) Kind() string {
	return "token2"
}

func (t *Token) Generate() string {
	t.ShortId = rand.ShortId()
	return t.ShortId
}
