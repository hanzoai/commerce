package models

import (
	"net/http"
	"github.com/mholt/binding"
)


// A single piece of metadata
type Datum struct {
	Key   string
	Type  string
	Value string
}

func (m Datum) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Type and Value cannot be the empty string.

	if m.Key == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Key"},
			Classification: "InputError",
			Message:        "Key cannot be empty.",
		})
	}

	if m.Value == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Value"},
			Classification: "InputError",
			Message:        "Value cannot be empty.",
		})
	}
	return errs
}
