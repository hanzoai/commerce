package models

import (
	"github.com/mholt/binding"
	"net/http"
	"time"
)

type LineItem struct {
	FieldMapMixin
	SKU          string
	Cost         int64
	Description  string
	DiscountAmnt int64
	LineNo       int
	Quantity     int
	UOM          string
	// Material     string
	// NetAmnt      string
	// TaxAmnt      string
	// UPC          string
}

func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if li.SKU == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"SKU"},
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

	// Validate against database?
	if li.Cost < 1 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Cost"},
			Classification: "InputError",
			Message:        "Cost is too low.",
		})
	}
	return errs
}

type Cart struct {
	FieldMapMixin
	Id        string
	Items     []LineItem
	CreatedAt time.Time
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
	Type    string
}

type Order struct {
	FieldMapMixin
	Account         PaymentAccount
	BillingAddress  Address
	CreatedAt       time.Time
	Id              string
	Shipping        int64
	ShippingAddress Address
	Subtotal        int64
	Tax             int64
	Total           int64
	User            User
	Items           []LineItem
	// ShippingOption  ShippingOption
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
