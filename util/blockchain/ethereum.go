package ethereum

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/util/log"
)

func MakePayment(ctx appengine.Context, from wallet.Account, to wallet.Account, amount big.Int, typ blockchains.Type) error {
	// Create needed client.

	client := ethereum.Client{}
	switch typ {
	case blockchains.EthereumType:
		client = ethereum.New(ctx, config.Ethereum.MainNetNodes[0])
		client.Chain = ethereum.MainNet.ChainId
	case blockchains.EthereumRopstenType:
		client = ethereum.New(ctx, config.Ethereum.TestNetNodes[0])
		client.Chain = ethereum.Ropsten.ChainId
	default:
		return errors.New(fmt.Sprintf("Unsupported blockchain type: %v", typ))
	}
	return ethereum.MakePayment(client, from.PrivateKey, from.Address, to.Address, client.Chain)
}
