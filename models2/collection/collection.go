package collection

import (
	"time"

	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/gob"

	. "crowdstart.io/models2"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// A collection of Products/Variants to be listed on a store
type Collection struct {
	mixin.Model

	// Unique human readable identifier
	Slug string `json:"slug"`

	// Name of Collection
	Name string `json:"name"`

	// Description of collection
	Description string `datastore:",noindex" json:"description"`

	// Image/Video/Other Media to show in a gallery
	Media []Media `json:"media"`

	// What time is this collection available to deliver/purchase
	AvailableBy time.Time `json:"availableBy"`

	// Show this on store?
	Published bool `json:"published"`

	// Is this a preorder?
	Preorder bool `json:"preorder"`

	// Is this in stock?
	OutOfStock bool `json:"outOfStock"`

	// Lists of products or specific product variants that are part of this collection
	ProductIds []string `json:"productIds"`
	VariantIds []string `json:"variantIds"`

	// Discount for this purchase
	Discounts  []*Discount `json:"discounts"`
	Discounts_ []byte      `json:"-"`

	History []Event `json:"history"`
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Model = mixin.Model{Db: db, Entity: c}
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
	c.Discounts = make([]*Discount, 0)
	c.History = make([]Event, 0)
	return c
}

func (c Collection) Kind() string {
	return "collection2"
}

func (c *Collection) Load(ch <-chan aeds.Property) (err error) {
	// Load properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(c, ch)); err != nil {
		return err
	}

	// Deserialize gob encoded properties
	c.Discounts = make([]*Discount, 0)

	if len(c.Discounts_) > 0 {
		err = gob.Decode(c.Discounts_, &c.Discounts)
	}

	return err
}

func (c *Collection) Save(ch chan<- aeds.Property) (err error) {
	// Gob encode problematic properties
	c.Discounts_, err = gob.Encode(&c.Discounts)

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(c, ch))
}

func (c Collection) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Collection) DisplayTitle() string {
	return DisplayTitle(c.Name)
}
