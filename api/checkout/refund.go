package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	// Try decode request body
	refundReq := new(RefundRequest)
	if err := json.Decode(c.Request.Body, &refundReq); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return FailedToDecodeRequestBody
	}

	return stripe.Refund(org, ord, currency.Cents(refundReq.Amount))
}
