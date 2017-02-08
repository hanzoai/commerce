package token

import (
	"time"

	"hanzo.io/models/mixin"
)

type Token struct {
	mixin.Model

	Email   string    `json:"email"`
	UserId  string    `json:"userId"`
	Used    bool      `json:"used"`
	Expires time.Time `json:"expires"`
}

func (t Token) Expired() bool {
	if t.Used || time.Now().After(t.Expires) {
		return true
	}
	return false
}
