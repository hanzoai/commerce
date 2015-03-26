package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment/stripe"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func authorizationRequest(c *gin.Context, ord *order.Order) (*AuthReq, error) {
	// Create AuthReq properly by calling order.New
	ar := new(AuthReq)
	ar.Order = ord

	// In case people are using the version of the api that takes existing
	// orders Update order in request with id
	if id := c.Params.ByName("id"); id != "" {
		if err := ar.Order.Get(id); err != nil {
			return nil, OrderDoesNotExist
		}
	}

	// Try decode request body
	if err := json.Decode(c.Request.Body, &ar); err != nil {
		return nil, FailedToDecodeRequestBody
	}

	return ar, nil
}

func authorize(c *gin.Context, org *organization.Organization, ord *order.Order) (*order.Order, error) {
	// Process authorization request
	ar, err := authorizationRequest(c, ord)
	if err != nil {
		return nil, err
	}

	// Peel off order for convience
	ord = ar.Order

	// Update order totals
	ord.Tally()
	log.Debug("Order: %#v", ord)

	// Get payment from source, update order
	pay, err := ar.Source.Payment(ord.Db)
	if err != nil {
		return nil, err
	}

	pay.Status = "unpaid"
	pay.Amount = ord.Total
	pay.Currency = ord.Currency

	// Get user from source, update order
	u, err := ar.Source.User(ord.Db)
	if err != nil {
		return nil, err
	}

	// Have stripe handle authorization
	if err := stripe.Authorize(org, ord, u, pay); err != nil {
		return nil, err
	}

	// Set user as parent of order
	ord.Parent = u.Key()
	ord.UserId = u.Id()

	// Create payment
	ord.PaymentIds = append(ord.PaymentIds, pay.Id())

	// Save order, payment and user!
	ord.Put()

	// Set order as parent of payment
	pay.Parent = ord.Key()
	pay.Put()

	// Save user
	u.Put()

	return ord, nil
}
