package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/paypal"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
)

func cancel(c *gin.Context, org *organization.Organization, ord *order.Order) (err error) {
	// Handle payment cancellation
	switch ord.Type {
	case "paypal":
		err = paypal.Cancel(c, org, ord)
	default:
		return errors.New("Invalid order type")
	}

	return err
}
