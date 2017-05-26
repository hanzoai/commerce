package order

import (
	"strings"
	"time"

	"appengine/search"

	"hanzo.io/models/mixin"
	"hanzo.io/models/types/country"
)

type Document struct {
	mixin.DocumentSaveLoad `datastore:"-" json:"-"`

	// Special Kind Option
	Kind search.Atom `search:",facet"`

	Id_        string
	Number     float64
	UserId     string
	StoreId    string
	CartId     string
	ReferrerId string

	ProductNames string
	ProductIds   string
	ProductSlugs string

	BillingAddressName        string
	BillingAddressLine1       string
	BillingAddressLine2       string
	BillingAddressCity        string
	BillingAddressState       string
	BillingAddressCountryCode string
	BillingAddressCountry     string
	BillingAddressPostalCode  string

	ShippingAddressName        string
	ShippingAddressLine1       string
	ShippingAddressLine2       string
	ShippingAddressCity        string
	ShippingAddressState       string
	ShippingAddressCountryCode string
	ShippingAddressCountry     string
	ShippingAddressPostalCode  string

	Type string

	CouponCodes string

	Status            string
	PaymentStatus     string
	FulfillmentStatus string
	Preorder          string
	Confirmed         string

	FulfillmentTracking   string
	FulfillmentExternalId string
	FulfillmentService    string
	FulfillmentCarrier    string

	CreatedAt time.Time
	UpdatedAt time.Time

	// Facets
	ProductNameOption0 search.Atom `search:"productName,facet"`
	ProductNameOption1 search.Atom `search:"productName,facet"`
	ProductNameOption2 search.Atom `search:"productName,facet"`
	ProductNameOption3 search.Atom `search:"productName,facet"`
	ProductNameOption4 search.Atom `search:"productName,facet"`
	ProductNameOption5 search.Atom `search:"productName,facet"`
	ProductNameOption6 search.Atom `search:"productName,facet"`
	ProductNameOption7 search.Atom `search:"productName,facet"`
	ProductNameOption8 search.Atom `search:"productName,facet"`
	ProductNameOption9 search.Atom `search:"productName,facet"`

	BillingAddressCityOption       search.Atom `search:"billingAddressCity,facet"`
	BillingAddressStateOption      search.Atom `search:"billingAddressState,facet"`
	BillingAddressPostalCodeOption search.Atom `search:"billingAddressPostalCode,facet"`
	BillingAddressCountryOption    search.Atom `search:"billingAddressCountry,facet"`

	ShippingAddressCityOption       search.Atom `search:"shippingAddressCity,facet"`
	ShippingAddressStateOption      search.Atom `search:"shippingAddressState,facet"`
	ShippingAddressPostalCodeOption search.Atom `search:"shippingAddressPostalCode,facet"`
	ShippingAddressCountryOption    search.Atom `search:"shippingAddressCountry,facet"`

	DiscountOption   float64 `search:"discount,facet"`
	SubtotalOption   float64 `search:"subtotal,facet"`
	ShippingOption   float64 `search:"shipping,facet"`
	TaxOption        float64 `search:"tax,facet"`
	AdjustmentOption float64 `search:"adjustment,facet"`
	TotalOption      float64 `search:"total,facet"`
	PaidOption       float64 `search:"paid,facet"`
	RefundedOption   float64 `search:"refunded,facet"`

	TypeOption        search.Atom `search:"type,facet"`
	CouponCodeOption0 search.Atom `search:"couponCode,facet"`

	StatusOption            search.Atom `search:"status,facet"`
	PaymentStatusOption     search.Atom `search:"paymentStatus,facet"`
	FulfillmentStatusOption search.Atom `search:"fulfillmentStatus,facet"`

	FulfillmentTrackingOption0 search.Atom `search:"fulfillmentTracking,facet"`
	FulfillmentTrackingOption1 search.Atom `search:"fulfillmentTracking,facet"`
	FulfillmentTrackingOption2 search.Atom `search:"fulfillmentTracking,facet"`
}

func (d *Document) Id() string {
	return d.Id_
}

func (d *Document) Init() {
	d.SetDocument(d)
}

func (o Order) Document() mixin.Document {
	productIds := make([]string, 0)
	productSlugs := make([]string, 0)
	productNames := make([]string, 0)
	for _, item := range o.Items {
		productIds = append(productIds, item.ProductId)
		productSlugs = append(productIds, item.ProductSlug)
		productNames = append(productIds, item.ProductName)
	}
	trackings := make([]string, 0)
	for _, tracking := range o.Fulfillment.Trackings {
		trackings = append(trackings, tracking.Number)
	}

	doc := &Document{}
	doc.Init()
	doc.Kind = search.Atom(kind)
	doc.Id_ = o.Id()
	doc.Number = float64(o.NumberFromId())
	doc.UserId = o.UserId
	doc.StoreId = o.StoreId
	doc.CartId = o.CartId
	doc.ReferrerId = o.ReferrerId

	doc.ProductNames = strings.Join(productNames, " ")
	doc.ProductIds = strings.Join(productIds, " ")
	doc.ProductSlugs = strings.Join(productSlugs, " ")

	doc.BillingAddressName = o.BillingAddress.Name
	doc.BillingAddressLine1 = o.BillingAddress.Line1
	doc.BillingAddressLine2 = o.BillingAddress.Line2
	doc.BillingAddressCity = o.BillingAddress.City
	doc.BillingAddressState = o.BillingAddress.State
	doc.BillingAddressCountryCode = o.BillingAddress.Country
	doc.BillingAddressCountry = country.ByISOCodeISO3166_2[o.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder
	doc.BillingAddressPostalCode = o.BillingAddress.PostalCode

	doc.ShippingAddressName = o.ShippingAddress.Name
	doc.ShippingAddressLine1 = o.ShippingAddress.Line1
	doc.ShippingAddressLine2 = o.ShippingAddress.Line2
	doc.ShippingAddressCity = o.ShippingAddress.City
	doc.ShippingAddressState = o.ShippingAddress.State
	doc.ShippingAddressCountryCode = o.ShippingAddress.Country
	doc.ShippingAddressCountry = country.ByISOCodeISO3166_2[o.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder
	doc.ShippingAddressPostalCode = o.ShippingAddress.PostalCode

	doc.Type = string(o.Type)

	doc.CreatedAt = o.CreatedAt
	doc.UpdatedAt = o.UpdatedAt

	doc.CouponCodes = strings.Join(o.CouponCodes, " ")
	doc.ReferrerId = o.ReferrerId

	doc.Status = string(o.Status)
	doc.PaymentStatus = string(o.PaymentStatus)
	doc.FulfillmentStatus = string(o.Fulfillment.Status)
	if o.Preorder {
		doc.Preorder = "preorder"
	}
	if !o.Unconfirmed {
		doc.Confirmed = "confirmed"
	}

	doc.FulfillmentTracking = strings.Join(trackings, " ")
	doc.FulfillmentExternalId = o.Fulfillment.ExternalId
	doc.FulfillmentService = o.Fulfillment.Service
	doc.FulfillmentCarrier = o.Fulfillment.Carrier

	nItems := len(o.Items)
	if nItems > 0 {
		doc.ProductNameOption0 = search.Atom(o.Items[0].ProductName)
	}
	if nItems > 1 {
		doc.ProductNameOption1 = search.Atom(o.Items[1].ProductName)
	}
	if nItems > 2 {
		doc.ProductNameOption2 = search.Atom(o.Items[2].ProductName)
	}
	if nItems > 3 {
		doc.ProductNameOption3 = search.Atom(o.Items[3].ProductName)
	}
	if nItems > 4 {
		doc.ProductNameOption4 = search.Atom(o.Items[4].ProductName)
	}
	if nItems > 5 {
		doc.ProductNameOption5 = search.Atom(o.Items[5].ProductName)
	}
	if nItems > 6 {
		doc.ProductNameOption6 = search.Atom(o.Items[6].ProductName)
	}
	if nItems > 7 {
		doc.ProductNameOption7 = search.Atom(o.Items[7].ProductName)
	}
	if nItems > 8 {
		doc.ProductNameOption8 = search.Atom(o.Items[8].ProductName)
	}
	if nItems > 9 {
		doc.ProductNameOption9 = search.Atom(o.Items[9].ProductName)
	}

	doc.BillingAddressCityOption = search.Atom(doc.BillingAddressCity)
	doc.BillingAddressStateOption = search.Atom(doc.BillingAddressState)
	doc.BillingAddressPostalCodeOption = search.Atom(doc.BillingAddressPostalCode)
	doc.BillingAddressCountryOption = search.Atom(doc.BillingAddressCountry)

	doc.ShippingAddressCityOption = search.Atom(doc.ShippingAddressCity)
	doc.ShippingAddressStateOption = search.Atom(doc.ShippingAddressState)
	doc.ShippingAddressPostalCodeOption = search.Atom(doc.ShippingAddressPostalCode)
	doc.ShippingAddressCountryOption = search.Atom(doc.ShippingAddressCountry)

	doc.DiscountOption = o.Currency.ToFloat(o.Discount)
	doc.SubtotalOption = o.Currency.ToFloat(o.Subtotal)
	doc.ShippingOption = o.Currency.ToFloat(o.Shipping)
	doc.TaxOption = o.Currency.ToFloat(o.Tax)
	doc.AdjustmentOption = o.Currency.ToFloat(o.Adjustment)
	doc.TotalOption = o.Currency.ToFloat(o.Total)
	doc.PaidOption = o.Currency.ToFloat(o.Paid)
	doc.RefundedOption = o.Currency.ToFloat(o.Refunded)

	doc.TypeOption = search.Atom(o.Type)

	nCoupons := len(o.CouponCodes)
	if nCoupons > 0 {
		doc.CouponCodeOption0 = search.Atom(o.CouponCodes[0])
	}

	doc.StatusOption = search.Atom(doc.Status)
	doc.PaymentStatusOption = search.Atom(doc.PaymentStatus)
	doc.FulfillmentStatusOption = search.Atom(doc.FulfillmentStatus)

	nTrackings := len(o.Fulfillment.Trackings)
	if nTrackings > 0 {
		doc.FulfillmentTrackingOption0 = search.Atom(o.Fulfillment.Trackings[0].Number)
	}
	if nTrackings > 1 {
		doc.FulfillmentTrackingOption1 = search.Atom(o.Fulfillment.Trackings[1].Number)
	}
	if nTrackings > 2 {
		doc.FulfillmentTrackingOption2 = search.Atom(o.Fulfillment.Trackings[2].Number)
	}

	return doc
}
