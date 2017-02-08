package balance

import (
	"errors"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
)

var InsufficientCredit = errors.New("Insufficient credit")

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	pay.Type = payment.Balance
	pay.Live = true

	if err := usr.CalculateBalances(); err != nil {
		return err
	}

	if usr.Balances[ord.Currency] < ord.Total {
		return InsufficientCredit
	}

	return nil
}
