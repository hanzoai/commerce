package store

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/shipping"
	"hanzo.io/models/types/weight"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/reflect"
	"hanzo.io/util/val"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Everything is a pointer, which allows fields to be nil. This way when we
// serialize to/from JSON we know what has and has not been set.
type Listing struct {
	// Not customizable
	ProductId string        `json:"productId,omitempty"`
	Slug      string        `json:"slug,omitempty"`
	VariantId string        `json:"variantId,omitempty"`
	SKU       string        `json:"sku,omitempty"`
	Currency  currency.Type `json:"currency,omitempty"`

	// Everything else May be overriden

	Name *string `json:"name"`

	Headline    *string `json:"headline,omitempty"`
	Excerpt     *string `json:"excerpt,omitempty"`
	Description *string `json:"description,omitempty"`

	// Product Media
	HeaderImage *Media   `json:"headerImage,omitempty"`
	Media       *[]Media `json:"media,omitempty"`

	Sold *int `json:"sold"`

	Price     *currency.Cents `json:"price,omitempty"`
	ListPrice *currency.Cents `json:"listPrice,omitempty"`
	Shipping  *currency.Cents `json:"shipping,omitempty"`
	Taxable   *bool           `json:"taxable,omitempty"`

	WeightUnit *weight.Unit `json:"weightUnit,omitempty"`

	Available    *bool         `json:"available,omitempty"`
	Availability *Availability `json:"availability,omitempty"`

	Hidden *bool `json:"hidden,omitempty"`
}

var ListingFields = reflect.FieldNames(Listing{})

type Listings map[string]Listing
type ShippingRateTable map[string]shipping.Rates

type Store struct {
	mixin.Model

	// Full name of store
	Name string `json:"name"`

	// Unique human readable id for url <slug>.hanzo.ioe
	Slug string `json:"slug"`

	// Where this is hosted if not on hanzo.io
	Domain string `json:"domain"`
	Prefix string `json:"prefix"`

	// Currency for store
	Currency currency.Type `json:"currency"`

	// Taxation information

	Address  Address   `json:"address,omitempty"`
	TaxNexus []Address `json:"taxNexus,omitempty"`

	// Shipping Rate Table, country name to shipping rate
	ShippingRateTable  ShippingRateTable `json:"shippingRates" datastore:"-"`
	ShippingRateTable_ string            `json:"-" datastore:",noindex"`

	// Overrides per item
	Listings  Listings `json:"listings" datastore:"-"`
	Listings_ string   `json:"-" datastore:",noindex"`

	Salesforce struct {
		PriceBookId string `json:"PriceBookId"`
	} `json:"-"`

	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Timezone string `json:"timezone,omitempty`

	Mailchimp struct {
		ListId string `json:"listId"`
		APIKey string `json:"apiKey"`
	} `json:"mailchimp,omitempty`
}

func (s *Store) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Listings_) > 0 {
		err = json.DecodeBytes([]byte(s.Listings_), &s.Listings)
	}

	if len(s.ShippingRateTable_) > 0 {
		err = json.DecodeBytes([]byte(s.ShippingRateTable_), &s.ShippingRateTable)
	}

	return err
}

func (s *Store) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Listings_ = string(json.EncodeBytes(&s.Listings))
	s.ShippingRateTable_ = string(json.EncodeBytes(&s.ShippingRateTable))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Store) Validator() *val.Validator {
	return val.New()
}

// Add a new listing to the listings map
func (s *Store) AddListing(id string, listing Listing) {
	listing.Currency = s.Currency
	s.Listings[id] = listing
}

// Update product/variant using listing for said item
func (s *Store) UpdateFromListing(entity mixin.Entity) {
	// Check if we have a listing for this product/variant
	listing, ok := s.Listings[entity.Id()]
	if !ok {
		log.Warn("No listing found that matches given %s", entity.Kind())
		return
	}

	ev := reflect.Indirect(reflect.ValueOf(entity))
	lv := reflect.ValueOf(listing)

	// Loop over listing fields and set any that this listing has that are non-nil
	for _, name := range ListingFields {
		field := ev.FieldByName(name)
		val := reflect.Indirect(lv.FieldByName(name))
		if val.IsValid() && field.IsValid() {
			field.Set(val)
		}
	}

	// Ensure currency is set to currency of store
	field := ev.FieldByName("Currency")
	field.Set(reflect.ValueOf(s.Currency))
}
