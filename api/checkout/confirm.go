package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/paypal"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
)

func confirm(c *gin.Context, org *organization.Organization, ord *order.Order) (err error) {
	// Handle payment confirmation
	switch ord.Type {
	case "paypal":
		err = paypal.Confirm(c, org, ord)
	default:
		return errors.New("Invalid order type")
	}

	return err
}
