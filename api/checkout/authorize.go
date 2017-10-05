package checkout

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/balance"
	"hanzo.io/api/checkout/ethereum"
	"hanzo.io/api/checkout/null"
	"hanzo.io/api/checkout/paypal"
	"hanzo.io/api/checkout/stripe"
	"hanzo.io/models/multi"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/store"
	"hanzo.io/models/tokensale"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
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
	// Decode authorization request
	usr, pay, tsPass, err := decodeAuthorization(c, ord)
	if err != nil {
		return nil, err
	}

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, ok := c.Get("store")
	if ok {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
	}

	// Update order with information from datastore, and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error("Invalid or incomplete order error: %v", err, c)
		return nil, InvalidOrIncompleteOrder
	}

	// Validate token sale
	if (ord.TokenSaleId != "") == (tsPass != nil) {
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

		_, err = w.CreateAccount(wallet.Ethereum, []byte(tsPass.Passphrase))
		if err != nil {
			log.Error("Funding account creation error: %v", err, c)
			return nil, FundingAccountCreationError
		}
	}

	// Override total to $0.50 is test email is used
	if org.IsTestEmail(pay.Buyer.Email) {
		ord.Total = currency.Cents(50)
		pay.Test = true
	}

	// Use updated order total
	pay.Amount = ord.Total

	// Capture client information to retain information about user at time of checkout
	pay.Client = client.New(c)

	// Calculate affiliate, partner and platform fees
	platformFees, partnerFees := org.Pricing()
	fee, fees, err := ord.CalculateFees(platformFees, partnerFees)
	pay.Fee = fee

	// Save payment Id on order
	ord.PaymentIds = append(ord.PaymentIds, pay.Id())

	// Handle authorization
	switch ord.Type {
	case payment.Balance:
		err = balance.Authorize(org, ord, usr, pay)
	case payment.Ethereum:
		if org.Currency != currency.ETH {
			return nil, UnsupportedEthereumCurrency
		}
		err = ethereum.Authorize(org, ord, usr)
	case payment.Null:
		err = null.Authorize(org, ord, usr, pay)
	case payment.PayPal:
		err = paypal.Authorize(org, ord, usr, pay)
	case payment.Stripe:
		if org.Currency.IsCrypto() {
			return nil, UnsupportedStripeCurrency
		}
		err = stripe.Authorize(org, ord, usr, pay)
	default:
		err = stripe.Authorize(org, ord, usr, pay)
	}

	// Bail on authorization failure
	if err != nil {
		// Update payment status accordingly
		ord.Status = order.Cancelled
		pay.Status = payment.Cancelled
		pay.Account.Error = err.Error()
		return nil, err
	}

	// If the charge is not live or test flag is set, then it is a test charge
	ord.Test = pay.Test || !pay.Live

	// Batch save user, order, payment, fees
	entities := []interface{}{usr, ord, pay}

	if ord.Type == payment.Ethereum {
		entities = []interface{}{usr, ord}
	} else {
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
