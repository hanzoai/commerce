package store

import (
	"reflect"

	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/models2/types/shipping"
	"crowdstart.io/models2/types/weight"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/structs"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Everything is a pointer, which allows fields to be nil. This way when we
// serialize to/from JSON we know what has and has not been set.
type Listing struct {
	ProductId string `json:"productId,omitempty"`
	VariantId string `json:"variantId,omitempty"`

	Name *string `json:"name"`

	Headline    *string `json:"headline,omitempty"`
	Excerpt     *string `json:"excerpt,omitempty"`
	Description *string `json:"description,omitempty"`

	// Product Media
	HeaderImage *Media  `json:"headerImage,omitempty"`
	Media       []Media `json:"media,omitempty"`

	Sold *int `json:"sold"`

	Price    *currency.Cents `json:"price,omitempty"`
	Shipping *currency.Cents `json:"shipping,omitempty"`
	Taxable  *bool           `json:"taxable,omitempty"`

	WeightUnit weight.Unit `json:"weightUnit,omitempty"`

	Available    *bool         `json:"available,omitempty"`
	Availability *Availability `json:"availability,omitempty"`

	Hidden *bool `json:"hidden,omitempty"`
}

var ListingFields = structs.FieldNames(Listing{})

type Listings map[string]Listing
type ShippingRateTable map[string]shipping.Rates

type Store struct {
	mixin.Model

	// Full name of store
	Name string `json:"name"`

	// Unique human readable id for url <slug>.crowdstart.come
	Slug string `json:"slug"`

	// Where this is hosted if not on crowdstart.com
	Hostname string `json:"hostname"`
	Prefix   string `json:"prefix"`

	// Currency for store
	Currency currency.Type `json:"currency"`

	// Taxation information
	TaxNexus []Address `json:"taxNexus"`

	// Shipping Rate Table, country name to shipping rate
	ShippingRateTable  ShippingRateTable `json:"shippingRates" datastore:"-"`
	ShippingRateTable_ string            `json:"-" datastore:",noindex"`

	// Overrides per item
	Listings  Listings `json:"listings" datastore:"-"`
	Listings_ string   `json:"-" datastore:",noindex"`

	Salesforce struct {
		PriceBookId string `json:"PriceBookId"`
	} `json:"-"`
}

func (s Store) Kind() string {
	return "store"
}

func (s *Store) Init() {
	s.ShippingRateTable = make(ShippingRateTable)
	s.Listings = make(Listings)
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s *Store) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Init()

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

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Store) Validator() *val.Validator {
	return val.New(s)
}

func (s *Store) Override(entity mixin.Entity) {
	listing, ok := s.Listings[entity.Id()]
	if !ok {
		log.Warn("No listing found that matches given %s", entity.Kind())
		return
	}

	ev := reflect.ValueOf(entity)
	lv := reflect.ValueOf(listing)

	for _, name := range ListingFields {
		field := reflect.Indirect(ev).FieldByName(name)
		val := lv.FieldByName(name)
		field.Set(val)
	}
}
