package models

import (
	"github.com/mholt/binding"
	"net/http"
	"time"
)

type LineItem struct {
	FieldMapMixin
	Product		 Product
	Variant      ProductVariant
	Description  string `schema:"-"`
	DiscountAmnt int64 `schema:"-"`
	LineNo       int `schema:"-"`
	Quantity     int
	UOM          string `schema:"-"`
	// Material     string
	// NetAmnt      string
	// TaxAmnt      string
	// UPC          string
}

func (li LineItem) Price() int64 {
	return li.Variant.Price * int64(li.Quantity)
}

func (li LineItem) DisplayPrice() string {
	return DisplayPrice(li.Price())
}

func (li LineItem) SKU() string {
	return li.Variant.SKU
}

func (li LineItem) Slug() string {
	return li.Product.Slug
}

func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if li.SKU() == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Variant.SKU"},
			Classification: "InputError",
			Message:        "SKU cannot be empty.",
		})
	}

	if li.Quantity < 1 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Quantity"},
			Classification: "InputError",
			Message:        "Quantity cannot be less than 1.",
		})
	}

	return errs
}

type Cart struct {
	FieldMapMixin
	Id        string `schema:"-"`
	Items     []LineItem
	CreatedAt time.Time `schema:"-"`
}

func (c Cart) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Cart cannot be empty.
	if len(c.Items) == 0 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Items"},
			Classification: "InputError",
			Message:        "Cart is empty.",
		})
	} else {
		for _, v := range c.Items {
			errs = v.Validate(req, errs)
		}
	}
	return errs
}

type PaymentAccount struct {
	CVV2    int
	Expiry  int
	Number  string
	Type    string `schema:"-"`
}

type Order struct {
	FieldMapMixin
	Account         PaymentAccount
	BillingAddress  Address
	CreatedAt       time.Time `schema:"-"`
	Id              string `schema:"-"`
	Shipping        int64 `schema:"-"`
	ShippingAddress Address
	Subtotal        int64 `schema:"-"`
	Tax             int64 `schema:"-"`
	Total           int64 `schema:"-"`
	User            User
	Items           []LineItem
	// ShippingOption  ShippingOption
}

func (o Order) DisplaySubtotal() string {
	return DisplayPrice(o.Subtotal)
}

func (o Order) DisplayTax() string {
	return DisplayPrice(o.Tax)
}

func (o Order) DisplayTotal() string {
	return DisplayPrice(o.Total)
}

func (o Order) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if len(o.Items) == 0 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Items"},
			Classification: "InputError",
			Message:        "Order has no items.",
		})
	} else {
		for _, v := range o.Items {
			errs = v.Validate(req, errs)
		}
	}

	return errs
}

type ShippingOption struct {
	Name  string
	Price int64
}

func (so ShippingOption) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if so.Name == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Name"},
			Classification: "InputError",
			Message:        "Shipping option has no name.",
		})
	}
	return errs
}
