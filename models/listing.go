package models

import "strings"

type Config struct {
	FieldMapMixin
	Product         string //product id
	Variant         string //optional variant sku
	Quantity        int    //number of products of optional variant type
	PriceAdjustment int    //cost adjustment for individual product in package (subtracted from actual price)
}

type Listing struct {
	FieldMapMixin
	Id          string
	SKU         string
	Title       string
	Description string `datastore:",noindex"`
	Disabled    bool

	Images []Image

	Configs []Config
}

func (l Listing) GetProductSlugs() []string {
	productConfigs := l.Configs
	slugs := make([]string, len(productConfigs), len(productConfigs))
	for i, productConfig := range productConfigs {
		slugs[i] = productConfig.Product
	}
	return slugs
}

func (l Listing) GetProductSlugsString() string {
	return strings.Join(l.GetProductSlugs(), " ")
}

func (l Listing) GetDescriptionParagraphs() []string {
	return SplitParagraph(l.Description)
}

func (l Listing) DisplayTitle() string {
	return DisplayTitle(l.Title)
}
