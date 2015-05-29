package order

type Document struct {
	Id_    string
	UserId string

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
}

func (d Document) Id() string {
	return string(d.Id_)
}
