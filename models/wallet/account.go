package wallet

import (
	"time"

	"hanzo.io/models/blockchains"

	"hanzo.io/util/crypto/aes"
	"hanzo.io/util/rand"
)

type Account struct {
	Encrypted string `json:"encrypted"`
	Salt      string `json:"salt"`

	Name           string `json:"name"`
	PrivateKey     string `json:"-" datastore:"-"`
	PublicKey      string `json:"-"`
	Address        string `json:"address,omitempty"`
	TestNetAddress string `json:"testnetaddress,omitempty"`

	Deleted bool             `json:"-"`
	Type    blockchains.Type `json:"type"`

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

func (a *Account) Delete() {
	a.Deleted = true
}
