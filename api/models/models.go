package models

import (
	"time"
)

type Currency struct {
	value int64
}

func (c Currency) Add()    {}
func (c Currency) Sub()    {}
func (c Currency) Mul()    {}
func (c Currency) String() {}

type LineItem struct {
	product     Product
	variant     ProductVariant
	description string
	quantity    int
}

type Cart struct {
	id        string
	items     []LineItem
	createdAt time.Time
}

type ShippingOption struct {
	name  string
	price Currency
}

type Order struct {
	id              string
	items           []LineItem
	createdAt       time.Time
	user            User
	shippingAddress Address
	billingAddress  Address
	subtotal        int
	tax             int
	shippingOption  ShippingOption
	shipping        int
	total           int
}

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

type User struct {
	id     string
	name   string
	email  string
	phone  string
	orders []Order
	cart   Cart
}

type Address struct {
	street     string
	unit       string
	city       string
	state      string
	postalCode string
	country    string
}
