package product

import (
	"net/http"
	"reflect"
	"time"

	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/variant"

	. "crowdstart.io/models2"
)

type Option struct {
	// Ex. Size
	Name string
	// Ex. [S, M, L]
	Values []string
}

// Prune down since Product Listing has a lot of this info now
type Product struct {
	*mixin.Model `datastore:"-"`

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
	Variants []variant.Variant

	// Reference to options used
	Option []Option
}

func New(db *datastore.Datastore) *Product {
	p := new(Product)
	p.Model = mixin.NewModel(db, p)
	return p
}

func (p Product) Kind() string {
	return "product2"
}

func (p Product) DisplayName() string {
	return DisplayTitle(p.Name)
}

func (p Product) DisplayImage() Media {
	for _, media := range p.Media {
		if media.Type == MediaImage {
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
