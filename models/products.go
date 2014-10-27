package models

import (
	"time"
	"net/http"
	"github.com/mholt/binding"
)

type Product struct {
	Id          string
	Title       string
	Variants    []ProductVariant
	Images      []Image
	Description string
	Stocked     int
	Available   bool
	Released    time.Time
	AddLabel    string // Pre-order now or Add to cart
	FieldMapMixin
}

func (p Product) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if p.Title == "" {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Title"},
			Classification:	"InputError",
			Message:		"Product does not have a title.",
		})
	}

	if len(p.Images) > 0 {
		for _,image := range p.Images {
			errs = image.Validate(req, errs)
		}
	}

	if len(p.Variants) == 0 {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Variants"},
			Classification:	"InputError",
			Message:		"No Variants on Product",
		})
	} else {
		for _,v := range p.Variants {
			errs = v.Validate(req,errs)
		}
	}
	return errs
}

type ProductVariant struct {
	Id		   string
	Sku        string
	Price      int64
	Stock      int
	Weight     int
	Dimensions string
	Color      string
	Size       string
	FieldMapMixin
}

func (pv ProductVariant) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if pv.Sku == "" {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Sku"},
			Classification:	"InputError",
			Message:		"Variant does not have a SKU",
		})
	}

	if pv.Dimensions == "" {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Dimensions"},
			Classification:	"InputError",
			Message:		"Variant has no given dimensions",
		})
	}
	return errs
}

type Image struct {
	Name string
	Url  string
	FieldMapMixin
}


func (i Image) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if i.Url == "" {
		errs = append(errs, binding.Error {
			FieldNames:		[]string{"Url"},
			Classification:	"InputError",
			Message:		"Image does not have a URL",
		})
	}
	return errs
}
