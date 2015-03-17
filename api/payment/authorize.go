package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe"

	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/util/json"
)

type AuthReq struct {
	Card stripe.Card
	// Paypal
	// Affirm
	Order *order.Order
}

func authorize(c *gin.Context) (o *order.Order, err error) {
	// Set up the db with the namespaced appengine context
	ctx := middleware.GetNamespace(c)
	db := datastore.New(ctx)

	// Create AuthReq properly by calling order.New
	var ar AuthReq
	ar.Order = order.New(db)

	// In case people are using the version of the api that takes
	// existing orders Update order in request with id
	id := c.Params.ByName("id")

	if id != "" {
		if err := ar.Order.Get(id); err != nil {
			return nil, OrderDoesNotExist
		}
	}

	// Try decode request body
	if err = json.Decode(c.Request.Body, &ar); err != nil {
		return nil, FailedToDecodeRequestBody
	}

	var token *stripe.Token
	if token, err = stripe.NewToken(&ar.Card, config.Stripe.APIKey); err != nil {
		return nil, AuthorizationFailed
	}

	org := middleware.GetOrg(c)

	var customer *stripe.Customer
	if customer, err = stripe.NewCustomer(ctx, org.Stripe.AccessToken, token.ID, &ar.Card, &ar.Order.Buyer); err != nil {
		return nil, FailedToCreateCustomer
	}

	p := models.PaymentAccount{}
	p.Name = ar.Card.Name
	p.Stripe.CustomerId = customer.ID
	//p.Stripe.ChargeId = charge.ID
	p.Stripe.CardType = string(token.Card.Brand)
	p.Stripe.Last4 = token.Card.LastFour
	p.Stripe.Expiration.Month = int(token.Card.Month)
	p.Stripe.Expiration.Year = int(token.Card.Year)

	return ar.Order, nil
}
