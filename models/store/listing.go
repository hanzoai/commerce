package store

import (
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/weight"

	. "hanzo.io/models"
)

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
