package wallet

import (
	"strings"
	"time"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/blockchains/blockaddress"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/thirdparty/bitcoin"
	"github.com/hanzoai/commerce/thirdparty/ethereum"
	"github.com/hanzoai/commerce/util/hashid"
)

type Wallet struct {
	mixin.BaseModel

	Accounts []Account `json:"accounts,omitempty"`
}

// Create a new Account, saves if wallet is created
func (w *Wallet) CreateAccount(name string, typ blockchains.Type, withPassword []byte) (*Account, error) {
	_, found := w.GetAccountByName(name)

	if found {
		return nil, ErrorNameCollision
	}

	var a *Account

	switch typ {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		priv, pub, add, err := ethereum.GenerateKeyPair()

		if err != nil {
			return nil, err
		}

		add = strings.ToLower(add)

		a = &Account{
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
			return nil, err
		}

		var add string

		switch typ {
		case blockchains.BitcoinType:
			add, _, err = bitcoin.PubKeyToAddress(pub, false)
			if err != nil {
				return nil, err
			}
		case blockchains.BitcoinTestnetType:
			add, _, err = bitcoin.PubKeyToAddress(pub, true)
			if err != nil {
				return nil, err
			}
		}

		a = &Account{
			Name:       name,
			PrivateKey: priv,
			PublicKey:  pub,
			Address:    add,
			Type:       typ,
			CreatedAt:  time.Now(),
		}

	default:
		return nil, ErrorInvalidTypeSpecified
	}

	if err := a.Encrypt(withPassword); err != nil {
		return nil, err
	}

	log.Debug("Attempting append to w.Accounts. Current state: %v", w.Accounts)
	log.Debug("Appending a. Current state: %v", a)
	w.Accounts = append(w.Accounts, *a)

	// Create a blockaddress so we track this in the readers
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

	// Only save if the wallet is created
	// Otherwise let the user manage that
	if w.Created() {
		if err := w.Update(); err != nil {
			return nil, err
		}
	}

	return &w.Accounts[len(w.Accounts)-1], err
}

func (w *Wallet) GetAccountByName(name string) (*Account, bool) {
	// Find The Test Account
	for i, a := range w.Accounts {
		if a.Name != name {
			continue
		}

		// Lazy Migration to Remove TestNetAddress as needed
		if a.Type == blockchains.BitcoinTestnetType && a.AddressBackup == "" && a.TestNetAddress != "" {
			log.Warn("Migrating TestNetAddress '%s', overwriting '%s'", a.TestNetAddress, a.Address, w.Context())
			w.Accounts[i].AddressBackup = a.Address
			w.Accounts[i].Address = a.TestNetAddress
			w.Accounts[i].TestNetAddress = ""
			w.MustUpdate()

			a = w.Accounts[i]
		}

		return &a, true
	}

	return nil, false
}
