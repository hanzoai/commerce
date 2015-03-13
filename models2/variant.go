package models

import (
	"net/http"

	"github.com/mholt/binding"
)

type Variant struct {
	SalesforceSObject

	SKU  string
	Name string

	Price Cents

	Inventory int
	Sold      int

	Weight     float64
	WeightUnit MassUnit
	Dimensions string

	Options []VariantOption
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
