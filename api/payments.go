package api

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

func charge(c *gin.Context) {
	authorize(c)
	capture(c)
}

type AuthReq struct {
	Card stripe.Card
	//Paypal
	//Affirm
	Order *order.Order
}

func authorize(c *gin.Context) {
	var err error
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
			json.Fail(c, 500, "Order does not exist.", err)
			return
		}
	}

	// Try decode request body
	if err = json.Decode(c.Request.Body, &ar); err != nil {
		json.Fail(c, 500, "Failed to decode request body.", err)
		return
	}

	var token *stripe.Token
	if token, err = stripe.NewToken(&ar.Card, config.Stripe.APIKey); err != nil {
		ctx.Errorf("[Api.Payment.Stripe] %v", err)
		json.Fail(c, 500, "Could not authorize.", err)
		return
	}

	org := middleware.GetOrg(c)

	var customer *stripe.Customer
	if customer, err = stripe.NewCustomer(ctx, org.Stripe.AccessToken, token.ID, &ar.Card, &ar.Order.Buyer); err != nil {
		ctx.Errorf("[Api.Payment.Stripe] %v", err)
		json.Fail(c, 500, "Could not create customer.", err)
		return
	}

	p := models.PaymentAccount{}
	p.Name = ar.Card.Name
	p.Stripe.CustomerId = customer.ID
	//p.Stripe.ChargeId = charge.ID
	p.Stripe.CardType = string(token.Card.Brand)
	p.Stripe.Last4 = token.Card.LastFour
	p.Stripe.Expiration.Month = int(token.Card.Month)
	p.Stripe.Expiration.Year = int(token.Card.Year)
}

func capture(c *gin.Context) {

}
