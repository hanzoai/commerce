package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe2"

	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/util/json"
)

type AuthReq struct {
	Card stripe.Card
	// Paypal
	// Affirm
	Order *order.Order
	Buyer models.Buyer
}

func authorize(c *gin.Context) (*order.Order, error) {
	// Get organization for this user
	org := middleware.GetOrg(c)
	ctx := org.Namespace(c)

	// Set up the db with the namespaced appengine context
	db := datastore.New(ctx)

	// Create AuthReq properly by calling order.New
	var ar AuthReq
	ar.Order = order.New(db)

	// In case people are using the version of the api that takes
	// existing orders Update order in request with id
	if id := c.Params.ByName("id"); id != "" {
		if err := ar.Order.Get(id); err != nil {
			return nil, OrderDoesNotExist
		}
	}

	// Try decode request body
	if err := json.Decode(c.Request.Body, &ar); err != nil {
		return nil, FailedToDecodeRequestBody
	}

	// Get client we can use for API calls
	client := stripe.New(ctx, org.Stripe.PublishableKey)

	// Do authorization
	token, err := client.Authorize(&ar.Card)
	if err != nil {
		return nil, AuthorizationFailed
	}

	// Create account
	account := models.PaymentAccount{}
	account.Buyer = ar.Buyer
	account.Type = "stripe"

	// Create Stripe customer, which we will attach to our payment account.
	customer, err := client.NewCustomer(token.ID, account.Buyer)
	if err != nil {
		return nil, FailedToCreateCustomer
	}
	account.Stripe.CustomerId = customer.ID

	payment := models.Payment{}
	payment.Status = "unpaid"
	payment.Account = account
	payment.Amount = ar.Order.Total
	payment.Currency = ar.Order.Currency

	// Fill with debug information about user's browser
	// payment.Client =

	// Create charge and associate with payment.
	charge, err := client.NewCharge(customer, ar.Order.Total, ar.Order.Currency)
	payment.ChargeId = charge.ID

	// Create payment
	ar.Order.Payments = append(ar.Order.Payments, payment)

	return ar.Order, nil
}
