package models

import (
	"net/http"
	"github.com/mholt/binding"
)

type User struct {
	Id              string `schema:"-"`
	Name            string
	Email           string
	Phone           string
	OrdersIds       []string `schema:"-"`
	Cart            Cart
	BillingAddress  Address
	ShippingAddress Address
	FieldMapMixin
}

func (u User) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Name cannot be empty string.
	if u.Name == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"Name"},
            Classification: "InputError",
            Message:        "User name cannot be empty.",
        })
    }

	if u.Email == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"Email"},
            Classification: "InputError",
            Message:        "User email cannot be empty.",
        })
    }

    // Validate cart implicitly.
    errs = u.Cart.Validate(req, errs)
    errs = u.BillingAddress.Validate(req,errs)
    errs = u.ShippingAddress.Validate(req,errs)


    return errs
}

type Address struct {
	Street     string
	Unit       string
	City       string
	State      string
	PostalCode string
	Country    string
}

func (a Address) Validate(req *http.Request, errs binding.Errors) binding.Errors {

	if a.Street == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"Street"},
            Classification: "InputError",
            Message:        "Address Street is required.",
        })
    }

	if a.City == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"City"},
            Classification: "InputError",
            Message:        "Address City is required.",
        })
    }

	if a.State == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"State"},
            Classification: "InputError",
            Message:        "Address State is required.",
        })
    }

	if a.Country == "" {
        errs = append(errs, binding.Error{
            FieldNames:     []string{"Country"},
            Classification: "InputError",
            Message:        "Address Country is required.",
        })
    }
    return errs
}
