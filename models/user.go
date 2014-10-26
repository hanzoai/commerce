package models

type User struct {
	Id              string
	Name            string
	Email           string
	Phone           string
	Orders          []Order
	Cart            Cart
	BillingAddress  Address
	ShippingAddress Address
}

type Address struct {
	Street     string
	Unit       string
	City       string
	State      string
	PostalCode string
	Country    string
}
