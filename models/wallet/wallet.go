package wallet

import (
	"strings"
	"time"

	"hanzo.io/models/blockchains"
	"hanzo.io/models/blockchains/blockaddress"
	"hanzo.io/models/mixin"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/hashid"
	"hanzo.io/util/log"
)

type Wallet struct {
	mixin.Model

	Accounts []Account `json:"accounts,omitempty"`
}

// Create a new Account, saves if wallet is created
func (w *Wallet) CreateAccount(name string, typ blockchains.Type, withPassword []byte) (Account, error) {
	var a Account

	switch typ {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		priv, pub, add, err := ethereum.GenerateKeyPair()

		if err != nil {
			return Account{}, err
		}

		add = strings.ToLower(add)

		a = Account{
			Name:       name,
			PrivateKey: priv,
			PublicKey:  pub,
			Address:    add,
			Type:       typ,
			CreatedAt:  time.Now(),
		}

	case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
		priv, pub, err := bitcoin.GenerateKeyPair()
		if err != nil {
			return Account{}, err
		}

		add, _, err := bitcoin.PubKeyToAddress(pub, false)
		if err != nil {
			return Account{}, err
		}
		testadd, _, err := bitcoin.PubKeyToAddress(pub, true)
		if err != nil {
			return Account{}, err
		}

		a = Account{
			Name:           name,
			PrivateKey:     priv,
			PublicKey:      pub,
			Address:        add,
			TestNetAddress: testadd,
			Type:           typ,
			CreatedAt:      time.Now(),
		}

		// TODO: Update account to do this when we have time
		// var add string
		// switch typ {
		// case blockchains.BitcoinType:
		// 	add, _, err = bitcoin.PubKeyToAddress(pub, false)
		// 	if err != nil {
		// 		return Account{}, err
		// 	}
		// case blockchains.BitcoinTestnetType:
		// 	add, _, err = bitcoin.PubKeyToAddress(pub, true)
		// 	if err != nil {
		// 		return Account{}, err
		// 	}
		// default:
		// 	log.Fatal("This shouldn't be possible", w.Context())
		// }

		// a = Account{
		// 	Name:       name,
		// 	PrivateKey: priv,
		// 	PublicKey:  pub,
		// 	Address:    add,
		// 	Type:       typ,
		// 	CreatedAt:  time.Now(),
		// }
	default:
		return Account{}, InvalidTypeSpecified
	}

	if err := a.Encrypt(withPassword); err != nil {
		return Account{}, err
	}

	w.Accounts = append(w.Accounts, a)

	// Create a blockaddress so we track this in the ethereum reader
	ba := blockaddress.New(w.Db)
	ba.Address = a.Address
	ba.Type = typ
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

	// There's a testnet id for every btc account I guess?
	if a.TestNetAddress != "" {
		// Create a blockaddress so we track this in the ethereum reader
		tba := blockaddress.New(w.Db)
		tba.Address = a.Address
		tba.Type = typ
		tba.WalletId = w.Id()

		tba.WalletNamespace = ns
		err = tba.Create()
		if err != nil {
			log.Error("Could not create BlockAddress: %v", err, w.Context())
		}
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

func (w *Wallet) GetAccountByName(name string) (*Account, bool) {
	// Find The Test Account
	for _, a := range w.Accounts {
		if a.Name != name {
			continue
		}
		return &a, true
	}

	return nil, false
}
