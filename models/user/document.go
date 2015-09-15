package user

import (
	"time"

	"appengine/search"
)

type Document struct {
	Id_               string
	Email             search.Atom
	EmailPartials     string
	Username          string
	UsernamePartials  string
	FirstName         string
	FirstNamePartials string
	LastName          string
	LastNamePartials  string
	Phone             string

	BillingAddressLine1       string
	BillingAddressLine2       string
	BillingAddressCity        string
	BillingAddressState       string
	BillingAddressCountryCode string
	BillingAddressCountry     string
	BillingAddressPostalCode  string

	ShippingAddressLine1       string
	ShippingAddressLine2       string
	ShippingAddressCity        string
	ShippingAddressState       string
	ShippingAddressCountryCode string
	ShippingAddressCountry     string
	ShippingAddressPostalCode  string

	CreatedAt time.Time
	UpdatedAt time.Time

	StripeBalanceTransactionId string
	StripeCardId               string
	StripeChargeId             string
	StripeCustomerId           string
	StripeLastFour             string
}

func (d Document) Id() string {
	return string(d.Id_)
}
