package balance

import (
	"errors"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
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
