package models

import (
	"net/http"

	"github.com/mholt/binding"
)

type User struct {
	FieldMapMixin
	Id              string `schema:"-" json:"-"`
	FirstName       string
	LastName        string
	Phone           string
	Cart            Cart `datastore:"-" json:"-"`
	BillingAddress  Address
	ShippingAddress Address
	Email           string
	Campaigns       []Campaign `schema:"-" datastore:"-"`
	PasswordHash    []byte     `schema:"-" json:"-"`

	Facebook struct {
		AccessToken string `json:"-"`
		UserId      string `json:"id"`
		FirstName   string `json:"first_name" datastore:"-"`
		LastName    string `json:"last_name" datastore:"-"`
		MiddleName  string `json:"middle_name"`
		Name        string `json:"name"`
		NameFormat  string `json:"name_format"` // For Chinese, Japanese, and Korean names. Possibly used in the future.
		Email       string `json:"email" datastore:"-"`
	}

	Stripe struct {
		CustomerId string
	}
}

func (u User) Name() string {
	return u.FirstName + " " + u.LastName
}

func (u User) HasPassword() bool {
	return len(u.PasswordHash) != 0
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
