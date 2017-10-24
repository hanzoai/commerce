package tasks

import (
	"errors"
	"math/big"

	"appengine"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"
)

var SimpleTransactionGasUsed = big.NewInt(21000)
var UnsupportedChainType = errors.New("Chain type is unsupported.")
var PlatformWalletNotFound = errors.New("Platform Wallet Not Found.")
var PlatformAccountNotFound = errors.New("Platform Account Not Found.")

var EthereumProcessPayment = delay.Func(
	"ethereum-process-payment",
	func(
		ctx appengine.Context,
		orgName,
		walletId,
		txHash,
		chainType string,
		amount *big.Int,
	) {
		if err := EthereumProcessPaymentImpl(ctx, orgName, walletId, txHash, chainType, amount); err != nil {
			panic(err)
		}
	})

func EthereumProcessPaymentImpl(
	ctx appengine.Context,
	orgName,
	walletId,
	txHash,
	ct string,
	amount *big.Int,
) error {
	// convert chaintype
	chainType := blockchains.Type(ct)

	// Get Org
	db := datastore.New(ctx)
	org := organization.New(db)
	if err := org.GetById(orgName); err != nil {
		log.Warn("Could not find Org '%s': %v", orgName, err, ctx)
		return err
	}

	// Namespace the context
	nsCtx := org.Namespaced(ctx)

	// Namespace the datastore
	nsDb := datastore.New(nsCtx)

	w := wallet.New(nsDb)
	if err := w.GetById(walletId); err != nil {
		log.Warn("Could not find Wallet '%s': %v", walletId, err, ctx)
		return err
	}

	// Check if there's an order with this wallet
	ord := order.New(nsDb)
	if ok, err := ord.Query().Filter("WalletId=", w.Id()).Get(); !ok {
		if err != nil {
			log.Warn("No order found for Wallet '%s': %v", w.Id(), err, ctx)
			return err
		}

		log.Warn("No order found for Wallet '%s'", w.Id(), ctx)
		return nil
	}

	// Get user so we can get a buyer
	usr := user.New(nsDb)
	if err := usr.GetById(ord.UserId); err != nil {
		log.Warn("User not found for Order '%s', Wallet '%s'", ord.Id(), w.Id(), ctx)
		return err
	}

	// Create payment, update order
	pay := payment.New(nsDb)
	if err := pay.RunInTransaction(func() error {
		// Make sure payment with TransactionHash does not exist (transaction
		// already processed)
		if ok, err := pay.Query().Filter("Account.EthereumTransactionHash=", txHash).Get(); ok {
			log.Warn("Payment already created for Wallet '%s', TxHash '%s'", w.Id(), txHash, ctx)
			return nil
		} else if err != nil {
			log.Warn("No payment expected for Wallet '%s', TxHash '%s' but error encountered: %v", w.Id(), txHash, err, ctx)
			return err
		}

		pay.Account.EthereumTransactionHash = txHash
		pay.Account.EthereumChainType = chainType
		pay.Account.WeiAmount = blockchains.BigNumber(amount.String())

		log.Warn(ord.Currency)

		pay.Test = ord.Test
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
		if pay.Amount >= ord.Total && ord.PaymentStatus != ord.PaymentStatus {
			ord.PaymentStatus = payment.Paid

			// Fees
			platformFees, partnerFees := org.Pricing()
			fee, fes, err := ord.CalculateFees(platformFees, partnerFees)
			if err != nil {
				log.Error("Fee calculation error: %v", err, ctx)
				return err
			}
			fees := fes
			pay.Fee = fee

			// Create client for transfering fees
			address := ""
			password := ""
			var account *wallet.Account

			pw := wallet.New(org.Db)
			if ok, err := pw.Query().Filter("Id_=", "platform-wallet").Get(); !ok {
				if err != nil {
					return err
				}
				return PlatformWalletNotFound
			}

			switch chainType {
			case blockchains.EthereumType:
				address = config.Ethereum.MainNetNodes[0]
				password = config.Ethereum.DepositPassword
				if a, ok := pw.GetAccountByName("Ethereum Deposit Account"); ok {
					account = a
				} else {
					return PlatformAccountNotFound
				}
			case blockchains.EthereumRopstenType:
				address = config.Ethereum.TestNetNodes[0]
				password = config.Ethereum.TestPassword
				if a, ok := pw.GetAccountByName("Ethereum Ropsten Test Account"); ok {
					account = a
				} else {
					return PlatformAccountNotFound
				}
			default:
				return UnsupportedChainType
			}

			client := ethereum.New(ctx, address)

			// Get current gas price so we can calculate transfer costs
			gasPrice, err := client.GasPrice()
			if err != nil {
				log.Error("Could not get gas price: %v", err, ctx)
				return err
			}

			log.Warn(password, gasPrice, account)

			// Link payments/fees
			for _, fe := range fees {
				fe.PaymentId = pay.Id()
				pay.FeeIds = append(pay.FeeIds, fe.Id())
			}
		}

		ord.Paid += pay.Amount
		ord.PaymentIds = append(ord.PaymentIds, pay.Id())

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
