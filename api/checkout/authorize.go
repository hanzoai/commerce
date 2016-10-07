package checkout

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/balance"
	"crowdstart.com/api/checkout/null"
	"crowdstart.com/api/checkout/paypal"
	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

func authorizationRequest(c *gin.Context, ord *order.Order) (*AuthorizationReq, error) {
	// Create AuthReq properly by calling order.New
	ar := new(AuthorizationReq)
	ar.Order = ord

	// Try decode request body
	if err := json.Decode(c.Request.Body, &ar); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, FailedToDecodeRequestBody
	}

	// This is kind of terrible to do here but oh well...
	if ar.Order.ShippingAddress.Empty() {
		ar.Order.ShippingAddress = ar.User_.ShippingAddress
	}

	if ar.Order.BillingAddress.Empty() {
		ar.Order.BillingAddress = ar.User_.BillingAddress
	}

	return ar, nil
}

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*payment.Payment, error) {
	// Process authorization request
	ar, err := authorizationRequest(c, ord)
	if err != nil {
		return nil, err
	}

	log.JSON("Authorization Request:", ar, c)

	// Peel off order for convience
	ord = ar.Order
	db := ord.Db
	ctx := db.Context

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, ok := c.Get("store")
	if ok {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
	}

	log.JSON("BEFORE TALLY '%s'", ord.Id(), ord, c)

	// Update order with information from datastore, store and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error(err, ctx)
		return nil, errors.New("Invalid or incomplete order")
	}

	log.JSON("AFTER TALLY '%s'", ord.Id(), ord, c)

	// Get user from request
	usr, err := ar.User()
	if err != nil {
		return nil, err
	}

	log.JSON("User '%s'", usr.Id(), usr, c)

	// Get payment from request, update order
	pay, err := ar.Payment()
	if err != nil {
		return nil, err
	}

	// Use user as buyer
	pay.Buyer = usr.Buyer()

	log.JSON("Buyer '%s'", pay.Buyer.Email, pay.Buyer, c)

	// Override total to $0.50 is test email is used
	if org.IsTestEmail(pay.Buyer.Email) {
		ord.Total = currency.Cents(50)
		pay.Test = true
	}

	// Capture client information to retain information about user at time of checkout
	pay.Client = client.New(c)

	// Update payment with order information
	pay.Amount = ord.Total

	// Calculate affiliate, partner and platform fees
	fee, fees, err := ord.CalculateFees(org.Fees, org.Partners)
	pay.Currency = ord.Currency
	pay.Description = ord.Description()
	pay.Fee = fee

	log.JSON("Payment '%s'", pay.Id(), pay, c)

	// Setup all relationships before we try to authorize to ensure that keys
	// that get created are actually valid.

	// User -> order
	ord.Parent = usr.Key()
	ord.UserId = usr.Id()

	// Order -> payment
	pay.Parent = ord.Key()
	pay.OrderId = ord.Id()

	// Save payment Id on order
	ord.PaymentIds = append(ord.PaymentIds, pay.Id())

	// Handle authorization
	switch ord.Type {
	case "null":
		err = null.Authorize(org, ord, usr, pay)
	case "balance":
		err = balance.Authorize(org, ord, usr, pay)
	case "paypal":
		err = paypal.Authorize(org, ord, usr, pay)
	case "stripe":
		err = stripe.Authorize(org, ord, usr, pay)
	default:
		err = stripe.Authorize(org, ord, usr, pay)
	}

	if err != nil {
		ord.Status = order.Cancelled
		pay.Status = payment.Cancelled
		pay.Account.Error = err.Error()

		return nil, err
	}

	// If the charge is not live or test flag is set, then it is a test charge
	ord.Test = pay.Test || !pay.Live

	ord.BillingAddress.Country = strings.ToUpper(ord.BillingAddress.Country)
	ord.ShippingAddress.Country = strings.ToUpper(ord.ShippingAddress.Country)

	// Batch save user, order, payment, fees
	entities := []interface{}{usr, ord, pay}

	// Link payments/fees
	for _, fe := range fees {
		fe.PaymentId = pay.Id()
		pay.FeeIds = append(pay.FeeIds, fe.Id())
		entities = append(entities, fe)
	}

	log.Warn("Fees: %v", fees)

	multi.MustCreate(entities)

	log.Debug("Order '%s' authorized", ord.Id(), c)

	return pay, nil
}
