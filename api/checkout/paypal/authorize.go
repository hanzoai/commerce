package paypal

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/paypal"
	"crowdstart.com/util/log"
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
