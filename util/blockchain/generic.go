package blockchain

import (
	"appengine"
	"errors"
	"fmt"

	"hanzo.io/models/blockchains"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/wallet"
)

func MakePayment(ctx appengine.Context, from wallet.Account, to string, amount, fee currency.Cents, password []byte) (string, error) {
	switch from.Type {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		return MakeEthereumPayment(ctx, from, to, currency.ETH.ToMinimalUnits(amount), currency.ETH.ToMinimalUnits(fee), password)
	case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
		return MakeBitcoinPayment(ctx, from, to, amount, fee, password)
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}
}
