package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

var NonStripePayment = errors.New("Only refunds for Stripe payments are supported at the moment.")

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	if ord.Type != "stripe" {
		return NonStripePayment
	}

	// Try decode request body
	refundReq := new(RefundRequest)
	if err := json.Decode(c.Request.Body, &refundReq); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return FailedToDecodeRequestBody
	}

	return stripe.Refund(org, ord, currency.Cents(refundReq.amount))
}
