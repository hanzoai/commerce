package product

import (
	"reflect"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/dimensions"
	"hanzo.io/models/types/weight"
	"hanzo.io/models/variant"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Option struct {
	// Ex. Size
	Name string `json:"name"`
	// Ex. [S, M, L]
	Values []string `json:"values"`
}

// Prune down since Product Listing has a lot of this info now
type Product struct {
	mixin.Model

	// Unique human readable id
	Slug string `json:"slug"`
	SKU  string `json:"sku,omitempty"`
	UPC  string `json:"upc,omitempty"`

	// 3-letter ISO currency code (lowercase).
	Currency      currency.Type  `json:"currency"`
	Price         currency.Cents `json:"price"`
	ListPrice     currency.Cents `json:"listPrice,omitempty"`
	InventoryCost currency.Cents `json:"-"`

	// Basic cost for shipping this product
	Shipping currency.Cents `json:"shipping"`

	Inventory int `json:"inventory"`

	Weight         weight.Mass     `json:"weight"`
	WeightUnit     weight.Unit     `json:"weightUnit"`
	Dimensions     dimensions.Size `json:"dimensions"`
	DimensionUnits dimensions.Unit `json:"dimensionsUnit"`

	Taxable bool `json:"taxable"`

	// Product Name
	Name string `json:"name"`

	// Product headline
	Headline string `json:"headline" datastore:",noindex"`

	// Product Excerpt
	Excerpt string `json:"excerpt" datastore:",noindex"`

	// Product Description
	Description string `json:"description", datastore:",noindex"`

	// Product Media
	Header Media   `json:"header"`
	Image  Media   `json:"image"`
	Media  []Media `json:"media"`

	// Is the product available
	Available bool `json:"available"`

	// Is product hidden from users
	Hidden bool `json:"hidden"`

	// Range in which product is available. If active, it takes precedent over
	// Available bool.
	Availability Availability `json:"availability"`

	// Is this product for preorder
	Preorder bool `json:"preorder"`

	// Pre-order now or Add to cart
	AddLabel string `json:"addLabel"`

	// Optional Estimated Delivery line
	EstimatedDelivery string `json:"estimatedDelivery"`

	// List of variants
	Variants  []*variant.Variant `json:"variants" datastore:"-"`
	Variants_ string             `json:"-" datastore:",noindex"`

	// Reference to options used
	Options  []*Option `json:"options" datastore:"-"`
	Options_ string    `json:"-" datastore:",noindex"`
}

func (p *Product) Validator() *val.Validator {
	return val.New().
		Check("Slug").Exists().
		Check("SKU").Exists().
		Check("Name").Exists()
	// 	if p.Name == "" {
	// 		errs = append(errs, binding.Error{
	// 			FieldNames:     []string{"Title"},
	// 			Classification: "InputError",
	// 			Message:        "Product does not have a title.",
	// 		})
	// 	}

	// 	if len(p.Media) > 0 {
	// 		for _, media := range p.Media {
	// 			errs = media.Validate(req, errs)
	// 		}
	// 	}

	// 	if len(p.Variants) == 0 {
	// 		errs = append(errs, binding.Error{
	// 			FieldNames:     []string{"Variants"},
	// 			Classification: "InputError",
	// 			Message:        "No Variants on Product",
	// 		})
	// 	} else {
	// 		for _, v := range p.Variants {
	// 			errs = v.Validate(req, errs)
	// 		}
	// 	}
	// 	return errs
}

func (p *Product) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	p.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(p, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Variants_) > 0 {
		err = json.DecodeBytes([]byte(p.Variants_), &p.Variants)
	}

	if len(p.Options_) > 0 {
		err = json.DecodeBytes([]byte(p.Options_), &p.Options)
	}

	return err
}

func (p *Product) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	p.Variants_ = string(json.EncodeBytes(&p.Variants))
	p.Options_ = string(json.Encode(&p.Options))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(p, c))
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
	return DisplayPrice(p.Currency, p.MinPrice())
}

func (p Product) MinPrice() currency.Cents {
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
