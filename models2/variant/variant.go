package variant

import (
	"net/http"

	"crowdstart.io/models/mixin"
	"github.com/mholt/binding"

	. "crowdstart.io/models2"
)

type Option struct {
	// Ex. Size
	Name string
	// Ex. M
	Value string
}

type Variant struct {
	mixin.Salesforce
	*mixin.Model `datastore:"-"`

	SKU  string
	Name string

	Price Cents

	Inventory int
	Sold      int

	Weight     float64
	WeightUnit MassUnit
	Dimensions string

	Options []Option
}

func (v Variant) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if v.SKU == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"SKU"},
			Classification: "InputError",
			Message:        "Variant does not have a SKU",
		})
	}

	if v.Dimensions == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Dimensions"},
			Classification: "InputError",
			Message:        "Variant has no given dimensions",
		})
	}
	return errs
}
