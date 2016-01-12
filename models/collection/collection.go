package collection

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
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

	// Is the collection available
	Available bool `json:"available"`

	// Range in which collection is available. If active, it takes precedent
	// over Available bool.
	Availability struct {
		Active    bool      `json:"active'"`
		StartDate time.Time `json:"startDate"`
		EndDate   time.Time `json:"endDate"`
	} `json:"availability"`

	// Show this on store?
	Published bool `json:"published"`

	// Is this a preorder?
	Preorder bool `json:"preorder"`

	// Is this in stock?
	OutOfStock bool `json:"outOfStock"`

	// Lists of products or specific product variants that are part of this collection
	ProductIds []string `json:"productIds"`
	VariantIds []string `json:"variantIds"`

	History []Event `json:"-"`
}

func New(db *datastore.Datastore) *Collection {
	c := new(Collection)
	c.Init(db)
	c.Defaults()
	return c
}

func (c *Collection) Init(db *datastore.Datastore) {
	c.Model = mixin.Model{Db: db, Entity: c}
}

func (c *Collection) Defaults() {
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
	c.History = make([]Event, 0)
}

func (c Collection) Kind() string {
	return "collection"
}

func (c *Collection) Validator() *val.Validator {
	return val.New().Check("Slug").Exists().
		Check("Name").Exists()
}

func (c Collection) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Collection) DisplayTitle() string {
	return DisplayTitle(c.Name)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
