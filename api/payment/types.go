package payment

import (
	"crowdstart.io/models/order"
	"crowdstart.io/models/payment"
	"crowdstart.io/models/user"
)

type AuthorizationReq struct {
	User_    *user.User       `json:"user"`
	Payment_ *payment.Payment `json:"payment"`
	Order    *order.Order     `json:"order"`
}

func (ar *AuthorizationReq) User() (*user.User, error) {
	usr := ar.User_
	usr.Model.Entity = ar.User_
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
	pay := ar.Payment_
	pay.Model.Entity = ar.Payment_
	pay.Model.Db = ar.Order.Db

	pay.Status = "unpaid"

	// Default all payment types to Stripe for now, although eventually we
	// should use organization settings
	pay.Type = payment.Stripe

	switch pay.Type {
	case payment.Stripe:
		return pay, nil
	default:
		return nil, UnsupportedPaymentType
	}
}
