package blockchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/wallet"
)

func MakePayment(ctx context.Context, from wallet.Account, to string, amount, fee currency.Cents, password []byte) (string, error) {
	switch from.Type {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		return MakeEthereumPayment(ctx, from, to, currency.ETH.ToMinimalUnits(amount), currency.ETH.ToMinimalUnits(fee), password)
	case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
		return MakeBitcoinPayment(ctx, from, to, amount, fee, password)
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}
}
