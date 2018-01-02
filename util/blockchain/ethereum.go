package blockchain

import (
	"appengine"
	"errors"
	"fmt"
	"math/big"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
)

func MakeEthereumPayment(ctx appengine.Context, from wallet.Account, to string, amount, gasPrice *big.Int, password []byte) (string, error) {
	// Create needed client.

	client := ethereum.Client{}
	switch from.Type {
	case blockchains.EthereumType:
		client = ethereum.New(ctx, config.Ethereum.MainNetNodes[0])
		client.Chain = ethereum.MainNet
	case blockchains.EthereumRopstenType:
		client = ethereum.New(ctx, config.Ethereum.TestNetNodes[0])
		client.Chain = ethereum.Ropsten
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}
	// Decrypt private key if needed.
	var err error
	if from.Encrypted != "" && from.Salt != "" && from.PrivateKey == "" {
		err = from.Decrypt(password)
	}
	if err != nil {
		return "", err
	}
	return ethereum.MakePayment(client, from.PrivateKey, from.Address, to, amount, gasPrice, client.Chain)
}
