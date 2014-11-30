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

type PaymentAccount struct {
	CVV2   int
	Month  int
	Year   int
	Expiry string
	Number string
	Type   string `schema:"-"`
}

type Charge struct {
	ID             string
	Live           bool
	Paid           bool
	Desc           string
	Email          string
	Amount         uint64
	FailMsg        string
	Created        int64
	Refunded       bool
	Captured       bool
	FailCode       string
	Statement      string
	AmountRefunded uint64
}

type ShippingOption struct {
	Name  string
	Price int64
}

func (so ShippingOption) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if so.Name == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Name"},
			Classification: "InputError",
			Message:        "Shipping option has no name.",
		})
	}
	return errs
}
