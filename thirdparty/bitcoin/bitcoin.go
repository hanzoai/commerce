package bitcoin

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"golang.org/x/crypto/ripemd160"

	"encoding/hex"
	"github.com/btcsuite/btcutil/base58"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
)

// The steps notated in the variable names here relate to the steps outlined in
// https://en.bitcoin.it/wiki/Technical_background_of_version_1_Bitcoin_addresses
func PubKeyToAddress(pubKey []byte, netId byte) ([]byte, string) {
	sha := sha256.New()
	ripe := ripemd160.New()
	step2 := sha.Sum(pubKey)

	ripe.Write(step2)

	step3 := ripe.Sum(nil)

	step4 := append([]byte{netId}, step3...)

	step5 := sha.Sum(step4)

	step6 := sha.Sum(step5)

	step7 := step6[0:4]

	step8 := append(step7, step4...)

	return step8, base58.Encode(step8)
}

func GenerateKeyPair() (string, string, error) {
	priv, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Remove the extra pubkey byte before serializing hex (drop the first 0x04)
	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)), nil
}
