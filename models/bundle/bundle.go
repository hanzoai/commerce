package bundle

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"

	. "crowdstart.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// A bundle of Products/Variants to be listed on a store
type Bundle struct {
	mixin.Model

	// Unique human readable identifier
	Slug string `json:"slug"`

	// Name of bundle
	Name string `json:"name"`

	// Description of bundle
	Description string `datastore:",noindex" json:"description"`

	// Image/Video/Other Media to show in a gallery
	Media []Media `json:"media"`

	// Is the bundle available
	Available bool `json:"available"`

	// Is product hidden from users
	Hidden bool `json:"hidden"`

	// Range in which bundle is available. If active, it takes precedent
	// over Available bool.
	Availability Availability `json:"availability"`

	// Show this on store?
	Published bool `json:"published"`

	// Is this a preorder?
	Preorder bool `json:"preorder"`

	// Lists of products or specific product variants that are part of this
	// bundle
	ProductIds []string `json:"productIds"`
	VariantIds []string `json:"variantIds"`
}

func (c *Bundle) Init() {
	c.Media = make([]Media, 0)
	c.ProductIds = make([]string, 0)
	c.VariantIds = make([]string, 0)
}

func New(db *datastore.Datastore) *Bundle {
	c := new(Bundle)
	c.Init()
	c.Model = mixin.Model{Db: db, Entity: c}
	return c
}

func (c Bundle) Kind() string {
	return "bundle"
}

func (c *Bundle) Validator() *val.Validator {
	return val.New(c).Check("Slug").Exists().
		Check("Name").Exists()
}

func (c Bundle) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Bundle) DisplayTitle() string {
	return DisplayTitle(c.Name)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
