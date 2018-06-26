package balance

import (
	"errors"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/user"
)

var InsufficientCredit = errors.New("Insufficient credit")

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	// Deprecated
	pay.Type = accounts.BalanceType

	pay.Account.Type = accounts.BalanceType
	pay.Live = org.Live

	if err := usr.CalculateBalances(!org.Live); err != nil {
		return err
	}

	if val, ok := usr.Transactions[ord.Currency]; !ok || val.Balance < ord.Total {
		return InsufficientCredit
	}

	return nil
}
