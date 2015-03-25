package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/thirdparty/stripe2"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Get namespaced context off order
	ctx := ord.Db.Context

	// Create AuthReq properly by calling order.New
	ar := new(AuthReq)
	ar.Order = ord

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

	// Get user for order
	user, err := ar.Source.User(ar.Order.Db)
	if err != nil {
		return nil, err
	}

	// Set user as parent of order
	ar.Order.Parent = user.Key()
	ar.Order.UserId = user.Id()

	// Update order totals
	ar.Order.Tally()
	log.Debug("Order: %#v", ar.Order)

	// Create stripe client
	client := stripe.New(ctx, org.Stripe.AccessToken)

	err = newStripeCustomer(client, ar, user)
	if err != nil {
		return nil, err
	}

	// Save order!
	ar.Order.Put()
	user.Put()

	return ar.Order, nil
}
