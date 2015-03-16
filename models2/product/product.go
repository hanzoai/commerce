package product

import (
	"net/http"
	"reflect"
	"time"

	aeds "appengine/datastore"

	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/gob"

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
	mixin.Model

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
	Variants  []variant.Variant `datastore:"-"`
	Variants_ []byte            `json:"-"`

	// Reference to options used
	Options  []Option `datastore:"-"`
	Options_ []byte   `json:"-"`
}

func (p *Product) Load(c <-chan aeds.Property) (err error) {
	// Load properties
	if err = aeds.LoadStruct(p, c); err != nil {
		return err
	}

	// Deserialize gob encoded properties
	p.Variants = make([]variant.Variant, 0)
	p.Options = make([]Option, 0)

	if len(p.Variants_) > 0 {
		err = gob.Decode(p.Variants_, &p.Variants)
	}

	if len(p.Options_) > 0 {
		err = gob.Decode(p.Options_, &p.Options)
	}

	return err
}

func (p *Product) Save(c chan<- aeds.Property) (err error) {
	// Gob encode problematic properties
	p.Variants_, err = gob.Encode(p.Variants)
	p.Options_, err = gob.Encode(p.Options)

	if err != nil {
		return err
	}

	// Save properties
	return aeds.SaveStruct(p, c)
}

func New(db *datastore.Datastore) *Product {
	p := new(Product)
	p.Variants = make([]variant.Variant, 0)
	p.Options = make([]Option, 0)
	p.Model = mixin.Model{Db: db, Entity: p}
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

func init() {
	gob.Register(variant.Variant{})
	gob.Register(Option{})
}
