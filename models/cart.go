package models

import (
	"time"
)

type LineItem struct {
	SKU			string
	Description string
	Quantity    int
}

type Cart struct {
	Id        string
	Items     []LineItem
	CreatedAt time.Time
}

type Order struct {
	Id              string
	Items           []LineItem
	CreatedAt       time.Time
	User            User
	ShippingAddress Address
	BillingAddress  Address
	Subtotal        int
	Tax             int
	ShippingOption  ShippingOption
	Shipping        int
	Total           int
}

type ShippingOption struct {
	Name  string
	Price Currency
}
