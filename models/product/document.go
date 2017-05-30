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

	Id_  string
	Slug string
	SKU  string
	UPC  string

	Currency      string
	Price         float64
	ListPrice     float64
	InventoryCost float64

	Shipping  float64
	Inventory float64

	Weight     float64
	WeightUnit string

	DimensionsLength float64
	DimensionsWidth  float64
	DimensionsHeight float64
	DimensionsUnit   string

	Name              string
	Description       string
	EstimatedDelivery string

	CreatedAt time.Time
	UpdatedAt time.Time

	// Facets
	PriceOption         float64 `search:"price,facet"`
	ListPriceOption     float64 `search:"listPrice,facet"`
	InventoryCostOption float64 `search:"inventoryCost,facet"`

	InventoryOption float64 `search:"inventory,facet"`

	WeightOption     float64     `search:"weight,facet"`
	WeightUnitOption search.Atom `search:"weightUnit,facet"`

	DimensionsLengthOption float64     `search:"dimensionsLength,facet"`
	DimensionsWidthOption  float64     `search:"dimensionsWidth,facet"`
	DimensionsHeightOption float64     `search:"dimensionsHeight,facet"`
	DimensionsUnitOption   search.Atom `search:"dimensionsUnit,facet"`

	AvailableOption search.Atom `search:"available,facet"`
	HiddenOption    search.Atom `search:"hidden,facet"`
	PreorderOption  search.Atom `search:"preorder,facet"`
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

	doc.Currency = string(p.Currency)
	doc.Price = p.Currency.ToFloat(p.Price)
	doc.ListPrice = p.Currency.ToFloat(p.ListPrice)
	doc.InventoryCost = p.Currency.ToFloat(p.InventoryCost)

	doc.Shipping = float64(p.Shipping)
	doc.Inventory = float64(p.Inventory)

	doc.Weight = float64(p.Weight)
	doc.WeightUnit = string(p.WeightUnit)

	doc.DimensionsLength = float64(p.Dimensions.Length)
	doc.DimensionsWidth = float64(p.Dimensions.Width)
	doc.DimensionsHeight = float64(p.Dimensions.Height)
	doc.DimensionsUnit = string(p.DimensionsUnit)

	doc.Name = p.Name
	doc.Description = p.Description
	doc.EstimatedDelivery = p.EstimatedDelivery

	doc.CreatedAt = p.CreatedAt
	doc.UpdatedAt = p.UpdatedAt

	doc.PriceOption = float64(p.Price)
	doc.ListPriceOption = float64(p.ListPrice)
	doc.InventoryCostOption = float64(p.InventoryCost)

	doc.InventoryOption = float64(p.Inventory)
	doc.WeightOption = float64(p.Weight)
	doc.WeightUnitOption = search.Atom(p.WeightUnit)

	doc.DimensionsLengthOption = p.Dimensions.Length
	doc.DimensionsWidthOption = p.Dimensions.Width
	doc.DimensionsHeightOption = p.Dimensions.Height
	doc.DimensionsUnitOption = search.Atom(p.DimensionsUnit)

	if p.Available {
		doc.AvailableOption = "available"
	}

	if p.Hidden {
		doc.HiddenOption = "hidden"
	}

	if p.Preorder {
		doc.PreorderOption = "preorder"
	}

	return doc
}
