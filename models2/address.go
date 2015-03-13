package models

import (
	"net/http"

	"github.com/mholt/binding"
)

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
