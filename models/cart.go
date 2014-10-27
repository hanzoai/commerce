package models

import (
	"time"
	"net/http"
	"github.com/mholt/binding"
)

type LineItem struct {
	SKU			string
	Description string
	Quantity    int
}

type Cart struct {
	Id        string
	Items     []LineItem
	CreatedAt time.Time
	FieldMapMixin
}

func (c Cart) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Cart cannot be empty.
	if len(c.Items) == 0 {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"Items"},
            Classification: "InputError",
            Message:        "Cart is empty.",
        })
    }
    return errs
}

type Order struct {
	Id              string
	Items           []LineItem
	CreatedAt       time.Time
	User            User
	ShippingAddress Address
	BillingAddress  Address
	Subtotal        int
	Tax             int
	ShippingOption  ShippingOption
	Shipping        int
	Total           int
	FieldMapMixin
}

type ShippingOption struct {
	Name  string
	Price Currency
}
