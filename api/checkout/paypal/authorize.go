package paypal

import (
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/paypal"
	"github.com/hanzoai/commerce/log"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	ctx := ord.Context()

	client := paypal.New(ctx)

	// Do authorization
	payKey, err := client.GetPayKey(pay, ord, org)
	if err != nil {
		log.Warn("Failed to authorize payment '%s'", pay.Id())
		log.JSON(pay)
		return err
	}

	pay.Account.PayKey = payKey
	usr.Accounts.PayPal.PayKey = payKey

	return nil
}
