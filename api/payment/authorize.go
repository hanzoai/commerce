package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe2"

	"crowdstart.io/models2/order"
	"crowdstart.io/util/json"
)

type AuthReq struct {
	Card stripe.Card
	// Paypal
	// Affirm
	Order *order.Order
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

	// Create new customer
	customer, err := client.NewCustomer(token.ID, &ar.Card, &ar.Order.Buyer)
	if err != nil {
		return nil, FailedToCreateCustomer
	}

	// Create charge
	charge, err := client.NewCharge(customer, ar.Order.Total, ar.Order.Currency)

	// Create payment

	return ar.Order, nil
}
