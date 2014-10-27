package models

import (
	"time"
	"github.com/mholt/binding"
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

func (c *Cart) FieldMap() binding.FieldMap {
	return binding.FieldMap{}
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

func (o *Order) FieldMap() binding.FieldMap {
	return binding.FieldMap{}
}

type ShippingOption struct {
	Name  string
	Price Currency
}
