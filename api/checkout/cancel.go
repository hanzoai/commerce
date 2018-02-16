package checkout

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/paypal"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
)

func cancel(c *context.Context, org *organization.Organization, ord *order.Order) (err error) {
	// Handle payment cancellation
	switch ord.Type {
	case "paypal":
		err = paypal.Cancel(c, org, ord)
	default:
		return errors.New("Invalid order type")
	}

	return err
}
