package checkout

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
)

var NonStripePayment = errors.New("Only refunds for Stripe payments are supported at the moment.")

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	if ord.Type != "stripe" {
		return NonStripePayment
	}
	rawAmount := c.DefaultQuery("amount", "")
	if rawAmount == "" {
		return stripe.Refund(org, ord, currency.Cents(ord.Total))
	} else {
		refundAmount, err := strconv.ParseUint(rawAmount, 10, 64)
		if err != nil {
			return err
		}
		return stripe.Refund(org, ord, currency.Cents(refundAmount))
	}
}
