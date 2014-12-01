package models

import (
	"net/http"
	"time"

	"github.com/mholt/binding"
)

type Cart struct {
	FieldMapMixin
	Id        string `schema:"-"`
	Items     []LineItem
	CreatedAt time.Time `schema:"-"`
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
