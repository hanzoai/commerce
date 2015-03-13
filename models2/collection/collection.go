package collection

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"

	. "crowdstart.io/models2"
)

// A collection of Products/Variants to be listed on a store
type Collection struct {
	*mixin.Model `datastore:"-"`

	// Unique human readable identifier
	Slug string

	// Name of Collection
	Name string

	// Description of collection
	Description string `datastore:",noindex"`

	// Image/Video/Other Media to show in a gallery
	Media []Media

	// What time is this collection available to deliver/purchase
	AvailableBy time.Time

	// Show this on store?
	Published bool

	// Is this a preorder?
	Preorder bool

	// Is this in stock?
	OutOfStock bool

	// Lists of products or specific product variants that are part of this collection
	ProductIds []string
	VariantIds []string

	// Discount for this purchase
	Discounts []Discount

	// Book keeping stuff for us
	CreatedAt time.Time
	UpdatedAt time.Time

	History []Event
}

func (c Collection) Kind() string {
	return "collection"
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Model = mixin.NewModel(db, c)
	return c
}

func (c Collection) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Collection) DisplayTitle() string {
	return DisplayTitle(c.Name)
}
