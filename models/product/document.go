package product

import (
	"time"

	"appengine/search"

	"hanzo.io/models/mixin"
)

type Document struct {
	mixin.DocumentSaveLoad `datastore:"-" json:"-"`

	// Special Kind Option
	Kind search.Atom `search:",facet"`

	Id_               string
	Slug              string
	SKU               string
	UPC               string
	Name              string
	Description       string
	EstimatedDelivery string

	// Facets
	PriceOption         float64 `search:"price,facet"`
	ListPriceOption     float64 `search:"listPrice,facet"`
	InventoryCostOption float64 `search:"inventoryCost,facet"`

	InventoryOption float64 `search:"inventory,facet"`

	WeightOption     float64     `search:"weight,facet"`
	WeightUnitOption search.Atom `search:"weightUnit,facet"`

	DimensionsLengthOption float64     `search:"dimensionLength,facet"`
	DimensionsWidthOption  float64     `search:"dimensionWidth,facet"`
	DimensionsHeightOption float64     `search:"dimensionHeight,facet"`
	DimensionUnitsOption   search.Atom `search:"dimensionUnits,facet"`

	AvailableOption search.Atom `search:"available,facet"`
	HiddenOption    search.Atom `search:"hidden,facet"`
	PreorderOption  search.Atom `search:"preorder,facet"`

	CreatedAtOption time.Time `search:"createdAt,facet"`
	UpdatedAtOption time.Time `search:"updatedAt,facet"`
}

func (d *Document) Id() string {
	return d.Id_
}

func (d *Document) Init() {
	d.SetDocument(d)
}

func (p Product) Document() mixin.Document {
	doc := &Document{}
	doc.Init()
	doc.Kind = search.Atom(kind)
	doc.Id_ = p.Id()
	doc.Slug = p.Slug
	doc.SKU = p.SKU
	doc.UPC = p.UPC
	doc.Name = p.Name
	doc.Description = p.Description
	doc.EstimatedDelivery = p.EstimatedDelivery

	doc.PriceOption = p.Currency.ToFloat(p.Price)
	doc.ListPriceOption = p.Currency.ToFloat(p.ListPrice)
	doc.InventoryCostOption = p.Currency.ToFloat(p.InventoryCost)

	doc.InventoryOption = float64(p.Inventory)
	doc.WeightOption = float64(p.Weight)
	doc.WeightUnitOption = search.Atom(p.WeightUnit)

	doc.DimensionsLengthOption = p.Dimensions.Length
	doc.DimensionsWidthOption = p.Dimensions.Width
	doc.DimensionsHeightOption = p.Dimensions.Height
	doc.DimensionUnitsOption = search.Atom(p.DimensionUnits)

	if p.Available {
		doc.AvailableOption = "available"
	}

	if p.Hidden {
		doc.HiddenOption = "hidden"
	}

	if p.Preorder {
		doc.PreorderOption = "preorder"
	}

	doc.CreatedAtOption = p.CreatedAt
	doc.UpdatedAtOption = p.UpdatedAt

	return doc
}
