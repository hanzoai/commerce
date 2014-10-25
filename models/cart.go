package models

import (
	"time"
)

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

type ShippingOption struct {
	name  string
	price Currency
}
