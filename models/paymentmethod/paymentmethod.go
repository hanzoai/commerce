package paymentmethod

import (
	"hanzo.io/models/mixin"
	"hanzo.io/thirdparty/paymentmethods"
)

type PaymentMethod struct {
	mixin.Model
	paymentmethods.PaymentMethodOutput

	UserId string `json:"userId"`
}
