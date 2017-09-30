package ether

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"

	"hanzo.io/util/tokensale/ether/crypto"
)

func GenerateKeyPair() (string, string, string, error) {
	priv, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}

	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)), PubkeyToAddress(priv.PublicKey), nil
}

func PubkeyToAddress(p ecdsa.PublicKey) string {
	return crypto.PubkeyToAddress(p).Hex()
}
