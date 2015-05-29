package user

import "appengine/search"

type Document struct {
	Id_       string
	Email     search.Atom
	Username  string
	FirstName string
	LastName  string
	Phone     string

	BillingAddressLine1      string
	BillingAddressLine2      string
	BillingAddressCity       string
	BillingAddressState      string
	BillingAddressCountry    string
	BillingAddressPostalCode string

	ShippingAddressLine1      string
	ShippingAddressLine2      string
	ShippingAddressCity       string
	ShippingAddressState      string
	ShippingAddressCountry    string
	ShippingAddressPostalCode string

	StripeBalanceTransactionId string
	StripeCardId               string
	StripeChargeId             string
	StripeCustomerId           string
	StripeLastFour             string
}

func (d Document) Id() string {
	return string(d.Id_)
}
