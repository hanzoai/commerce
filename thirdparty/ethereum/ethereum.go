package ethereum

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"math/big"

	"hanzo.io/thirdparty/ethereum/go-ethereum/common"
	"hanzo.io/thirdparty/ethereum/go-ethereum/core/types"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/thirdparty/ethereum/go-ethereum/rlp"
	r "hanzo.io/util/rand"
)

type ChainId int64

const (
	MainNet ChainId = 1
	Morden  ChainId = 2
	Ropsten ChainId = 3
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

// Create a signed transaction and return the hex string as input to "eth_sendRawTransaction"
func NewSignedTransaction(chainId ChainId, pk, to string, amount, gasLimit, gasPrice int64, data []byte) (string, error) {
	// Decode the private key
	privKey, err := crypto.HexToECDSA(pk)
	if err != nil {
		return "", err
	}

	// Create a signer for the particular chain using the modern signature
	// algorithm
	signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
	tx := types.NewTransaction(uint64(r.Int64()), common.StringToAddress(to), big.NewInt(amount), big.NewInt(gasLimit), big.NewInt(gasPrice), data)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return "", err
	}

	// get RLP of transaction
	bytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", err
	}

	// return hex of rlp
	return common.ToHex(bytes), nil
}
