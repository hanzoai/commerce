package collection

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"

	. "crowdstart.io/models2"
)

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
	Discounts []Discount `json:"discounts"`

	History []Event `json:"history"`
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Model = mixin.Model{Db: db, Entity: c}
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
	c.Discounts = make([]Discount, 0)
	c.History = make([]Event, 0)
	return c
}

func (c Collection) Kind() string {
	return "collection2"
}

func (c Collection) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Collection) DisplayTitle() string {
	return DisplayTitle(c.Name)
}
