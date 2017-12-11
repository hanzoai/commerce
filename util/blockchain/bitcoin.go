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
)

func MakeBitcoinPayment(ctx appengine.Context, from wallet.Account, to string, amount *big.Int, password []byte) (string, error) {
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
	amountAvailable := big.NewInt(0)
	transactionsToUse := make([]bitcoin.Origin, 0)
	addressToAudit := ""
	if from.Type == blockchains.BitcoinType {
		addressToAudit = from.Address
	} else {
		addressToAudit = from.TestNetAddress
	}

	transactions, _ := bitcoin.GetBitcoinTransactions(ctx, addressToAudit)
	for _, transaction := range transactions {
		if transaction.Amount > int64(bitcoin.CalculateFee(1, 0)) { // do not include transactions worth so little it will cost more to send them
			amountAvailable.Add(amountAvailable, big.NewInt(transaction.Amount))
			transactionsToUse = append(transactionsToUse, transaction.Origin)
		}
		if amountAvailable.Cmp(amount) > -1 { // break out if we have enough money
			break
		}
	}

	// Decrypt private key if needed.
	var err error
	if from.Encrypted != "" && from.Salt != "" && from.PrivateKey == "" {
		err = from.Decrypt(password)
	}
	if err != nil {
		return "", err
	}

	destination := []bitcoin.Destination{bitcoin.Destination{Value: amount.Int64(), Address: to}}
	sender := bitcoin.Sender{PrivateKey: from.PrivateKey, PublicKey: from.PublicKey, Address: from.Address}
	ret, err := bitcoin.CreateTransaction(client, transactionsToUse, destination, sender)
	return string(ret), err
}
