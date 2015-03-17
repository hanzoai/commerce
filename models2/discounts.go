package models

import "encoding/gob"

type Discount struct {
	// Possible values: flat, percent
	Type string `json:"type"`

	// Discount code applied.
	Code string `json:"code"`

	// Reasoning for price adjustment.
	Reason string `json:"reason"`

	// Authorizer of price adjustment.
	Issuer string `json:"issuer"`

	// Discount amount (500 == 5.00% off or 1000 == $10 off)
	Amount int `json:"amount"`

	// Products this applies to
	ProductIds []string `json:"productIds"`

	// Variants this applies to
	VariantIds []string `json:"variantIds"`

	// Whether to apply to all matching items, or just once.
	Once bool `json:"once"`
}

func init() {
	gob.Register(Discount{})
}
