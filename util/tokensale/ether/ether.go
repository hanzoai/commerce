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

	// Remove the extra pubkey byte before serializing hex (drop the first 0x04)
	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)[1:]), PubkeyToAddress(priv.PublicKey), nil
}

func PubkeyToAddress(p ecdsa.PublicKey) string {
	// Remove the '0x' from the address
	return crypto.PubkeyToAddress(p).Hex()[2:]
}
