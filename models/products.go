package models

import (
	"time"
)

type Product struct {
	id          string
	title       string
	variants    []ProductVariant
	images      []Image
	description string
	stocked     int
	available   bool
	released    time.Time
	addLabel    string // Pre-order now or Add to cart
}

type ProductVariant struct {
	sku        string
	price      Currency
	stock      int
	weight     int
	dimensions string
	color      string
	size       string
}

type Image struct {
	name string
	url  string
}
