package blockchain

import (
	"appengine"
	"errors"
	"fmt"
	"math/big"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/thirdparty/ethereum"
)

func MakePayment(ctx appengine.Context, from wallet.Account, to string, amount *big.Int, password []byte) (string, error) {
	switch from.Type {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
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
		return MakeEthereumPayment(ctx, client, from, to, amount, password)
	case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
		client := bitcoin.BitcoinClient{}
		switch from.Type {
		case blockchains.BitcoinType:
			client = bitcoin.New(ctx, config.Bitcoin.MainNetNodes[0], config.Bitcoin.MainNetUsernames[0], config.Bitcoin.MainNetPasswords[0])
		case blockchains.BitcoinTestnetType:
			client = bitcoin.New(ctx, config.Bitcoin.TestNetNodes[0], config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0])
		default:
			return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
		}
		return MakeBitcoinPayment(ctx, client, from, to, amount, password)
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}
}
