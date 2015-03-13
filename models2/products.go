package models

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/mholt/binding"

	"crowdstart.io/util/json"
)

// Prune down since Product Listing has a lot of this info now
type Product struct {
	// Unique human readable id
	Slug string

	// Product Name
	Name string

	// Product headline
	Headline string

	// Product Excerpt
	Excerpt string

	// Product Description
	Description string `datastore:",noindex"`

	// Product Media
	HeaderImage Media
	Media       []Media

	// When is the product available
	AvailableBy time.Time

	// Is this product for preorder
	Preorder bool
	AddLabel string // Pre-order now or Add to cart

	// List of variants
	Variants []Variant

	// Reference to options used
	Option []Option
}

func (p Product) JSON() string {
	return json.Encode(&p)
}

func (p Product) DisplayTitle() string {
	return DisplayTitle(p.Name)
}

func (p Product) DisplayImage() Media {
	for _, media := range p.Media {
		if media.Type == MediaTypeImage {
			return media
		}
	}
	return Media{}
}

func (p Product) DisplayPrice() string {
	return DisplayPrice(p.MinPrice())
}

func (p Product) MinPrice() Cents {
	min := p.Variants[0].Price

	for _, v := range p.Variants {
		if v.Price < min {
			min = v.Price
		}
	}

	return min
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
	if p.Name == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Title"},
			Classification: "InputError",
			Message:        "Product does not have a title.",
		})
	}

	if len(p.Media) > 0 {
		for _, media := range p.Media {
			errs = media.Validate(req, errs)
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

type MediaType string

const (
	MediaTypeVideo      OrderStatus = "video"
	MediaTypeImage                  = "image"
	MediaTypeLiveStream             = "livestream"
	MediaTypeWebGL                  = "webgl"
	MediaTypeAudio                  = "audio"
	MediaTypeEmbed                  = "embed"
)

type Media struct {
	Type MediaType
	Alt  string
	Url  string
	X    int
	Y    int
}

func (i Media) Dimensions() string {
	return fmt.Sprintf("%sx%s", i.X, i.Y)
}

func (i Media) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if i.Url == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Url"},
			Classification: "InputError",
			Message:        "Image does not have a URL",
		})
	}
	return errs
}
