package models

import (
	"github.com/mholt/binding"
	"net/http"
)

type User struct {
	FieldMapMixin
	Id              string `schema:"-"`
	FirstName       string
	LastName        string
	Email           string
	Phone           string
	OrdersIds       []string `schema:"-"`
	Cart            Cart
	BillingAddress  Address
	ShippingAddress Address

	// SHOULD ONLY BE ACCESSED INTERNALLY. PLZ.
	StripeToken      string
	CardconnectToken string
}

type Owner struct {
	Id           string
	Name         string
	Email        string
	PasswordHash []byte
	Campaigns    []Campaign
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Name cannot be empty string.
	if u.FirstName == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"FirstName"},
			Classification: "InputError",
			Message:        "User first name cannot be empty.",
		})
	}

	if u.LastName == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"LastName"},
			Classification: "InputError",
			Message:        "User last name cannot be empty.",
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
	errs = u.BillingAddress.Validate(req, errs)
	errs = u.ShippingAddress.Validate(req, errs)

	return errs
}

type Address struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

func (a Address) Line() string {
	return a.Line1 + " " + a.Line2
}

func (a Address) Validate(req *http.Request, errs binding.Errors) binding.Errors {

	if a.Line() == "" {
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
