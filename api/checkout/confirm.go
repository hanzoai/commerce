package checkout

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/checkout/paypal"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
)

func confirm(c *zip.Ctx, org *organization.Organization, ord *order.Order) (err error) {
	// Handle payment confirmation
	switch ord.Type {
	case "paypal":
		err = paypal.Confirm(c, org, ord)
	default:
		return errors.New("Invalid order type")
	}

	return err
}
