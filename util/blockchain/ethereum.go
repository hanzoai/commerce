package ethereum

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

func MakePayment(ctx appengine.Context, from wallet.Account, to string, amount *big.Int, typ blockchains.Type, password []byte) error {
	// Create needed client.

	client := ethereum.Client{}
	switch typ {
	case blockchains.EthereumType:
		client = ethereum.New(ctx, config.Ethereum.MainNetNodes[0])
		client.Chain = ethereum.MainNet
	case blockchains.EthereumRopstenType:
		client = ethereum.New(ctx, config.Ethereum.TestNetNodes[0])
		client.Chain = ethereum.Ropsten
	default:
		return errors.New(fmt.Sprintf("Unsupported blockchain type: %v", typ))
	}
	// Decrypt private key if needed.
	var err error
	if from.Encrypted != "" && from.Salt != "" && from.PrivateKey == "" {
		err = from.Decrypt(password)
	}
	if err != nil {
		return err
	}
	return ethereum.MakePayment(client, from.PrivateKey, from.Address, to, amount, client.Chain)
}
