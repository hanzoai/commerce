package store

import (
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/models2/types/shipping"
	"crowdstart.io/models2/types/weight"
	"crowdstart.io/util/json"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Listing struct {
	ProductId string `json:"productId,omitempty"`
	VariantId string `json:"variantId,omitempty"`

	Price currency.Cents `json:"price"`

	Taxable bool `json:"taxable"`

	WeightUnit weight.Unit `json:"weightUnit"`

	Available bool `json:"available"`
}

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
	ShippingRateTable  ShippingRateTable `json:"shippingRates" datastore"-"`
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
