package wallet

import (
	"hanzo.io/models/mixin"
)

type Account struct {
	Private string `json:"privateKey,omitempty" datastore:"-"`
	Address string `json:"address,omitempty"`
}

type Wallet struct {
	mixin.Model

	Encrypted []byte    `json:"encrypted,omitempty"`
	Accounts  []Account `json:"accounts,omitempty"`
}

func (w *Wallet) CreateAccount(withPassword []byte) {

}
