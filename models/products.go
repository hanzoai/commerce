package models

import (
	"github.com/dustin/go-humanize"
	"github.com/mholt/binding"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func FloatPrice(price int64) float64 {
	return math.Floor(float64(price)*100+0.5) / 1000000
}

type Product struct {
	FieldMapMixin
	Id          string
	Slug        string
	Title       string
	Headline    string
	Excerpt     string
	Description string
	Released    time.Time
	Available   bool
	Stocked     int
	AddLabel    string // Pre-order now or Add to cart
	HeaderImage Image
	Images      []Image
	Variants    []ProductVariant
}

func (p Product) DisplayImage() Image {
	if len(p.Images) > 0 {
		return p.Images[0]
	}
	return Image{}
}

func (p Product) MinPrice() int64 {
	min := p.Variants[0].Price

	for _, v := range p.Variants {
		if v.Price < min {
			min = v.Price
		}
	}

	return min
}

func (p Product) DisplayPrice() string {
	f := strconv.FormatFloat(FloatPrice(p.MinPrice()), 'f', 2, 64)
	bits := strings.Split(f, ".")
	decimal := bits[1]
	integer, _ := strconv.ParseInt(bits[0], 10, 64)
	return humanize.Comma(integer) + "." + decimal
}

// TODO: Don't do this.
func (p Product) VariantOptions(name string) (options []string) {
	set := make(map[string]bool)

	for _, v := range p.Variants {
		r := reflect.ValueOf(v)
		f := reflect.Indirect(r).FieldByName(name)
		v := f.String()
		if v != "" {
			set[v] = true
		}
	}

	for key := range set {
		options = append(options, key)
	}

	return options
}

func (p Product) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if p.Title == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Title"},
			Classification: "InputError",
			Message:        "Product does not have a title.",
		})
	}

	if len(p.Images) > 0 {
		for _, image := range p.Images {
			errs = image.Validate(req, errs)
		}
	}

	if len(p.Variants) == 0 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Variants"},
			Classification: "InputError",
			Message:        "No Variants on Product",
		})
	} else {
		for _, v := range p.Variants {
			errs = v.Validate(req, errs)
		}
	}
	return errs
}

type ProductVariant struct {
	FieldMapMixin
	Id         string
	SKU        string
	Price      int64
	Stock      int
	Weight     int
	Dimensions string
	Color      string
	Size       string
}

func (pv ProductVariant) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if pv.SKU == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"SKU"},
			Classification: "InputError",
			Message:        "Variant does not have a SKU",
		})
	}

	if pv.Dimensions == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Dimensions"},
			Classification: "InputError",
			Message:        "Variant has no given dimensions",
		})
	}
	return errs
}

type Image struct {
	FieldMapMixin
	Alt string
	Url string
}

func (i Image) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if i.Url == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Url"},
			Classification: "InputError",
			Message:        "Image does not have a URL",
		})
	}
	return errs
}
