package shippingoptionrule

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type ShippingOptionRule struct {
	mixin.Model

	Attribute        string `json:"attribute"`
	Operator         string `json:"operator"` // "eq", "ne", "gt", "lt", "gte", "lte", "in", "nin"
	Value            string `json:"value" datastore:",noindex"`
	ShippingOptionId string `json:"shippingOptionId"`
}
