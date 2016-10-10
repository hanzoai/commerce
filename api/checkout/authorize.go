package checkout

import (
	"errors"

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

func newAuthorization(c *gin.Context, ord *order.Order) (*Authorization, error) {
	a := new(Authorization)
	a.Order = ord

	// Try decode request body
	if err := json.Decode(c.Request.Body, a); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, FailedToDecodeRequestBody
	}

	return a, a.Init(ord.Db)
}

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*payment.Payment, error) {
	db := ord.Db
	ctx := db.Context

	// Process authorization request
	a, err := newAuthorization(c, ord)
	if err != nil {
		return nil, err
	}

	log.JSON("Authorization request:", a, c)

	// Grab payment and user off authorization
	usr := a.User
	pay := a.Payment

	// Check if store has been set, if so pull it out of the context
	var stor *store.Store
	v, ok := c.Get("store")
	if ok {
		stor = v.(*store.Store)
		ord.Currency = stor.Currency // Set currency
	}

	// Update order with information from datastore, and tally
	if err := ord.UpdateAndTally(stor); err != nil {
		log.Error(err, ctx)
		return nil, errors.New("Invalid or incomplete order")
	}

	// Override total to $0.50 is test email is used
	if org.IsTestEmail(pay.Buyer.Email) {
		ord.Total = currency.Cents(50)
		pay.Test = true
	}

	// Capture client information to retain information about user at time of checkout
	pay.Client = client.New(c)

	// Calculate affiliate, partner and platform fees
	fee, fees, err := ord.CalculateFees(org.Fees, org.Partners)
	pay.Fee = fee

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

	// Update payment status accordingly
	if err != nil {
		ord.Status = order.Cancelled
		pay.Status = payment.Cancelled
		pay.Account.Error = err.Error()

		return nil, err
	}

	// If the charge is not live or test flag is set, then it is a test charge
	ord.Test = pay.Test || !pay.Live

	// Batch save user, order, payment, fees
	entities := []interface{}{usr, ord, pay}

	// Link payments/fees
	for _, fe := range fees {
		fe.PaymentId = pay.Id()
		pay.FeeIds = append(pay.FeeIds, fe.Id())
		entities = append(entities, fe)
	}

	multi.MustCreate(entities)

	return pay, nil
}
