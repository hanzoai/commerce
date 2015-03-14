package models

import (
	"crowdstart.io/util/rand"
)

type Token struct {
	Id      string
	Email   string
	UserId  string
	Used    bool
	Expired bool
}

func (t *Token) GenerateId() {
	t.Id = rand.ShortId()
}
