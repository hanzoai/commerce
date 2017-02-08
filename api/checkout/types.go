package checkout

import (
	"strings"

	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
)

type AuthorizationReq struct {
	User_    *user.User       `json:"user"`
	Payment_ *payment.Payment `json:"payment"`
	Order    *order.Order     `json:"order"`
}

func (ar *AuthorizationReq) User() (*user.User, error) {
	// Pull user id off request
	id := ar.User_.Id_

	// If id is set, this is a pre-existing user, use data from datastore
	if id != "" {
		ar.User_ = user.New(ar.Order.Db)
		if err := ar.User_.Get(id); err != nil {
			// Try to load using email
			email := ar.User_.Email
			if email != "" {
				if err := ar.User_.GetByEmail(email); err != nil {
					return nil, UserDoesNotExist
				}
			} else {
				return nil, UserDoesNotExist
			}
		} else {
			return ar.User_, nil
		}
	}

	// Ensure model mixin is setup correctly
	ar.User_.Init(ar.Order.Db)

	// See if order has address if we don't.
	if ar.User_.ShippingAddress.Empty() {
		ar.User_.ShippingAddress = ar.Order.ShippingAddress
	}

	if ar.User_.BillingAddress.Empty() {
		ar.User_.BillingAddress = ar.Order.BillingAddress
	}

	// Normalize a few things we get in
	ar.User_.Email = strings.ToLower(strings.TrimSpace(ar.User_.Email))
	ar.User_.Username = strings.ToLower(strings.TrimSpace(ar.User_.Username))

	return ar.User_, nil
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

type RefundRequest struct {
	Amount uint64
}
