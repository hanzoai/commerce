package wallet

import (
	"time"

	"hanzo.io/models/mixin"
	"hanzo.io/util/crypto/aes"
	"hanzo.io/util/rand"
	"hanzo.io/util/tokensale/ether"
)

type Account struct {
	Encrypted string `json:"encrypted,omitempty"`
	Salt      string `json:"salt,omitempty"`

	PrivateKey string `json:"privateKey,omitempty" datastore:"-"`
	PublicKey  string `json:"publicKey,omitempty"`
	Address    string `json:"address,omitempty"`

	Deleted   bool      `json:"-"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// Encrypt the Account's Private Key
func (a *Account) Encrypt(withPassword []byte) error {
	if a.PrivateKey == "" {
		return NoPrivateKeySetError
	}

	// generate salt
	salt := rand.SecretKey()
	a.Salt = salt

	key, err := aes.AES128KeyFromPassword(withPassword, []byte(salt))

	if err != nil {
		return err
	}

	e, err := aes.EncryptCBC(key, a.PrivateKey)

	if err != nil {
		return err
	}

	a.Encrypted = e

	return nil
}

// Decrypt the Account's Private Key
func (a *Account) Decrypt(withPassword []byte) error {
	if a.Encrypted == "" {
		return NoEncryptedKeyFound
	}

	if a.Salt == "" {
		return NoSaltSetError
	}

	key, err := aes.AES128KeyFromPassword(withPassword, []byte(a.Salt))

	p, err := aes.DecryptCBC(key, a.Encrypted)

	if err != nil {
		return err
	}

	a.PrivateKey = p

	return nil
}

type Wallet struct {
	mixin.Model

	Accounts []Account `json:"accounts,omitempty"`
}

// Create a new Account
func (w *Wallet) CreateAccount(withPassword []byte) (*Account, error) {
	priv, pub, add, err := ether.GenerateKeyPair()

	if err != nil {
		return nil, err
	}

	a := &Account{
		PrivateKey: priv,
		PublicKey:  pub,
		Address:    add,
		CreatedAt:  time.Now(),
	}

	if err := a.Encrypt(withPassword); err != nil {
		return nil, err
	}

	return a, nil
}
