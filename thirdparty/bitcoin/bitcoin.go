package bitcoin

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/ripemd160"

	"github.com/btcsuite/btcutil/base58"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/util/log"
)

// The steps notated in the variable names here relate to the steps outlined in
// https://en.bitcoin.it/wiki/Technical_background_of_version_1_Bitcoin_addresses
func PubKeyToAddress(pubKey string, testNet bool) (string, byte[], error) {
	ripe := ripemd160.New()
	step2decode, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", "", err
	}
	step2 := sha256.Sum256(step2decode)

	log.Debug("public key: %v", pubKey)
	log.Debug("public key hex decode: %v", step2decode)
	log.Debug("Step 2 hex: %v", hex.EncodeToString(step2[:]))
	if len(step2) != 32 {
		return "", "", fmt.Debugf("Step 2: Invalid length. %v", len(step2))
	}

	log.Debug("Step 2: %v", step2)
	ripe.Write(step2[:])
	step3 := ripe.Sum(nil)

	log.Debug("Step 3 hex: %v", hex.EncodeToString(step3))
	if len(step3) != 20 {
		return "", "", fmt.Debugf("Step 3: Invalid length. %v", len(step3))
	}

	step4 := append([]byte{byte(0)}, step3...)

	log.Debug("Step 4 hex: %v", hex.EncodeToString(step4))
	if len(step4) != 21 {
		return "", "", fmt.Debugf("Step 4: Invalid length. %v", len(step4))
	}

	step5 := sha256.Sum256(step4)

	log.Debug("Step 5 hex: %v", hex.EncodeToString(step5[:]))
	if len(step5) != 32 {
		return "", "", fmt.Debugf("Step 5: Invalid length. %v", len(step5))
	}

	step6 := sha256.Sum256(step5[:])

	log.Debug("Step 6 hex: %v", hex.EncodeToString(step6[:]))
	if len(step6) != 32 {
		return "", "", fmt.Debugf("Step 6: Invalid length. %v", len(step6))
	}
	step7 := step6[0:4]

	log.Debug("Step 7 hex: %v", hex.EncodeToString(step7[:]))
	if len(step7) != 4 {
		return "", "", fmt.Debugf("Step 7: Invalid length. %v", len(step7))
	}
	step8 := append(step4, step7...)

	log.Debug("Step 8 hex: %v", hex.EncodeToString(step8[:]))
	log.Debug("Step 8 Base58 encode: %v", base58.Encode(step8))
	if len(step8) != 25 {
		return "", "", fmt.Debugf("Step 8: Invalid length. %v", len(step8))
	}

	return base58.Encode(step8), step8, nil
}

func GenerateKeyPair() (string, string, error) {
	priv, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Remove the extra pubkey byte before serializing hex (drop the first 0x04)
	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)), nil
}
