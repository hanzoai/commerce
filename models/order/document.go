package order

import (
	"strings"
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/country"
)

type Document struct {
	Id_    string
	UserId string

	ProductIds string

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

	Type string

	CreatedAt time.Time
	UpdatedAt time.Time

	Currency    string
	Total       float64
	CouponCodes string
	ReferrerId  string

	Status            string
	PaymentStatus     string
	FulfillmentStatus string
	Preorder          string
	Confirmed         string
}

func (d Document) Id() string {
	return d.Id_
}

func (o Order) Document() mixin.Document {
	preorder := "true"
	if !o.Preorder {
		preorder = "false"
	}
	confirmed := "true"
	if o.Unconfirmed {
		confirmed = "false"
	}

	productIds := make([]string, 0)
	for _, item := range o.Items {
		productIds = append(productIds, item.ProductId)
		productIds = append(productIds, item.ProductSlug)
	}

	return &Document{
		o.Id(),
		o.UserId,

		strings.Join(productIds, " "),

		o.BillingAddress.Line1,
		o.BillingAddress.Line2,
		o.BillingAddress.City,
		o.BillingAddress.State,
		o.BillingAddress.Country,
		country.ByISOCodeISO3166_2[o.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.BillingAddress.PostalCode,

		o.ShippingAddress.Line1,
		o.ShippingAddress.Line2,
		o.ShippingAddress.City,
		o.ShippingAddress.State,
		o.BillingAddress.Country,
		country.ByISOCodeISO3166_2[o.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.ShippingAddress.PostalCode,

		o.Type,

		o.CreatedAt,
		o.UpdatedAt,

		string(o.Currency),
		float64(o.Total),
		strings.Join(o.CouponCodes, " "),
		o.ReferrerId,

		string(o.Status),
		string(o.PaymentStatus),
		string(o.FulfillmentStatus),
		string(preorder),
		string(confirmed),
	}
}
