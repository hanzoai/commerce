package blockchain

import (
	"appengine"
	"errors"
	"fmt"

	"hanzo.io/config"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

func MakeBitcoinPayment(ctx context.Context, from wallet.Account, to string, amount, feePerByte currency.Cents, password []byte) (string, error) {
	// Create needed client.

	client := bitcoin.BitcoinClient{}
	switch from.Type {
	case blockchains.BitcoinType:
		client = bitcoin.New(ctx, config.Bitcoin.MainNetNodes[0], config.Bitcoin.MainNetUsernames[0], config.Bitcoin.MainNetPasswords[0])
	case blockchains.BitcoinTestnetType:
		client = bitcoin.New(ctx, config.Bitcoin.TestNetNodes[0], config.Bitcoin.TestNetUsernames[0], config.Bitcoin.TestNetPasswords[0])
	default:
		return "", errors.New(fmt.Sprintf("Unsupported blockchain type: %v", from.Type))
	}

	oris, err := bitcoin.GetBitcoinTransactions(ctx, from.Address)
	if err != nil {
		log.Info("Address '%s' Transaction: %v", from.Address, json.Encode(oris), ctx)
		return "", err
	}

	total := int64(amount)

	prunedOris, err := bitcoin.PruneOriginsWithAmount(oris, total)
	if err != nil {
		log.Info("Address '%s' Transaction: %v", from.Address, json.Encode(prunedOris), ctx)
		return "", err
	}

	in := bitcoin.OriginsWithAmountToOrigins(prunedOris)
	out := []bitcoin.Destination{
		bitcoin.Destination{
			Value:   total,
			Address: to,
		},
	}

	// Decrypt private key if needed.
	if from.Encrypted != "" && from.Salt != "" && from.PrivateKey == "" {
		err = from.Decrypt(password)
	}

	rawTrx, err := bitcoin.CreateTransaction(client, in, out, bitcoin.Sender{
		PrivateKey: from.PrivateKey,
		PublicKey:  from.PublicKey,
		Address:    from.Address,
	}, int64(feePerByte))

	return client.SendRawTransaction(rawTrx)
}
