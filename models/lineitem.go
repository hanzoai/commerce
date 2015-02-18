package models

import (
	"net/http"
	"strings"

	"github.com/mholt/binding"
)

type LineItem struct {
	FieldMapMixin
	SalesforceSObject

	SKU_         string         `json:"SKU"`
	Slug_        string         `json:"Slug"`
	Product      Product        `datastore:"-"`
	Variant      ProductVariant `datastore:"-"`
	Description  string
	DiscountAmnt int64
	LineNo       int
	Quantity     int

	// UOM string `schema:"-"`
	// UPC          string
	// Material     string
	// NetAmnt      string
	// TaxAmnt      string
}

func (li LineItem) Price() int64 {
	return li.Variant.Price * int64(li.Quantity)
}

func (li LineItem) DisplayPrice() string {
	return DisplayPrice(li.Price())
}

func (li LineItem) SKU() string {
	return li.Variant.SKU
}

func (li LineItem) Slug() string {
	return li.Product.Slug
}

func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if li.SKU() == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Variant.SKU"},
			Classification: "InputError",
			Message:        "SKU cannot be empty.",
		})
	}

	if li.Quantity < 1 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Quantity"},
			Classification: "InputError",
			Message:        "Quantity cannot be less than 1.",
		})
	}

	return errs
}

// Displays nice "/" delimited variant information.
func (li LineItem) DisplayShortDescription() string {
	opts := []string{}
	for _, opt := range []string{li.Product.Title, li.Variant.Color, li.Variant.Style, li.Variant.Size} {
		if opt != "" {
			opts = append(opts, opt)
		}
	}
	if len(opts) > 0 {
		return strings.Join(opts, " / ")
	} else {
		return li.SKU()
	}
}
