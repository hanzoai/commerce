package ether

import (
	"crypto/ecdsa"
	"math/big"

	"hanzo.io/util/tokensale/ether/crypto"
)

func GenerateKeyPairFromBytes(pk []byte) (ecdsa.PrivateKey, ecdsa.PublicKey) {
	curve := crypto.S256()

	x, y := curve.ScalarBaseMult(pk)

	priv := ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(pk),
	}

	return priv, priv.PublicKey
}
