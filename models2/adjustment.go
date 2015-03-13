package models

type Adjustment struct {
	// Possible values: flat.
	Type string
	// Reasoning for price adjustment.
	Reason string
	// Authorizer of price adjustment.
	Issuer string
	// Amount of price adjustment.
	Amount Cents
}
