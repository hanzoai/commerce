package bundle

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
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

func (c *Bundle) Validator() *val.Validator {
	return val.New().
		Check("Slug").Exists().
		Check("Name").Exists()
}

func (c Bundle) GetDescriptionParagraphs() []string {
	return SplitParagraph(c.Description)
}

func (c Bundle) DisplayTitle() string {
	return DisplayTitle(c.Name)
}
