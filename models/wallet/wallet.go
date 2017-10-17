package wallet

import (
	"time"

	"hanzo.io/models/blockchains"
	"hanzo.io/models/blockchains/blockaddress"
	"hanzo.io/models/mixin"
	"hanzo.io/util/hashid"
	"hanzo.io/util/log"
	"hanzo.io/util/tokensale/ethereum"
)

type Wallet struct {
	mixin.Model

	Accounts []Account `json:"accounts,omitempty"`
}

// Create a new Account, saves if wallet is created
func (w *Wallet) CreateAccount(name string, typ Type, withPassword []byte) (Account, error) {
	switch typ {
	case Ethereum:
		priv, pub, add, err := ethereum.GenerateKeyPair()

		if err != nil {
			return Account{}, err
		}

		a := Account{
			Name:       name,
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

		// Create a blockaddress so we track this in the ethereum reader
		ba := blockaddress.New(w.Db)
		ba.Address = add
		ba.Type = blockchains.EthereumType
		ba.WalletId = w.Id()

		ns, err := hashid.GetNamespace(w.Db.Context, w.Id())
		if err != nil {
			log.Warn("Could not determine namespace, probably '': %v", err, w.Context())
		}

		ba.WalletNamespace = ns
		err = ba.Create()
		if err != nil {
			log.Error("Could not create BlockAddress: %v", err, w.Context())
		}

		// Only save if the wallet is created
		// Otherwise let the user manage that
		if w.Created() {
			if err := w.Update(); err != nil {
				return Account{}, err
			}
		}

		return a, err
	}

	return Account{}, InvalidTypeSpecified
}
