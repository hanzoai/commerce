package blockchain

import (
	"appengine"
	"math/big"

	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
)

func MakeEthereumPayment(ctx appengine.Context, client ethereum.Client, from wallet.Account, to string, amount *big.Int, password []byte) (string, error) {
	// Decrypt private key if needed.
	var err error
	if from.Encrypted != "" && from.Salt != "" && from.PrivateKey == "" {
		err = from.Decrypt(password)
	}
	if err != nil {
		return "", err
	}
	return ethereum.MakePayment(client, from.PrivateKey, from.Address, to, amount, client.Chain)
}
