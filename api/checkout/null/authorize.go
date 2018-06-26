package null

import (
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/user"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	// Deprecated
	pay.Type = accounts.NullType

	pay.Account.Type = accounts.NullType
	pay.Live = true
	return nil
}
