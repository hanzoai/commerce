package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/api/checkout/paypal"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
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
