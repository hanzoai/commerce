package checkout

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/balance"
	"hanzo.io/api/checkout/bitcoin"
	"hanzo.io/api/checkout/ethereum"
	"hanzo.io/api/checkout/null"
	"hanzo.io/api/checkout/authorizenet"
	"hanzo.io/api/checkout/paypal"
	"hanzo.io/api/checkout/stripe"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/fee"
	"hanzo.io/models/multi"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/store"
	"hanzo.io/models/tokensale"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/log"
	"hanzo.io/util/reflect"
)

// Decode authorization request, grab user and payment information off it
func decodeAuthorization(c *gin.Context, ord *order.Order) (*user.User, *payment.Payment, *TokenSale, error) {
	a := new(Authorization)
	db := ord.Db

	// Decode request
	if err := json.Decode(c.Request.Body, a); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, nil, nil, FailedToDecodeRequestBody
	}

	log.JSON("Authorization:", a)

	// Copy request order into order used everywhere
	if a.Order != nil {
		reflect.Copy(a.Order, ord)
	}

	// Use provided order rather than initialize another order and break references
	a.Order = ord

	// Initialize and normalize models in authorization request
	if err := a.Init(db); err != nil {
		return nil, nil, nil, err
	}

	log.JSON("Order after initalization:", ord)

	return a.User, a.Payment, a.TokenSale, nil
}

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*payment.Payment, error) {
	var fees []*fee.Fee

	// Decode authorization request
	usr, pay, tsPass, err := decodeAuthorization(c, ord)
	if err != nil {
		log.Warn("Could not Decode '%v': '%v'", ord, err, c)
		return nil, err
	}

	log.Info("Decoded:", c)
	log.Info("User: '%v'", json.Encode(usr), c)
	log.Info("Payment: '%v'", json.Encode(pay), c)
	log.Info("Token Sale: '%v'", json.Encode(tsPass), c)

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, ok := c.Get("store")
	if ok {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
		log.Info("Using Store '%v'", stor.Id(), c)
	} else if ord.StoreId != "" {
		stor = store.New(ord.Db)
		if err := stor.GetById(ord.StoreId); err != nil {
			log.Warn("Store '%v' does not exist: %v", ord.StoreId, err, c)
			stor = nil
		}
		log.Info("Using Store '%v'", ord.StoreId, c)
	}

	log.Info("Order Before Tally: '%v'", json.Encode(ord), c)

	// Update order with information from datastore, and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error("Invalid or incomplete order error: %v", err, c)
		return nil, InvalidOrIncompleteOrder
	}

	log.Info("Order After Tally: '%v'", json.Encode(ord), c)

	// Validate token sale only if both password and id are set
	if (ord.TokenSaleId != "") && (tsPass != nil) {
		ts := tokensale.New(ord.Db)
		if err := ts.GetById(ord.TokenSaleId); err != nil {
			log.Error("Token sale not found error: %v", err, c)
			return nil, TokenSaleNotFound
		}

		// Create ethereum block chain wallets for funding
		w, err := usr.GetOrCreateWallet(usr.Db)
		if err != nil {
			log.Error("Wallet creation error: %v", err, c)
			return nil, WalletCreationError
		}

		_, err = w.CreateAccount("Account for Order "+ord.Id(), blockchains.EthereumType, []byte(tsPass.Passphrase))
		if err != nil {
			log.Error("Funding account creation error: %v", err, c)
			return nil, FundingAccountCreationError
		}
	} else if (ord.TokenSaleId != "") || (tsPass != nil) {
		log.Error("TokensSaleId '%s' or TokenSalePassphrase '%s' is missing", ord.TokenSaleId, tsPass, c)
		return nil, MissingTokenSaleOrPassphrase
	}

	// Override total if test email is used
	if org.IsTestEmail(usr.Email) {
		switch ord.Currency {
		case currency.BTC, currency.XBT:
			ord.Total = currency.Cents(1e5)
		case currency.ETH:
			ord.Total = currency.Cents(1e7)
		default:
			ord.Total = currency.Cents(50)
		}

		if pay != nil {
			pay.Test = true
		}
	}

	// Set test mode based on org live status
	if !org.Live {
		ord.Test = true
		if pay != nil {
			pay.Test = true
		}
	}

	usingFiat := payment.IsFiatProcessorType(ord.Type)

	// Ethereum payments are handles in the webhook
	if usingFiat {
		// Use updated order total
		pay.Amount = ord.Total

		// Capture client information to retain information about user at time of checkout
		pay.Client = client.New(c)

		// Calculate affiliate, partner and platform fees
		platformFees, partnerFees := org.Pricing()
		fee, fes, err := ord.CalculateFees(platformFees, partnerFees)
		if err != nil {
			log.Error("Fee calculation error: %v", err, c)
			return nil, FeeCalculationError
		}
		fees = fes
		pay.Fee = fee

		// Save payment Id on order
		ord.PaymentIds = append(ord.PaymentIds, pay.Id())
	}

	// Handle authorization
	switch ord.Type {
	case accounts.BalanceType:
		err = balance.Authorize(org, ord, usr, pay)
	case accounts.EthereumType:
		if ord.Currency != currency.ETH {
			return nil, UnsupportedEthereumCurrency
		}
		err = ethereum.Authorize(org, ord, usr)
	case accounts.BitcoinType:
		if ord.Currency != currency.BTC && ord.Currency != currency.XBT {
			return nil, UnsupportedBitcoinCurrency
		}
		err = bitcoin.Authorize(org, ord, usr)
	case accounts.NullType:
		err = null.Authorize(org, ord, usr, pay)
	case accounts.PayPalType:
		err = paypal.Authorize(org, ord, usr, pay)
	case accounts.AuthorizeNetType:
		if ord.Currency.IsCrypto() {
			return nil, UnsupportedStripeCurrency
		}
		err = authorizenet.Authorize(org, ord, usr, pay)
	case accounts.StripeType:
	default:
		if ord.Currency.IsCrypto() {
			return nil, UnsupportedStripeCurrency
		}
		if ord.Total > 500000 {
			return nil, TransactionLimitReached
		}
		err = stripe.Authorize(org, ord, usr, pay)
	}

	// Bail on authorization failure
	if err != nil {
		// Update payment status accordingly
		ord.Status = order.Cancelled
		if pay != nil {
			pay.Status = payment.Cancelled
			pay.Account.Error = err.Error()
			pay.MustCreate()
		}
		ord.MustCreate()
		usr.MustCreate()
		return nil, err
	}

	// Batch save user, order, payment, fees
	entities := []interface{}{usr, ord, pay}

	if !usingFiat {
		entities = []interface{}{usr, ord}
	} else {
		// If the charge is not live or test flag is set, then it is a test charge
		ord.Test = pay.Test || !pay.Live

		// Link payments/fees
		for _, fe := range fees {
			fe.PaymentId = pay.Id()
			pay.FeeIds = append(pay.FeeIds, fe.Id())
			entities = append(entities, fe)
		}
	}

	if usr.CreatedAt.IsZero() && !ord.Test {
		if err := counter.IncrUser(usr.Context(), time.Now()); err != nil {
			log.Error("IncrUser Error %v", err, c)
		}
	}

	multi.MustCreate(entities)

	return pay, nil
}
