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

	// Get user from source
	usr, err := ar.Source.User(ord.Db)
	if err != nil {
		return nil, err
	}

	// Get payment from source, update order
	pay, err := ar.Source.Payment(ord.Db)
	if err != nil {
		return nil, err
	}

	// Fill with debug information about user's browser
	pay.Client.Ip = c.Request.RemoteAddr
	pay.Client.UserAgent = c.Request.UserAgent()
	pay.Client.Language = c.Request.Header.Get("Accept-Language")
	pay.Client.Referer = c.Request.Referer()

	// Update payment with order information
	pay.Amount = ord.Total
	pay.Currency = ord.Currency

	// Have stripe handle authorization
	if err := stripe.Authorize(org, ord, usr, pay); err != nil {
		return nil, err
	}

	// User -> order
	ord.Parent = usr.Key()
	ord.UserId = usr.Id()

	// Order -> payment
	pay.Parent = ord.Key()
	pay.OrderId = ord.Id()

	// Save payment Id on order
	ord.PaymentIds = append(ord.PaymentIds, pay.Id())

	// Save user, order, payment
	usr.Put()
	ord.Put()
	pay.Put()

	return ord, nil
}
