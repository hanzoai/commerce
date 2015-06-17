package payment

import (
	"strings"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
)

type AuthorizationReq struct {
	User_    *user.User       `json:"user"`
	Payment_ *payment.Payment `json:"payment"`
	Order    *order.Order     `json:"order"`
}

func (ar *AuthorizationReq) User() (*user.User, error) {
	usr := ar.User_
	usr.Model = mixin.Model{Db: ar.Order.Db, Entity: usr}

	// If id is set, this is a pre-existing user, use data from datastore
	if usr.Id_ != "" {
		id := usr.Id_
		usr = user.New(usr.Model.Db)
		if err := usr.Get(id); err != nil {
			return nil, UserDoesNotExist
		} else {
			return usr, nil
		}
	}

	// See if order has address if we don't.
	if usr.ShippingAddress.Empty() {
		usr.ShippingAddress = ar.Order.ShippingAddress
	}

	if usr.BillingAddress.Empty() {
		usr.BillingAddress = ar.Order.BillingAddress
	}

	// Normalize a few things we get in
	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))
	usr.Username = strings.ToLower(strings.TrimSpace(usr.Username))

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
