package ethereum

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"

	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
)

type ChainId int64

const (
	MainNet ChainId = 1
	Morden  ChainId = 2
	Ropsten ChainId = 3
)

const (
	DefaultGas      int64 = 90000
	DefaultGasPrice int64 = 10 * Shannon
)

func GenerateKeyPair() (string, string, string, error) {
	priv, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}

	// Remove the extra pubkey byte before serializing hex (drop the first 0x04)
	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)[1:]), PubkeyToAddress(priv.PublicKey), nil
}

func PubkeyToAddress(p ecdsa.PublicKey) string {
	// Remove the '0x' from the address
	return crypto.PubkeyToAddress(p).Hex()
}
