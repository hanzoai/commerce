package wallet

import (
	"time"

	"hanzo.io/models/mixin"
	"hanzo.io/util/tokensale/ethereum"
)

type Wallet struct {
	mixin.Model

	Accounts []Account `json:"accounts,omitempty"`
}

// Create a new Account, saves if wallet is created
func (w *Wallet) CreateAccount(typ Type, withPassword []byte) (Account, error) {
	switch typ {
	case Ethereum:
		priv, pub, add, err := ethereum.GenerateKeyPair()

		if err != nil {
			return Account{}, err
		}

		a := Account{
			PrivateKey: priv,
			PublicKey:  pub,
			Address:    add,
			Type:       typ,
			CreatedAt:  time.Now(),
		}

		if err := a.Encrypt(withPassword); err != nil {
			return Account{}, err
		}

		w.Accounts = append(w.Accounts, a)

		// Only save if the wallet is created
		// Otherwise let the user manage that
		if w.Created() {
			if err := w.Update(); err != nil {
				return Account{}, err
			}
		}

		return a, nil
	}

	return Account{}, InvalidTypeSpecified
}
