package paypal

import (
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/paypal"
	"hanzo.io/util/log"
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
