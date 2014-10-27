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

func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if li.SKU == "" {
		errs = append(errs, binding.Error{
			FieldNames:		[]string{"SKU"},
			Classification:	"InputError",
			Message:		"SKU cannot be empty.",
		})
	}
	return errs
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
    } else {
		for _,v := range c.Items {
    		errs = v.Validate(req,errs)
    	}
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

func (o Order) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if len(o.Items) == 0 {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Items"},
			Classification:	"InputError",
			Message:		"Order has no items.",
		})
	} else {
		for _,v := range o.Items {
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
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Name"},
			Classification:	"InputError",
			Message:		"Shipping option has no name.",
		})
	}
	return errs
}
