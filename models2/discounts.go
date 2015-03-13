package models

type Discount struct {
	// Possible values: flat, percent
	Type string

	// Discount code applied.
	Code string

	// Reasoning for price adjustment.
	Reason string

	// Authorizer of price adjustment.
	Issuer string

	// Discount amount (500 == 5.00% off or 1000 == $10 off)
	Amount int

	// Product this applies to
	ProductId string

	// Whether to apply to all matching items, or just once.
	Once bool
}
