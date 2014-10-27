package models

import (
	"time"
)

type Product struct {
	Id          string
	Title       string
	Variants    []ProductVariant
	Images      []Image
	Description string
	Stocked     int
	Available   bool
	Released    time.Time
	AddLabel    string // Pre-order now or Add to cart
}

type ProductVariant struct {
	Id		   string
	Sku        string
	Price      Currency
	Stock      int
	Weight     int
	Dimensions string
	Color      string
	Size       string
}

type Image struct {
	Name string
	Url  string
}
