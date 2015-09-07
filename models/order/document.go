package order

import "time"

type Document struct {
	IntId  string //not int64 support so encode as string to avoid rounding
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

	Created   time.Time
	UpdatedAt time.Time

	Currency    string
	Total       float64
	CouponCodes string
	ReferrerId  string

	Status             string
	PaymentStatus      string
	FullfillmentStatus string
	Preorder           string
	Confirmed          string
}

func (d Document) Id() string {
	return string(d.Id_)
}
