package task

import (
	"math/big"

	"appengine"

	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"
)

var EthereumProcessPayment = delay.Func("ethereum-process-payment", func(ctx appengine.Context, walletNs, walletId, txHash, chainType string, amount *big.Int) {
	if err := EthereumProcessPaymentImpl(ctx, walletNs, walletId, txHash, chainType, amount); err != nil {
		panic(err)
	}
})

func EthereumProcessPaymentImpl(ctx appengine.Context, walletNs, walletId, txHash, chainType string, amount *big.Int) error {
	// Namespace the context
	nsCtx, err := appengine.Namespace(ctx, walletNs)
	if err != nil {
		log.Warn("Could not change to Namespace '%s': %v", walletNs, err, ctx)
		return err
	}

	// Namespace the datastore
	db := datastore.New(nsCtx)
	w := wallet.New(db)
	if err := w.GetById(walletId); err != nil {
		log.Warn("Could not find Wallet '%s': %v", walletId, err, ctx)
		return err
	}

	// Check if there's an order with this wallet
	ord := order.New(db)
	if ok, err := ord.Query().Filter("WalletId=", w.Id()).Get(); !ok {
		if err != nil {
			log.Warn("No order found for Wallet '%s': %v", w.Id(), err, ctx)
			return err
		}

		log.Warn("No order found for Wallet '%s'", w.Id(), ctx)
		return nil
	}

	// Make sure payment with TransactionHash does not exist (transaction
	// already processed)
	pay := payment.New(db)
	if ok, err := pay.Query().Filter("EthereumTransactionHash=", txHash).Get(); ok {
		log.Warn("Payment already created for Wallet '%s', TxHash '%s'", w.Id(), txHash, ctx)
		return nil
	} else if err != nil {
		log.Warn("No payment expected for Wallet '%s', TxHash '%s' but error encountered: %v", w.Id(), txHash, err, ctx)
		return err
	}

	// Get user so we can get a buyer
	usr := user.New(db)
	if err := usr.GetById(ord.UserId); err != nil {
		log.Warn("User not found for Order '%s', Wallet '%s'", ord.Id(), w.Id(), ctx)
		return err
	}

	// Create payment, update order
	if err := pay.RunInTransaction(func() error {
		pay.Account.EthereumTransactionHash = txHash
		pay.Account.EthereumChainType = blockchains.Type(chainType)
		pay.Account.WeiAmount = amount

		pay.Status = payment.Paid
		pay.Type = ord.Type
		pay.Buyer = usr.Buyer()
		pay.Currency = ord.Currency
		pay.Parent = ord.Key()
		pay.OrderId = ord.Id()
		pay.UserId = usr.Id()
		pay.Amount = pay.Currency.FromMinimalUnits(amount)

		if err := pay.Create(); err != nil {
			log.Warn("Could not save payment for Order '%s', Wallet '%s', TxHash '%s'", ord.Id(), w.Id(), txHash, ctx)
			return err
		}

		// Update order status
		if pay.Amount >= ord.Total {
			ord.PaymentStatus = payment.Paid
		}

		ord.Paid += pay.Amount

		if err := pay.Create(); err != nil {
			log.Warn("Could not save payment for Order '%s', Wallet '%s', TxHash '%s'", ord.Id(), w.Id(), txHash, ctx)
			return err
		}

		if err := ord.Update(); err != nil {
			log.Warn("Could not update Order '%s' for Wallet '%s', TxHash '%s'", ord.Id(), w.Id(), txHash, ctx)
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
