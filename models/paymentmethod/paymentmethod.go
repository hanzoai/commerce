package paymentmethod

import (
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/thirdparty/paymentmethods"
)

type PaymentMethod struct {
	mixin.Model
	paymentmethods.PaymentMethodOutput

	UserId string `json:"userId"`
	Name   string `json:"name"`
}
