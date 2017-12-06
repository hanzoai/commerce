package blockchain

import (
	"appengine"
	"errors"
	"fmt"
	"math/big"

	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
)

func MakePayment(ctx appengine.Context, from wallet.Account, to string, amount *big.Int, password []byte) (string, error) {
	switch from.Type {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		return MakeEthereumPayment(ctx, from, to, amount, password)
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}
}
