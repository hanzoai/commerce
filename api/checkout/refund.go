package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
)

var NonStripePayment = errors.New("Only refunds for Stripe payments are supported at the moment.")

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	if ord.Type != "stripe" {
		return NonStripePayment
	}
	return stripe.Refund(org, ord)
}
