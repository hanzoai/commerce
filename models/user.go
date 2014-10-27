package models

import (
	"net/http"
	"github.com/mholt/binding"
)

type User struct {
	Id              string
	Name            string
	Email           string
	Phone           string
	OrdersIds       []string
	Cart            Cart
	BillingAddress  Address
	ShippingAddress Address
}

func (u *User) FieldMap() binding.FieldMap {
	return binding.FieldMap{}
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

    // Validate cart implicitly.
    u.Cart.Validate(req, errs)

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
