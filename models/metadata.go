package models

import (
	"net/http"

	"github.com/mholt/binding"
)

type Metadata struct {
	Type  string
	Value string
}

func (m Metadata) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Type and Value cannot be the empty string.

	if m.Type == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Type"},
			Classification: "InputError",
			Message:        "Type cannot be empty.",
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
