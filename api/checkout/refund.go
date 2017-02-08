package checkout

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/stripe"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

func refund(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	type Refund struct {
		Amount currency.Cents `json:"amount"`
	}

	req := new(Refund)
	if err := json.Decode(c.Request.Body, req); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return FailedToDecodeRequestBody
	}

	log.JSON(req)

	return stripe.Refund(org, ord, req.Amount)
}
