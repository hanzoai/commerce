package product

import (
	"reflect"

	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models/types/currency"
	"crowdstart.io/models/types/weight"
	"crowdstart.io/models/variant"
	"crowdstart.io/util/json"
	"crowdstart.io/util/val"

	. "crowdstart.io/models"
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
	SKU  string `json:"sku"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type  `json:"currency"`
	Price    currency.Cents `json:"price"`

	// Override for the shipping formula
	Shipping currency.Cents `json:"shipping"`

	Inventory int `json:"inventory"`
	Sold      int `json:"sold"`

	Weight     weight.Mass `json:"weight"`
	WeightUnit weight.Unit `json:"weightUnit"`
	Dimensions string      `json:"dimensions"`

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
	HeaderImage Media   `json:"headerImage"`
	Media       []Media `json:"media"`

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

	// List of variants
	Variants  []*variant.Variant `json:"variants" datastore:"-"`
	Variants_ string             `json:"-" datastore:",noindex"`

	// Reference to options used
	Options  []*Option `json:"options" datastore:"-"`
	Options_ string    `json:"-" datastore:",noindex"`
}

func (p *Product) Init() {
	p.Variants = make([]*variant.Variant, 0)
	p.Options = make([]*Option, 0)
}

func New(db *datastore.Datastore) *Product {
	p := new(Product)
	p.Init()
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p Product) Kind() string {
	return "product"
}

func (p *Product) Validator() *val.Validator {
	return val.New(p).Check("Slug").Exists().
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
	p.Init()

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
	return DisplayPrice(p.MinPrice())
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

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
