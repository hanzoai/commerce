package payment

import (
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
)

type AuthorizationReq struct {
	Type payment.Type `json:"type"`

	Buyer   *user.User      `json:"buyer"`
	Account payment.Account `json:"payment"`
	Order   *order.Order    `json:"order"`
}

func (ar *AuthorizationReq) User() (*user.User, error) {
	usr := ar.Buyer
	usr.Model.Entity = ar.Buyer
	usr.Model.Db = ar.Order.Db

	// If Id is set, this is a pre-existing user, user copy from datastore
	if usr.Id_ != "" {
		if err := usr.Get(usr.Id_); err != nil {
			return nil, UserDoesNotExist
		}
	}

	return usr, nil
}

func (ar *AuthorizationReq) Payment() (*payment.Payment, error) {
	pay := payment.New(ar.Order.Db)

	switch ar.Type {
	case payment.Stripe:
		pay.Type = payment.Stripe
		pay.Account = ar.Account
		pay.Status = "unpaid"
		return pay, nil
	default:
		return nil, UnsupportedPaymentType
	}
}
